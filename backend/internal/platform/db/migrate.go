package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/437i/client-matcher/backend/migrations"
)

var ErrODLinkStorageNotReady = errors.New("OD link storage is not ready")

func MigrateOD(ctx context.Context, pool *pgxpool.Pool) error {
	var valid bool
	err := pool.QueryRow(ctx, "SELECT COALESCE(current_schema() = 'apps' AND has_schema_privilege(current_schema(), 'USAGE') AND has_schema_privilege(current_schema(), 'CREATE'), false)").Scan(&valid)
	if err != nil || !valid {
		return errors.New("OD requires the apps schema with USAGE and CREATE privileges")
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return errors.New("initialize OD migration lock failed")
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.Files,
		goose.WithTableName("apps.goose_db_version"), goose.WithSessionLocker(locker))
	if err != nil {
		return errors.New("initialize OD migrations failed")
	}
	if _, err := provider.Up(ctx); err != nil {
		return errors.New("apply OD migrations failed")
	}
	// SELECT also covers the columns returned by PGLinkRepo.Create's RETURNING.
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(bool_and(COALESCE(has_column_privilege(
			to_regclass('apps.client_links'), column_name, privilege), false)), false)
		FROM (VALUES
			('id', 'SELECT'),
			('crm_id', 'SELECT'),
			('site_id', 'SELECT'),
			('created_at', 'SELECT'),
			('crm_id', 'INSERT'),
			('site_id', 'INSERT'),
			('manually_confirmed', 'INSERT'),
			('confirmation_reason', 'INSERT'),
			('confirmation_codes', 'INSERT'),
			('confirmed_at', 'INSERT'),
			('assessment_rule_version', 'INSERT')
		) AS required(column_name, privilege)`).Scan(&valid)
	if err != nil || !valid {
		return ErrODLinkStorageNotReady
	}
	return nil
}
