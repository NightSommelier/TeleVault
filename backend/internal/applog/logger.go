package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultFileMaxBytes = int64(100 * 1024 * 1024)
	defaultFileBackups  = 5
)

type Options struct {
	Component  string
	FileDir    string
	MaxBytes   int64
	MaxBackups int
}

func New(level string, options Options) *slog.Logger {
	writer := io.Writer(os.Stdout)
	if strings.TrimSpace(options.FileDir) != "" {
		component := sanitizeComponent(options.Component)
		maxBytes := options.MaxBytes
		if maxBytes <= 0 {
			maxBytes = defaultFileMaxBytes
		}
		maxBackups := options.MaxBackups
		if maxBackups <= 0 {
			maxBackups = defaultFileBackups
		}
		logPath := filepath.Join(options.FileDir, component+".log")
		fileWriter, err := newRotatingFileWriter(logPath, maxBytes, maxBackups)
		if err == nil {
			writer = io.MultiWriter(os.Stdout, fileWriter)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "televault logger: failed to enable file log %q: %v\n", logPath, err)
		}
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: ParseLevel(level),
	}))
}

func NewFromEnv(component string) *slog.Logger {
	maxBytes, _ := strconv.ParseInt(strings.TrimSpace(os.Getenv("LOG_FILE_MAX_BYTES")), 10, 64)
	maxBackups, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("LOG_FILE_MAX_BACKUPS")))
	return New(os.Getenv("LOG_LEVEL"), Options{
		Component:  component,
		FileDir:    os.Getenv("LOG_FILE_DIR"),
		MaxBytes:   maxBytes,
		MaxBackups: maxBackups,
	})
}

func sanitizeComponent(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "app"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

func ParseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
