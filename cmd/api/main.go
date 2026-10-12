package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/config"
	"github.com/TonnPcst/martgo-backend/internal/handler"
	"github.com/TonnPcst/martgo-backend/internal/service"
	"github.com/TonnPcst/martgo-backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	st := store.New(pool)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	services := handler.Services{
		Auth:       service.NewAuthService(st, tokens),
		Users:      service.NewUserService(st),
		Categories: service.NewCategoryService(st),
		Suppliers:  service.NewSupplierService(st),
		Products:   service.NewProductService(st),
		Sales:      service.NewSaleService(st),
		Purchases:  service.NewPurchaseService(st),
	}

	router := handler.NewRouter(logger, tokens, st, pool, services, handler.RouterOptions{
		TrustProxy: cfg.TrustProxy,
		EnableDocs: cfg.EnableDocs,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server started", "addr", cfg.HTTPAddr, "env", cfg.Env)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	stop()

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
