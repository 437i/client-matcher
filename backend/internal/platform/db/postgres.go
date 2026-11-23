package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/platform/config"
	"github.com/437i/client-matcher/backend/internal/platform/sshtunnel"
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
	poolConfig, err := newPoolConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("create database pool failed")
	}

	if err := pingWithRetry(ctx, pool); err != nil {
		pool.Close()
		if ctx.Err() != nil {
			return nil, fmt.Errorf("db ping: %w", ctx.Err())
		}
		return nil, errors.New("database did not respond to ping")
	}
	slog.Info("db connection pool established")

	return pool, nil
}

func newPoolConfig(cfg config.DBConfig) (*pgxpool.Config, error) {
	poolConfig, err := pgxpool.ParseConfig(connString(cfg))
	if err != nil {
		return nil, errors.New("invalid database connection configuration")
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	return poolConfig, nil
}

func NewTunnelPool(ctx context.Context, cfg config.DBConfig, tunnel *sshtunnel.Tunnel, useApps bool) (*pgxpool.Pool, error) {
	poolConfig, err := tunnelPoolConfig(cfg, tunnel, useApps)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.New("create tunnel database pool failed")
	}
	return pool, nil
}

func tunnelPoolConfig(cfg config.DBConfig, tunnel *sshtunnel.Tunnel, useApps bool) (*pgxpool.Config, error) {
	host, port, err := net.SplitHostPort(tunnel.Address())
	if err != nil {
		return nil, errors.New("invalid tunnel listener address")
	}
	cfg.Host, cfg.Port = host, port
	poolConfig, err := newPoolConfig(cfg)
	if err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	poolConfig.ConnConfig.DialFunc = tunnel.DialContext
	poolConfig.PrepareConn = func(_ context.Context, conn *pgx.Conn) (bool, error) {
		return tunnel.CheckConn(conn.PgConn().Conn())
	}
	if useApps {
		poolConfig.ConnConfig.RuntimeParams["search_path"] = "apps"
	}
	return poolConfig, nil
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

		slog.Warn("db ping failed", "attempt", i)
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
