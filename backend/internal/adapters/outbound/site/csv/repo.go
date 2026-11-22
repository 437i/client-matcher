package csv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/437i/client-matcher/backend/internal/adapters/outbound/csvutil"
	"github.com/437i/client-matcher/backend/internal/adapters/outbound/site/shared"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

var ErrDuplicateID = errors.New("duplicate id")

var siteHeader = []string{"id", "title", "birthday"}

type CSVSiteRepo struct {
	byID map[int64]client.SiteClient
}

var _ ports.SiteReader = (*CSVSiteRepo)(nil)

func NewCSVSiteRepo(path string) (*CSVSiteRepo, error) {
	rows, err := csvutil.ReadFile(path, siteHeader)
	if err != nil {
		return nil, fmt.Errorf("load site csv: %w", err)
	}

	repo := &CSVSiteRepo{byID: make(map[int64]client.SiteClient, len(rows))}
	for _, row := range rows {
		cl, err := parseRow(row.Fields)
		if err != nil {
			slog.Warn("site csv: skip invalid row", "file", path, "line", row.Line)
			continue
		}
		if _, dup := repo.byID[cl.ID]; dup {
			return nil, fmt.Errorf("load site csv: line %d: %w", row.Line, ErrDuplicateID)
		}
		repo.byID[cl.ID] = cl
	}

	return repo, nil
}

func parseRow(f []string) (client.SiteClient, error) {
	id, err := strconv.ParseInt(f[0], 10, 64)
	if err != nil {
		return client.SiteClient{}, errors.New("invalid site id")
	}
	birthday, err := client.ParseOptionalDate(f[2])
	if err != nil {
		return client.SiteClient{}, err
	}
	unstructured, first, last := shared.ParseTitle(f[1])
	return client.SiteClient{ID: id, Birthday: birthday, UnstructuredName: unstructured, ShortName: first, LastName: last}, nil
}

func (r *CSVSiteRepo) GetByID(ctx context.Context, id int64) (client.SiteClient, error) {
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
