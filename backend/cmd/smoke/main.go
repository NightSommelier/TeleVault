package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/televault/TeleVault/backend/internal/auth"
	appconfig "github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/crypto/secrets"
	"github.com/televault/TeleVault/backend/internal/db"
	"github.com/televault/TeleVault/backend/internal/telegramauth"
	"github.com/televault/TeleVault/backend/internal/uploads"
)

const smokeWorkerID = "smoke-worker"

var (
	errPartStaged       = errors.New("upload part staged")
	errUploadIncomplete = errors.New("upload incomplete")
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("smoke configuration failed", "error", err)
		os.Exit(1)
	}

	if err := run(context.Background(), cfg, logger); err != nil {
		logger.Error("smoke failed", "error", err)
		os.Exit(1)
	}
}

type config struct {
	BaseURL   string
	CookieJar string
	FilePath  string
}

type cookies struct {
	Refresh string
	CSRF    string
}

func loadConfig() (config, error) {
	cfg := config{
		BaseURL:   strings.TrimRight(getEnv("SMOKE_BASE_URL", "http://localhost:8080"), "/"),
		CookieJar: os.Getenv("SMOKE_COOKIE_JAR"),
		FilePath:  os.Getenv("SMOKE_FILE"),
	}
	if cfg.CookieJar == "" {
		return config{}, errors.New("SMOKE_COOKIE_JAR is required")
	}
	if cfg.FilePath == "" {
		return config{}, errors.New("SMOKE_FILE is required")
	}
	return cfg, nil
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	cookies, err := readCookies(cfg.CookieJar)
	if err != nil {
		return err
	}

	input, err := os.ReadFile(filepath.Clean(cfg.FilePath))
	if err != nil {
		return err
	}
	checksum := sha256.Sum256(input)
	name := filepath.Base(cfg.FilePath)
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	if err := ready(ctx, client, cfg.BaseURL); err != nil {
		return err
	}

	uploadID, err := createUpload(ctx, client, cfg, cookies, name, mimeType, len(input), hex.EncodeToString(checksum[:]))
	if err != nil {
		return err
	}
	logger.Info("smoke upload created", "upload_id", uploadID)

	if err := uploadPart(ctx, client, cfg, cookies, uploadID, input); err != nil {
		if !errors.Is(err, errPartStaged) {
			return err
		}
		if err := drainStagedPart(ctx, logger); err != nil {
			return err
		}
	}
	logger.Info("smoke part uploaded", "upload_id", uploadID)

	fileID, err := completeUpload(ctx, client, cfg, cookies, uploadID)
	if err != nil {
		return err
	}
	logger.Info("smoke upload complete", "file_id", fileID)

	output, err := downloadFile(ctx, client, cfg, cookies, fileID)
	if err != nil {
		return err
	}
	outputChecksum := sha256.Sum256(output)
	if !bytes.Equal(outputChecksum[:], checksum[:]) {
		return errors.New("download checksum mismatch")
	}

	logger.Info("smoke complete", "file_id", fileID, "bytes", len(output), "sha256", hex.EncodeToString(outputChecksum[:]))
	return nil
}

func ready(ctx context.Context, client *http.Client, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned %s", resp.Status)
	}
	return nil
}

func createUpload(ctx context.Context, client *http.Client, cfg config, cookies cookies, name string, mimeType string, size int, checksum string) (string, error) {
	body := map[string]any{
		"name":            name,
		"mime_type":       mimeType,
		"plaintext_size":  size,
		"checksum":        checksum,
		"idempotency_key": fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
	}
	var response struct {
		Upload struct {
			ID string `json:"id"`
		} `json:"upload"`
	}
	if err := postJSON(ctx, client, cfg.BaseURL+"/uploads", cookies, body, &response); err != nil {
		return "", err
	}
	if response.Upload.ID == "" {
		return "", errors.New("upload response missing id")
	}
	return response.Upload.ID, nil
}

func uploadPart(ctx context.Context, client *http.Client, cfg config, cookies cookies, uploadID string, input []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/uploads/"+uploadID+"/parts/1", bytes.NewReader(input))
	if err != nil {
		return err
	}
	addAuthHeaders(req, cookies)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return errPartStaged
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload part returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func completeUpload(ctx context.Context, client *http.Client, cfg config, cookies cookies, uploadID string) (string, error) {
	var response struct {
		File struct {
			ID string `json:"id"`
		} `json:"file"`
	}
	if err := postJSON(ctx, client, cfg.BaseURL+"/uploads/"+uploadID+"/complete", cookies, nil, &response); err != nil {
		if strings.Contains(err.Error(), "upload_incomplete") {
			return "", errUploadIncomplete
		}
		return "", err
	}
	if response.File.ID == "" {
		return "", errors.New("complete response missing file id")
	}
	return response.File.ID, nil
}

func drainStagedPart(ctx context.Context, logger *slog.Logger) error {
	cfg, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("load app config for smoke worker: %w", err)
	}
	telegramSessionKey, err := secrets.ParseBase64Key(cfg.TelegramSessionKey)
	if err != nil {
		return fmt.Errorf("parse telegram session key: %w", err)
	}
	secretsBox, err := secrets.NewBox(telegramSessionKey)
	if err != nil {
		return fmt.Errorf("initialize telegram session crypto: %w", err)
	}
	telegramAppID, err := cfg.TelegramAppID()
	if err != nil {
		return fmt.Errorf("parse telegram api id: %w", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database for smoke worker: %w", err)
	}
	defer database.Close()

	spool, err := uploads.NewLocalSpool(cfg.UploadStagingDir)
	if err != nil {
		return fmt.Errorf("open upload staging for smoke worker: %w", err)
	}
	worker, err := uploads.NewDrainWorker(
		uploads.NewStore(database),
		spool,
		auth.NewTelegramSessionCrypto(secretsBox),
		telegramauth.NewClient(telegramAppID, cfg.TelegramAPIHash),
		uploads.WorkerSettings{
			WorkerID:      smokeWorkerID,
			LeaseDuration: 5 * time.Minute,
			UploadTimeout: 30 * time.Minute,
		},
	)
	if err != nil {
		return fmt.Errorf("initialize smoke worker: %w", err)
	}

	worked, err := worker.DrainOne(ctx)
	if err != nil {
		return fmt.Errorf("drain staged part: %w", err)
	}
	if !worked {
		return errUploadIncomplete
	}
	logger.Info("smoke staged part drained")
	return nil
}

func downloadFile(ctx context.Context, client *http.Client, cfg config, cookies cookies, fileID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/files/"+fileID+"/download", nil)
	if err != nil {
		return nil, err
	}
	addCookieHeader(req, cookies)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func postJSON(ctx context.Context, client *http.Client, url string, cookies cookies, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	if err != nil {
		return err
	}
	addAuthHeaders(req, cookies)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func addAuthHeaders(req *http.Request, cookies cookies) {
	addCookieHeader(req, cookies)
	req.Header.Set("X-CSRF-Token", cookies.CSRF)
}

func addCookieHeader(req *http.Request, cookies cookies) {
	req.Header.Set("Cookie", "td_refresh="+cookies.Refresh+"; td_csrf="+cookies.CSRF)
}

func readCookies(path string) (cookies, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return cookies{}, err
	}

	var out cookies
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#HttpOnly_") {
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		switch fields[5] {
		case "td_refresh":
			out.Refresh = fields[6]
		case "td_csrf":
			out.CSRF = fields[6]
		}
	}
	if out.Refresh == "" || out.CSRF == "" {
		return cookies{}, errors.New("cookie jar must contain td_refresh and td_csrf")
	}
	return out, nil
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
