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
	"github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/db"
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

	cfg, err := config.LoadDatabase()
	if err != nil {
		return err
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Ping(ctx, database); err != nil {
		return err
	}

	store := adminusers.NewStore(database)
	switch args[0] {
	case "list":
		return listUsers(ctx, store)
	case "promote":
		telegramID, err := parseTelegramID(args[1:])
		if err != nil {
			return err
		}
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
		user, err := store.DemoteByTelegramID(ctx, telegramID)
		if err != nil {
			return err
		}
		printUser("demoted", user)
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
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
}
