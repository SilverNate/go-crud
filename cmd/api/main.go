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

	"github.com/go-crud/go-task-api/internal/config"
	httpdelivery "github.com/go-crud/go-task-api/internal/delivery/http"
	"github.com/go-crud/go-task-api/internal/logger"
	"github.com/go-crud/go-task-api/internal/repository/memory"
	"github.com/go-crud/go-task-api/internal/repository/postgres"
	"github.com/go-crud/go-task-api/internal/usecase"
	"github.com/go-crud/go-task-api/internal/worker"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.LogLevel)

	repo, closeRepo, err := buildRepository(cfg)
	if err != nil {
		log.Error("initialize repository", slog.Any("error", err))
		os.Exit(1)
	}
	defer closeRepo()

	events := worker.New(cfg.WorkerQueueSize, log)
	events.Start()
	defer events.Stop()

	taskUsecase := usecase.NewTask(repo, events, log)
	handler := httpdelivery.NewRouter(taskUsecase, log)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", slog.String("address", cfg.HTTPAddr), slog.String("storage", cfg.StorageDriver))
		serverErr <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown failed", slog.Any("error", err))
	}
}

func buildRepository(cfg config.Config) (usecase.TaskRepository, func(), error) {
	switch cfg.StorageDriver {
	case "memory":
		repo, err := memory.NewFromJSON("testdata/tasks.json")
		if err != nil {
			return nil, nil, err
		}
		return repo, func() {}, nil
	case "postgres":
		repo, err := postgres.New(context.Background(), cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		return repo, func() { repo.Close() }, nil
	default:
		return nil, nil, errors.New("STORAGE_DRIVER must be memory or postgres")
	}
}
