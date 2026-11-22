package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/link"
)

const (
	table        = "client_links"
	idCol        = "id"
	crmIDCol     = "crm_id"
	siteIDCol    = "site_id"
	createdAtCol = "created_at"

	// имена ограничений из миграции — по ним понятно, какой ID уже занят
	uqCRMIDConstraint  = "uq_client_links_crm_id"
	uqSiteIDConstraint = "uq_client_links_site_id"

	pgUniqueViolation = "23505"
)

var linkColumns = []string{idCol, crmIDCol, siteIDCol, createdAtCol}

type PGLinkRepo struct {
	pool         *pgxpool.Pool
	queryBuilder sq.StatementBuilderType
}

var _ ports.ClientLinkRepo = (*PGLinkRepo)(nil)

func NewPGLinkRepo(pool *pgxpool.Pool) *PGLinkRepo {
	return &PGLinkRepo{
		pool:         pool,
		queryBuilder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *PGLinkRepo) Create(ctx context.Context, data link.Decision) (link.ClientLink, error) {
	var reason *string
	var confirmedAt *time.Time
	var ruleVersion *int
	codes := []string{}
	if data.Confirmation != nil {
		reason = &data.Confirmation.Reason
		confirmedAt = &data.Confirmation.At
		codes = data.Confirmation.Codes
		ruleVersion = &data.RuleVersion
	}
	sql, args, err := r.queryBuilder.
		Insert(table).
		Columns(crmIDCol, siteIDCol, "manually_confirmed", "confirmation_reason", "confirmation_codes", "confirmed_at", "assessment_rule_version").
		Values(data.CRMID, data.SiteID, data.Confirmation != nil, reason, codes, confirmedAt, ruleVersion).
		Suffix("RETURNING " + strings.Join(linkColumns, ", ")).
		ToSql()
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("build create query: %w", err)
	}

	// Ошибку сервера (в том числе нарушение уникальности) pgx может вернуть
	// как из Query, так и при чтении строк, поэтому маппим оба места.
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("execute create query: %w", mapCreateError(err))
	}

	result, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[link.ClientLink])
	if err != nil {
		return link.ClientLink{}, fmt.Errorf("collect created link: %w", mapCreateError(err))
	}

	return result, nil
}

// mapCreateError превращает нарушение уникальности в доменные ошибки.
func mapCreateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgUniqueViolation {
		return err
	}
	switch pgErr.ConstraintName {
	case uqCRMIDConstraint:
		return link.ErrCRMAlreadyLinked
	case uqSiteIDConstraint:
		return link.ErrSiteAlreadyLinked
	default:
		return err
	}
}

// List возвращает все связи от старых к новым (created_at, id — стабильный порядок).
func (r *PGLinkRepo) List(ctx context.Context) ([]link.ClientLink, error) {
	sql, args, err := r.queryBuilder.
		Select(linkColumns...).
		From(table).
		OrderBy(createdAtCol, idCol).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get list query: %w", err)
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("execute get list query: %w", err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[link.ClientLink])
	if err != nil {
		return nil, fmt.Errorf("collect links: %w", err)
	}

	return results, nil
}

func (r *PGLinkRepo) LookupBySourceIDs(ctx context.Context, crmID string, siteID int64) (bool, bool, error) {
	var crm, site bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM client_links WHERE crm_id=$1), EXISTS (SELECT 1 FROM client_links WHERE site_id=$2)", crmID, siteID).Scan(&crm, &site)
	return crm, site, err
}
