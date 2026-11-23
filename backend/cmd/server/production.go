package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/platform/config"
	"github.com/437i/client-matcher/backend/internal/platform/db"
	"github.com/437i/client-matcher/backend/internal/platform/sshtunnel"
)

type productionDBs struct {
	site, od             *pgxpool.Pool
	siteTunnel, odTunnel *sshtunnel.Tunnel
}

func (p *productionDBs) closeTransports() {
	if p.siteTunnel != nil {
		p.siteTunnel.Close()
	}
	if p.odTunnel != nil {
		p.odTunnel.Close()
	}
}

func (p *productionDBs) close() {
	p.closeTransports()
	if p.site != nil {
		p.site.Close()
	}
	if p.od != nil {
		p.od.Close()
	}
}

func openProduction(ctx context.Context, cfg config.Config) (_ *productionDBs, err error) {
	p := &productionDBs{}
	defer func() {
		if err != nil {
			p.close()
		}
	}()
	startup, failStartup := context.WithCancel(ctx)
	defer failStartup()
	initCtx, cancelInit := context.WithTimeout(startup, 30*time.Second)
	defer cancelInit()
	p.siteTunnel, err = sshtunnel.New(cfg.SiteSSH, net.JoinHostPort(cfg.SiteDB.Host, cfg.SiteDB.Port), cfg.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("initialize SITE tunnel: %w", err)
	}
	p.odTunnel, err = sshtunnel.New(cfg.ODSSH, net.JoinHostPort(cfg.DB.Host, cfg.DB.Port), cfg.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("initialize OD tunnel: %w", err)
	}
	p.site, err = db.NewTunnelPool(context.Background(), cfg.SiteDB, p.siteTunnel, false)
	if err != nil {
		return nil, fmt.Errorf("initialize SITE pool: %w", err)
	}
	p.od, err = db.NewTunnelPool(context.Background(), cfg.DB, p.odTunnel, true)
	if err != nil {
		return nil, fmt.Errorf("initialize OD pool: %w", err)
	}
	if err = p.siteTunnel.Start(initCtx, p.site.Reset, failStartup); err != nil {
		return nil, fmt.Errorf("connect SITE SSH: %w", err)
	}
	if err = p.odTunnel.Start(initCtx, p.od.Reset, failStartup); err != nil {
		return nil, fmt.Errorf("connect OD SSH: %w", err)
	}
	if err = p.ping(initCtx); err != nil {
		return nil, err
	}
	cancelInit()

	migration, cancelMigration := context.WithTimeout(startup, 2*time.Minute)
	// Goose may attempt to unlock with a context that ignores cancellation.
	stopAbort := context.AfterFunc(migration, p.closeTransports)
	err = db.MigrateOD(migration, p.od)
	stopped := stopAbort()
	migrationErr := migration.Err()
	cancelMigration()
	if err != nil {
		return nil, err
	}
	if !stopped || migrationErr != nil {
		return nil, fmt.Errorf("OD migration interrupted: %w", migrationErr)
	}

	final, cancelFinal := context.WithTimeout(startup, 5*time.Second)
	defer cancelFinal()
	if err = p.ping(final); err != nil {
		return nil, err
	}
	if err = sshtunnel.EnableRecovery(p.siteTunnel, p.odTunnel); err != nil {
		return nil, fmt.Errorf("enable database tunnel recovery: %w", err)
	}
	if err = final.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *productionDBs) ping(ctx context.Context) error {
	if err := p.site.Ping(ctx); err != nil {
		return errors.New("SITE PostgreSQL did not respond to ping")
	}
	if err := p.od.Ping(ctx); err != nil {
		return errors.New("OD PostgreSQL did not respond to ping")
	}
	return nil
}
