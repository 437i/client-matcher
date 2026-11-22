package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/adapters/outbound/site/shared"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

type SiteRepo struct{ pool *pgxpool.Pool }

var _ ports.SiteReader = (*SiteRepo)(nil)

func NewSiteRepo(pool *pgxpool.Pool) *SiteRepo { return &SiteRepo{pool: pool} }

func (r *SiteRepo) GetByID(ctx context.Context, id int64) (client.SiteClient, error) {
	if err := ctx.Err(); err != nil {
		return client.SiteClient{}, err
	}
	const query = "SELECT id, title, ((birthday AT TIME ZONE 'UTC') AT TIME ZONE 'Europe/Moscow')::date AS birthday FROM public.children WHERE id = $1"
	var (
		siteID   int64
		title    pgtype.Text
		birthday pgtype.Date
	)
	err := r.pool.QueryRow(ctx, query, id).Scan(&siteID, &title, &birthday)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return client.SiteClient{}, client.ErrSiteNotFound
		}
		if isInvalidDateError(err) {
			return client.SiteClient{}, fmt.Errorf("read site client: %w", client.ErrInvalidSourceData)
		}
		return client.SiteClient{}, fmt.Errorf("read site client: %w", err)
	}
	if !birthday.Valid {
		return buildSite(siteID, title, nil), nil
	}
	if birthday.InfinityModifier != pgtype.Finite {
		return client.SiteClient{}, client.ErrInvalidSourceData
	}
	d := client.Date{Year: birthday.Time.Year(), Month: int(birthday.Time.Month()), Day: birthday.Time.Day()}
	if !d.IsValid() {
		return client.SiteClient{}, client.ErrInvalidSourceData
	}
	return buildSite(siteID, title, &d), nil
}

func buildSite(id int64, title pgtype.Text, birthday *client.Date) client.SiteClient {
	value := ""
	if title.Valid {
		value = title.String
	}
	unstructured, shortName, lastName := shared.ParseTitle(value)
	result := client.SiteClient{ID: id, Birthday: client.CloneDate(birthday), UnstructuredName: unstructured, ShortName: shortName, LastName: lastName}
	return result
}

func isInvalidDateError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22008"
}
