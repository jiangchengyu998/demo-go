package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jiangchengyu998/demo-go/internal/config"
	"github.com/jiangchengyu998/demo-go/internal/httpapi"
	"github.com/jiangchengyu998/demo-go/internal/item"
	"github.com/jiangchengyu998/demo-go/internal/observability"
	"github.com/jiangchengyu998/demo-go/internal/storage"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		slog.Error("load settings", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.ConfigureTracing(ctx, settings)
	if err != nil {
		logger.Error("configure tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			logger.Warn("shutdown tracing", "error", err)
		}
	}()

	repository, cleanup, err := buildRepository(ctx, settings)
	if err != nil {
		logger.Error("initialize repository", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	service := item.NewService(repository, logger)
	handler := observability.HTTPMiddleware(httpapi.NewHandler(service), logger)
	server := &http.Server{
		Addr:              ":" + settings.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", server.Addr, "environment", settings.DeploymentEnvironment)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}
}

func buildRepository(ctx context.Context, settings config.Settings) (item.Repository, func(), error) {
	if !settings.Database.Enabled {
		slog.Info("database env not found; using in-memory repository")
		return storage.NewMemoryRepository(), func() {}, nil
	}

	repository, err := storage.NewMySQLRepository(ctx, settings.Database)
	if err != nil {
		return nil, func() {}, err
	}

	return repository, func() {
		if err := repository.Close(); err != nil {
			slog.Warn("close repository", "error", err)
		}
	}, nil
}
