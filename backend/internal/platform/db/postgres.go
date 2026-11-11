package db

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

func connString(cfg config.DBConfig) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + cfg.Name,
		RawQuery: url.Values{"sslmode": {cfg.SSLMode}}.Encode(),
	}
	return u.String()
}

// NewPool создаёт пул и убеждается, что БД отвечает (с несколькими попытками —
// при старте через docker compose база может подняться чуть позже приложения).
// Вызывающий код отвечает за pool.Close().
func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(connString(cfg))
	if err != nil {
		return nil, fmt.Errorf("parse db connection string: %w", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pingWithRetry(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	slog.Info("db connection pool established")

	return pool, nil
}

func pingWithRetry(ctx context.Context, pool *pgxpool.Pool) error {
	const (
		attempts    = 5
		pingTimeout = 3 * time.Second
		retryDelay  = time.Second
	)

	var err error
	for i := 1; i <= attempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}

		slog.Warn("db ping failed", "attempt", i, "error", err)
		if i == attempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	return err
}
