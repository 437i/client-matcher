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
	tokens []string // нормализованные слова ФИО для поиска
	key    string   // ключ сортировки
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
			slog.Warn("crm csv: skip row", "file", path, "line", row.Line, "error", err)
			continue
		}
		// Дубликат ID — неоднозначность, а не «плохая строка»: падаем сразу.
		if _, dup := repo.byID[cl.ID]; dup {
			return nil, fmt.Errorf("load crm csv: line %d: client %s: %w", row.Line, cl.ID, ErrDuplicateID)
		}
		repo.byID[cl.ID] = cl

		tokens := strings.Fields(client.NormalizeName(cl.FullName()))
		repo.records = append(repo.records, record{
			client: cl,
			tokens: tokens,
			key:    strings.Join(tokens, " ") + "\x00" + cl.ID,
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
		return "", "", "", fmt.Errorf("unexpected crm title %q: want \"Фамилия Имя [Отчество]\"", title)
	}
	return parts[0], parts[1], strings.Join(parts[2:], " "), nil
}

// Search: каждое слово запроса должно быть началом отдельного слова ФИО
// (фамилии, имени, отчества) — порядок слов и регистр не важны, ё = е.
func (r *CSVCRMRepo) Search(ctx context.Context, query string, limit int) ([]client.CRMClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := []client.CRMClient{}
	queryTokens := strings.Fields(client.NormalizeName(query))
	if len(queryTokens) == 0 || limit <= 0 {
		return result, nil
	}
	// Длинные слова запроса сопоставляем первыми: они специфичнее коротких.
	slices.SortStableFunc(queryTokens, func(a, b string) int { return len(b) - len(a) })

	for _, rec := range r.records {
		if matchTokens(queryTokens, rec.tokens) {
			result = append(result, rec.client)
			if len(result) == limit {
				break
			}
		}
	}

	return result, nil
}

// matchTokens: каждому слову запроса должно найтись своё слово ФИО, начинающееся
// с него. «Свои» — чтобы запрос «Иван Иванов» не находил «Иванова Мария»,
// где оба слова запроса совпали бы с одним и тем же словом.
func matchTokens(query, tokens []string) bool {
	used := make([]bool, len(tokens))
	for _, q := range query {
		found := false
		for i, t := range tokens {
			if !used[i] && strings.HasPrefix(t, q) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
