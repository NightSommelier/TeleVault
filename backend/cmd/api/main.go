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

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/applog"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/agefile"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/db"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/httpserver"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramauth"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}
	logger = applog.New(cfg.LogLevel, applog.Options{
		Component:  "api",
		FileDir:    cfg.LogFileDir,
		MaxBytes:   cfg.LogFileMaxBytes,
		MaxBackups: cfg.LogFileMaxBackups,
	})

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

	ageRecipient, err := agefile.RecipientFromIdentity(cfg.AppAgeIdentity)
	if err != nil {
		logger.Error("age recipient initialization failed", "error", err)
		os.Exit(1)
	}
	ageIdentity, err := agefile.IdentityFromString(cfg.AppAgeIdentity)
	if err != nil {
		logger.Error("age identity initialization failed", "error", err)
		os.Exit(1)
	}

	telegramAppID, err := cfg.TelegramAppID()
	if err != nil {
		logger.Error("telegram api id validation failed", "error", err)
		os.Exit(1)
	}
	telegramClient := telegramauth.NewClient(telegramAppID, cfg.TelegramAPIHash)

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpserver.New(cfg, logger, database, secretsBox, ageRecipient, ageIdentity, telegramClient),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("api server listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "debug", cfg.AppDebug, "version", buildinfo.Version, "commit", buildinfo.Commit)
		errs <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("api server stopped")
}
