package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	web "github.com/437i/client-matcher/backend/internal/adapters/inbound/http"
	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
	crmCSV "github.com/437i/client-matcher/backend/internal/adapters/outbound/crm/csv"
	linkPG "github.com/437i/client-matcher/backend/internal/adapters/outbound/link/postgres"
	siteCSV "github.com/437i/client-matcher/backend/internal/adapters/outbound/site/csv"
	"github.com/437i/client-matcher/backend/internal/application/services"
	"github.com/437i/client-matcher/backend/internal/platform/config"
	"github.com/437i/client-matcher/backend/internal/platform/db"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load() // .env нужен только для локального запуска; в Docker переменные задаёт compose

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// ctx отменяется по Ctrl+C / SIGTERM (docker stop) — запускает graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	crmRepo, err := crmCSV.NewCSVCRMRepo(cfg.CSV.CRMPath)
	if err != nil {
		return err
	}
	siteRepo, err := siteCSV.NewCSVSiteRepo(cfg.CSV.SitePath)
	if err != nil {
		return err
	}

	pool, err := db.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	linkRepo := linkPG.NewPGLinkRepo(pool)

	searchService := services.NewClientSearchService(crmRepo, siteRepo)
	linkService := services.NewClientLinkService(crmRepo, siteRepo, linkRepo)

	router, err := web.NewRouter(
		handlers.NewClientHandler(searchService),
		handlers.NewLinkHandler(linkService),
		handlers.NewHealthHandler(pool.Ping),
	)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", cfg.Server.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		// ListenAndServe вернул ошибку до shutdown (например, порт занят)
		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
