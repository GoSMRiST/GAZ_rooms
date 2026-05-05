package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rooms/internal/app"
	"rooms/internal/config"
	"rooms/internal/repository"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	ctx := context.Background()

	cfg, err := config.InitConfig()
	if err != nil {
		slog.Error(err.Error())
		return
	}

	slog.Info("Config is init")

	log := setupLogger(cfg.LogLevel)
	log.Info("Logger is init")

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := repository.InitDataBase(log, ctx, connString)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		return
	}
	defer db.Close()

	log.Info("Database connected")

	application, err := app.NewApp(log, cfg, db)
	if err != nil {
		log.Error("failed to init app", "error", err)
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := application.Run(); err != nil {
			log.Error("server stopped with error", "error", err)
		}
	}()

	<-stop

	log.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := application.Stop(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	log.Info("Server stopped gracefully")
}

func setupLogger(env string) *slog.Logger {
	var logger *slog.Logger

	switch env {
	case envLocal:
		logger = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)

	case envDev:
		logger = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)

	case envProd:
		logger = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)

	default:
		logger = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	}

	return logger
}
