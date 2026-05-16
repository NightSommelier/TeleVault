package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/db"
	"github.com/televault/TeleVault/backend/internal/uploads"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadDatabase()
	if err != nil {
		logger.Error("database configuration failed", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	result, err := uploads.NewStore(database).ExpireAbandoned(context.Background(), time.Now())
	if err != nil {
		logger.Error("cleanup failed", "error", err)
		os.Exit(1)
	}

	logger.Info("cleanup complete", "expired_uploads", result.ExpiredUploads)
}
