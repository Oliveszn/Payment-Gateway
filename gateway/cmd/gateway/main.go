package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"payment-gateway/internal/bank"
	"payment-gateway/internal/config"
	"payment-gateway/internal/handlers"
	"payment-gateway/internal/repository"
	"payment-gateway/internal/service"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	//Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	//Config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	//Database
	db, err := connectDB(cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to database")

	//layers
	repo := repository.New(db)
	bankClient := bank.New(cfg.BankBaseURL, cfg.MaxRetries, cfg.RetryBaseDelayMS)
	svc := service.New(repo, bankClient)
	handler := handlers.New(svc)
	router := handlers.NewRouter(handler, repo)

	//HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	//Start server in background
	go func() {
		slog.Info("gateway starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	//Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully...")

	//Give in-flight requests 30 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("gateway stopped")
}

func connectDB(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		if err := db.PingContext(ctx); err == nil {
			return db, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database not ready after 30s: %w", ctx.Err())
		case <-time.After(2 * time.Second):
			slog.Debug("waiting for database...")
		}
	}
}
