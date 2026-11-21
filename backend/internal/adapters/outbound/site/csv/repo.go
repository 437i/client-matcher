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
			slog.Warn("site csv: skip invalid row", "file", path, "line", row.Line)
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
	birthday, err := client.ParseOptionalDate(f[2])
	if err != nil {
		return client.SiteClient{}, err
	}
	parts := strings.Fields(f[1])
	cl := client.SiteClient{ID: f[0], Birthday: birthday}
	if len(parts) < 2 {
		cl.UnstructuredName = f[1]
	} else {
		cl.ShortName, cl.LastName = parts[0], strings.Join(parts[1:], " ")
	}
	return cl, nil
}

func (r *CSVSiteRepo) GetByID(ctx context.Context, id string) (client.SiteClient, error) {
	if err := ctx.Err(); err != nil {
		return client.SiteClient{}, err
	}

	cl, ok := r.byID[id]
	if !ok {
		return client.SiteClient{}, client.ErrSiteNotFound
	}

	cl.Birthday = client.CloneDate(cl.Birthday)
	return cl, nil
}
