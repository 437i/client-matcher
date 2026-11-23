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
	return nil
}
