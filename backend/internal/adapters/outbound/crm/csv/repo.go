package csv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/437i/client-matcher/backend/internal/adapters/outbound/csvutil"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

var ErrDuplicateID = errors.New("duplicate id")

var crmHeader = []string{"id", "title", "birthday", "clinic"}

type record struct {
	client client.CRMClient
	tokens []client.SearchToken // поисковые формы слов ФИО
	key    string               // стабильный ключ сортировки при равной релевантности
}

// CSVCRMRepo — реализация ports.CRMReader поверх CSV-файла. После создания
// данные только читаются, поэтому мьютекс не нужен. Все поля CRMClient —
// значения, так что возвращаемые копии не связаны с внутренним состоянием.
type CSVCRMRepo struct {
	records []record
	byID    map[string]client.CRMClient
}

var _ ports.CRMReader = (*CSVCRMRepo)(nil)

func NewCSVCRMRepo(path string) (*CSVCRMRepo, error) {
	rows, err := csvutil.ReadFile(path, crmHeader)
	if err != nil {
		return nil, fmt.Errorf("load crm csv: %w", err)
	}

	repo := &CSVCRMRepo{byID: make(map[string]client.CRMClient, len(rows))}
	for _, row := range rows {
		cl, err := parseRow(row.Fields)
		if err != nil {
			slog.Warn("crm csv: skip invalid row", "file", path, "line", row.Line)
			continue
		}
		// Дубликат ID — неоднозначность, а не «плохая строка»: падаем сразу.
		if _, dup := repo.byID[cl.ID]; dup {
			return nil, fmt.Errorf("load crm csv: line %d: client %s: %w", row.Line, cl.ID, ErrDuplicateID)
		}
		repo.byID[cl.ID] = cl

		tokens := client.TokenizeCRMName(cl)
		repo.records = append(repo.records, record{
			client: cl,
			tokens: tokens,
			key:    client.NormalizeName(cl.FullName()) + "\x00" + cl.ID,
		})
	}

	slices.SortFunc(repo.records, func(a, b record) int { return strings.Compare(a.key, b.key) })

	return repo, nil
}

func parseRow(f []string) (client.CRMClient, error) {
	if f[0] == "" {
		return client.CRMClient{}, errors.New("empty id")
	}
	lastName, firstName, patronymic, err := parseTitle(f[1])
	if err != nil {
		return client.CRMClient{}, err
	}
	birthday, err := client.ParseDate(f[2])
	if err != nil {
		return client.CRMClient{}, err
	}
	return client.CRMClient{
		ID:         f[0],
		LastName:   lastName,
		FirstName:  firstName,
		Patronymic: patronymic,
		Birthday:   birthday,
		Clinic:     f[3],
	}, nil
}

// parseTitle разбирает «Фамилия Имя [Отчество...]»; отчество может состоять
// из нескольких слов («Бек кызы»).
func parseTitle(title string) (lastName, firstName, patronymic string, err error) {
	parts := strings.Fields(title)
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("unexpected crm title: want \"Фамилия Имя [Отчество]\"")
	}
	return parts[0], parts[1], strings.Join(parts[2:], " "), nil
}

// Search ищет по любым словам ФИО, независимо от их порядка. Сначала идут
// точные совпадения, затем варианты имён, префиксы, подстроки и единичные
// опечатки. Результаты остаются кандидатами: этот поиск не подтверждает личность.
func (r *CSVCRMRepo) Search(ctx context.Context, query string, limit int) ([]client.CRMClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := make([]client.CRMClient, 0)
	queryTokens := client.TokenizeSearchQuery(query)
	if len(queryTokens) == 0 || limit <= 0 {
		return result, nil
	}

	type rankedRecord struct {
		record record
		score  int
	}
	ranked := make([]rankedRecord, 0)
	for _, rec := range r.records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		score := client.MatchNameTokens(queryTokens, rec.tokens)
		if score > 0 {
			ranked = append(ranked, rankedRecord{record: rec, score: score})
		}
	}

	slices.SortFunc(ranked, func(a, b rankedRecord) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return strings.Compare(a.record.key, b.record.key)
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	for _, item := range ranked {
		result = append(result, item.record.client)
	}

	return result, nil
}

func (r *CSVCRMRepo) GetByID(ctx context.Context, id string) (client.CRMClient, error) {
	if err := ctx.Err(); err != nil {
		return client.CRMClient{}, err
	}

	cl, ok := r.byID[id]
	if !ok {
		return client.CRMClient{}, client.ErrCRMNotFound
	}

	return cl, nil
}
