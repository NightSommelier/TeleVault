package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/televault/TeleVault/backend/internal/adminusers"
	"github.com/televault/TeleVault/backend/internal/auth"
	"github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/crypto/secrets"
	"github.com/televault/TeleVault/backend/internal/db"
	"github.com/televault/TeleVault/backend/internal/telegramauth"
	"github.com/televault/TeleVault/backend/internal/telegramprobe"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("admin command failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("command is required")
	}

	ctx := context.Background()
	switch args[0] {
	case "list":
		database, err := openDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		store := adminusers.NewStore(database)
		return listUsers(ctx, store)
	case "promote":
		telegramID, err := parseTelegramID(args[1:])
		if err != nil {
			return err
		}
		database, err := openDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		store := adminusers.NewStore(database)
		user, err := store.PromoteByTelegramID(ctx, telegramID)
		if err != nil {
			return err
		}
		printUser("promoted", user)
		return nil
	case "demote":
		telegramID, err := parseTelegramID(args[1:])
		if err != nil {
			return err
		}
		database, err := openDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		store := adminusers.NewStore(database)
		user, err := store.DemoteByTelegramID(ctx, telegramID)
		if err != nil {
			return err
		}
		printUser("demoted", user)
		return nil
	case "probe-telegram-limit":
		return probeTelegramLimit(ctx, args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func openDatabase() (*sql.DB, error) {
	cfg, err := config.LoadDatabase()
	if err != nil {
		return nil, err
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := db.Ping(pingCtx, database); err != nil {
		cancel()
		_ = database.Close()
		return nil, err
	}
	cancel()

	return database, nil
}

func parseTelegramID(args []string) (int64, error) {
	flags := flag.NewFlagSet("role", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	telegramIDValue := flags.String("telegram-id", "", "Telegram numeric user ID")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	if *telegramIDValue == "" {
		return 0, errors.New("--telegram-id is required")
	}

	telegramID, err := strconv.ParseInt(*telegramIDValue, 10, 64)
	if err != nil || telegramID <= 0 {
		return 0, fmt.Errorf("--telegram-id must be a positive integer")
	}
	return telegramID, nil
}

func listUsers(ctx context.Context, store *adminusers.Store) error {
	users, err := store.List(ctx)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ID\tTELEGRAM_ID\tUSERNAME\tDISPLAY_NAME\tROLE")
	for _, user := range users {
		fmt.Fprintf(writer, "%s\t%d\t%s\t%s\t%s\n",
			user.ID,
			user.TelegramID,
			nullString(user.Username),
			nullString(user.DisplayName),
			user.Role,
		)
	}
	return writer.Flush()
}

func probeTelegramLimit(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("probe-telegram-limit", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	telegramIDValue := flags.String("telegram-id", "", "Telegram numeric user ID")
	minBytes := flags.Int64("min-bytes", telegramprobe.DefaultMinBytes, "smallest test upload size")
	maxBytes := flags.Int64("max-bytes", 0, "largest test upload size")
	stepBytes := flags.Int64("step-bytes", telegramprobe.DefaultStepBytes, "test upload increment")
	execute := flags.Bool("execute", false, "send and delete Telegram probe files")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *telegramIDValue == "" {
		return errors.New("--telegram-id is required")
	}
	telegramID, err := strconv.ParseInt(*telegramIDValue, 10, 64)
	if err != nil || telegramID <= 0 {
		return fmt.Errorf("--telegram-id must be a positive integer")
	}
	if *maxBytes <= 0 {
		return errors.New("--max-bytes is required for telegram limit probing")
	}

	plan := telegramprobe.Plan{
		MinBytes:  *minBytes,
		MaxBytes:  *maxBytes,
		StepBytes: *stepBytes,
	}
	if err := plan.Validate(); err != nil {
		return err
	}

	if !*execute {
		result, err := telegramprobe.DryRun(plan)
		if err != nil {
			return err
		}
		printProbeResult("dry-run", result)
		fmt.Fprintln(os.Stderr, "dry-run only; add --execute to upload and delete Telegram probe files")
		return nil
	}

	database, err := openDatabase()
	if err != nil {
		return err
	}
	defer database.Close()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	telegramSessionKey, err := secrets.ParseBase64Key(cfg.TelegramSessionKey)
	if err != nil {
		return err
	}
	secretsBox, err := secrets.NewBox(telegramSessionKey)
	if err != nil {
		return err
	}
	telegramAppID, err := cfg.TelegramAppID()
	if err != nil {
		return err
	}

	probeStore := telegramprobe.NewStore(database)
	account, err := probeStore.AccountByTelegramID(ctx, telegramID)
	if err != nil {
		return err
	}

	nextProbeAt := time.Now().Add(24 * time.Hour)
	if err := probeStore.MarkPending(ctx, account.UserID, nextProbeAt); err != nil {
		return err
	}

	sessionCrypto := auth.NewTelegramSessionCrypto(secretsBox)
	session, err := sessionCrypto.DecryptForTelegramID(account.TelegramID, account.EncryptedSession)
	if err != nil {
		_ = probeStore.MarkFailed(context.Background(), account.UserID, err, nextProbeAt)
		return err
	}

	telegramClient := telegramauth.NewClient(telegramAppID, cfg.TelegramAPIHash)
	result, err := telegramprobe.Run(ctx, telegramClient, session, nullString(account.StoragePeer), plan)
	if err != nil {
		_ = probeStore.MarkFailed(context.Background(), account.UserID, err, nextProbeAt)
		printProbeResult("failed", result)
		return err
	}
	if err := probeStore.MarkSuccess(ctx, account.UserID, result.DetectedBytes, nextProbeAt); err != nil {
		return err
	}

	printProbeResult("success", result)
	return nil
}

func printProbeResult(status string, result telegramprobe.Result) {
	fmt.Printf("probe_status=%s dry_run=%t detected_bytes=%d failed_bytes=%d attempted_sizes=%v\n",
		status,
		result.DryRun,
		result.DetectedBytes,
		result.FailedBytes,
		result.AttemptedSizes,
	)
}

func printUser(action string, user adminusers.User) {
	fmt.Printf("%s telegram_id=%d user_id=%s role=%s\n", action, user.TelegramID, user.ID, user.Role)
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  admin list")
	fmt.Fprintln(os.Stderr, "  admin promote --telegram-id <id>")
	fmt.Fprintln(os.Stderr, "  admin demote --telegram-id <id>")
	fmt.Fprintln(os.Stderr, "  admin probe-telegram-limit --telegram-id <id> --max-bytes <n> [--min-bytes <n>] [--step-bytes <n>] [--execute]")
}
