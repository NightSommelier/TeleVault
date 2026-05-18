package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/crypto/secrets"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/db"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/telegramauth"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/uploads"
)

func main() {
	var flags workerFlags
	flag.StringVar(&flags.workerID, "worker-id", defaultWorkerID(), "stable worker identifier for queue leases")
	flag.BoolVar(&flags.once, "once", false, "drain at most one queued part and exit")
	flag.DurationVar(&flags.pollInterval, "poll-interval", 5*time.Second, "delay between empty queue polls")
	flag.DurationVar(&flags.leaseDuration, "lease-duration", 5*time.Minute, "queue lease duration")
	flag.DurationVar(&flags.uploadTimeout, "upload-timeout", 30*time.Minute, "per-part Telegram upload timeout")
	flag.DurationVar(&flags.retryBaseDelay, "retry-base-delay", 30*time.Second, "base retry delay for transient upload failures")
	flag.DurationVar(&flags.retryMaxDelay, "retry-max-delay", 30*time.Minute, "maximum retry delay for transient upload failures")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

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

	spool, err := uploads.NewLocalSpool(cfg.UploadStagingDir)
	if err != nil {
		logger.Error("upload staging initialization failed", "error", err)
		os.Exit(1)
	}

	worker, err := uploads.NewDrainWorker(
		uploads.NewStore(database),
		spool,
		auth.NewTelegramSessionCrypto(secretsBox),
		telegramauth.NewClient(telegramAppID, cfg.TelegramAPIHash),
		uploads.WorkerSettings{
			WorkerID:       flags.workerID,
			LeaseDuration:  flags.leaseDuration,
			RetryBaseDelay: flags.retryBaseDelay,
			RetryMaxDelay:  flags.retryMaxDelay,
			UploadTimeout:  flags.uploadTimeout,
		},
	)
	if err != nil {
		logger.Error("worker initialization failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

loop:
	for {
		worked, err := worker.DrainOne(ctx)
		if err != nil {
			logger.Warn("upload part drain failed", "error", err)
		}
		if flags.once || ctx.Err() != nil {
			break
		}
		if !worked {
			select {
			case <-ctx.Done():
				break loop
			case <-time.After(flags.pollInterval):
			}
		}
	}
}

type workerFlags struct {
	workerID       string
	once           bool
	pollInterval   time.Duration
	leaseDuration  time.Duration
	uploadTimeout  time.Duration
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
}

func defaultWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "worker"
	}
	return hostname
}
