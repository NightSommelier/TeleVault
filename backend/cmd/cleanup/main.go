package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/applog"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/db"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramauth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/uploads"
)

const cleanupArtifactLimit = 100

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}
	logger = applog.New(cfg.LogLevel, applog.Options{
		Component:  "cleanup",
		FileDir:    cfg.LogFileDir,
		MaxBytes:   cfg.LogFileMaxBytes,
		MaxBackups: cfg.LogFileMaxBackups,
	})
	logger.Info("cleanup starting", "debug", cfg.AppDebug, "version", buildinfo.Version, "commit", buildinfo.Commit)

	telegramSessionKey, err := secrets.ParseBase64Key(cfg.TelegramSessionKey)
	if err != nil {
		logger.Error("telegram session key validation failed", "error", err)
		os.Exit(1)
	}
	secretsBox, err := secrets.NewBox(telegramSessionKey)
	if err != nil {
		logger.Error("telegram session crypto initialization failed", "error", err)
		os.Exit(1)
	}

	telegramAppID, err := cfg.TelegramAppID()
	if err != nil {
		logger.Error("telegram api id validation failed", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	store := uploads.NewStore(database)
	result, err := store.ExpireAbandoned(context.Background(), time.Now())
	if err != nil {
		logger.Error("cleanup failed", "error", err)
		os.Exit(1)
	}

	spool, err := uploads.NewLocalSpool(cfg.UploadStagingDir)
	if err != nil {
		logger.Error("upload staging initialization failed", "error", err)
		os.Exit(1)
	}

	localArtifacts, err := store.PendingLocalStagingCleanupArtifacts(context.Background(), cleanupArtifactLimit)
	if err != nil {
		logger.Error("local staging cleanup artifact lookup failed", "error", err)
		os.Exit(1)
	}
	for _, artifact := range localArtifacts {
		if err := spool.Delete(artifact.StorageKey); err != nil {
			result.LocalStagingFailed++
			if markErr := store.MarkLocalStagingDeleteFailed(context.Background(), artifact.PartID, err); markErr != nil {
				logger.Warn("failed to record local staging cleanup error", "part_id", artifact.PartID, "error", markErr)
			}
			logger.Warn("local staging cleanup failed", "upload_id", artifact.UploadID, "part_id", artifact.PartID, "storage_key", artifact.StorageKey, "error", err)
			continue
		}
		if err := store.MarkLocalStagingDeleted(context.Background(), artifact.PartID); err != nil {
			result.LocalStagingFailed++
			logger.Warn("failed to mark local staging deleted", "upload_id", artifact.UploadID, "part_id", artifact.PartID, "storage_key", artifact.StorageKey, "error", err)
			continue
		}
		result.LocalStagingDeleted++
	}

	sessionCrypto := auth.NewTelegramSessionCrypto(secretsBox)
	telegramClient := telegramauth.NewClient(telegramAppID, cfg.TelegramAPIHash)

	artifacts, err := store.PendingTelegramCleanupArtifacts(context.Background(), cleanupArtifactLimit)
	if err != nil {
		logger.Error("telegram cleanup artifact lookup failed", "error", err)
		os.Exit(1)
	}

	for _, artifact := range artifacts {
		session, err := sessionCrypto.DecryptForTelegramID(artifact.OwnerTelegramID, artifact.EncryptedSession)
		if err != nil {
			result.TelegramArtifactsFailed++
			if markErr := store.MarkTelegramArtifactDeleteFailed(context.Background(), artifact.PartID, err); markErr != nil {
				logger.Warn("failed to record telegram cleanup error", "part_id", artifact.PartID, "error", markErr)
			}
			logger.Warn("telegram cleanup session decrypt failed", "source", artifact.Source, "resource_id", artifact.ResourceID, "part_id", artifact.PartID, "error", err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err = telegramClient.DeleteEncryptedPart(ctx, session, nullableString(artifact.TelegramPeer), artifact.MessageID)
		cancel()
		if err != nil {
			result.TelegramArtifactsFailed++
			if markErr := store.MarkTelegramArtifactDeleteFailed(context.Background(), artifact.PartID, err); markErr != nil {
				logger.Warn("failed to record telegram cleanup error", "part_id", artifact.PartID, "error", markErr)
			}
			logger.Warn("telegram artifact deletion failed", "source", artifact.Source, "resource_id", artifact.ResourceID, "part_id", artifact.PartID, "message_id", artifact.MessageID, "error", err)
			continue
		}

		if err := store.MarkTelegramArtifactDeleted(context.Background(), artifact.PartID, time.Now()); err != nil {
			result.TelegramArtifactsFailed++
			logger.Warn("failed to mark telegram artifact deleted", "source", artifact.Source, "resource_id", artifact.ResourceID, "part_id", artifact.PartID, "message_id", artifact.MessageID, "error", err)
			continue
		}
		result.TelegramArtifactsDeleted++
	}

	logger.Info(
		"cleanup complete",
		"expired_uploads", result.ExpiredUploads,
		"local_staging_deleted", result.LocalStagingDeleted,
		"local_staging_failed", result.LocalStagingFailed,
		"telegram_artifacts_deleted", result.TelegramArtifactsDeleted,
		"telegram_artifacts_failed", result.TelegramArtifactsFailed,
	)
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
