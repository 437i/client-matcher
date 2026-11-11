package csv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/437i/client-matcher/backend/internal/adapters/outbound/csvutil"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

var ErrDuplicateID = errors.New("duplicate id")

var siteHeader = []string{"id", "title", "birthday"}

type CSVSiteRepo struct {
	byID map[string]client.SiteClient
}

var _ ports.SiteReader = (*CSVSiteRepo)(nil)

func NewCSVSiteRepo(path string) (*CSVSiteRepo, error) {
	rows, err := csvutil.ReadFile(path, siteHeader)
	if err != nil {
		return nil, fmt.Errorf("load site csv: %w", err)
	}

	repo := &CSVSiteRepo{byID: make(map[string]client.SiteClient, len(rows))}
	for _, row := range rows {
		cl, err := parseRow(row.Fields)
		if err != nil {
			slog.Warn("site csv: skip row", "file", path, "line", row.Line, "error", err)
			continue
		}
		if _, dup := repo.byID[cl.ID]; dup {
			return nil, fmt.Errorf("load site csv: line %d: client %s: %w", row.Line, cl.ID, ErrDuplicateID)
		}
		repo.byID[cl.ID] = cl
	}

	return repo, nil
}

func parseRow(f []string) (client.SiteClient, error) {
	if f[0] == "" {
		return client.SiteClient{}, errors.New("empty id")
	}
	shortName, lastName, err := parseTitle(f[1])
	if err != nil {
		return client.SiteClient{}, err
	}
	birthday, err := client.ParseDate(f[2])
	if err != nil {
		return client.SiteClient{}, err
	}
	return client.SiteClient{
		ID:        f[0],
		ShortName: shortName,
		LastName:  lastName,
		Birthday:  birthday,
	}, nil
}

// parseTitle разбирает «Имя Фамилия»: первое слово — имя (как на сайте, часто
// уменьшительное), всё остальное — фамилия (может состоять из нескольких слов).
func parseTitle(title string) (shortName, lastName string, err error) {
	parts := strings.Fields(title)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected site title %q: want \"Имя Фамилия\"", title)
	}
	return parts[0], strings.Join(parts[1:], " "), nil
}

func (r *CSVSiteRepo) GetByID(ctx context.Context, id string) (client.SiteClient, error) {
	if err := ctx.Err(); err != nil {
		return client.SiteClient{}, err
	}

	cl, ok := r.byID[id]
	if !ok {
		return client.SiteClient{}, client.ErrSiteNotFound
	}

	return cl, nil
}
