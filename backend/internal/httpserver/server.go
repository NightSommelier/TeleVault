package httpserver

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"filippo.io/age"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/adminsettings"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/buildinfo"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/db"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/files"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/recovery"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/uploads"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/valkey"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	cfg          config.Config
	logger       *slog.Logger
	db           *sql.DB
	secrets      *secrets.Box
	ageRecipient age.Recipient
	ageIdentity  age.Identity
	telegram     telegramClient
	mux          *http.ServeMux
}

type telegramClient interface {
	auth.TelegramAuthClient
	auth.TelegramStorageClient
}

func New(cfg config.Config, logger *slog.Logger, database *sql.DB, secretsBox *secrets.Box, ageRecipient age.Recipient, ageIdentity age.Identity, telegram telegramClient) http.Handler {
	server := &Server{
		cfg:          cfg,
		logger:       logger,
		db:           database,
		secrets:      secretsBox,
		ageRecipient: ageRecipient,
		ageIdentity:  ageIdentity,
		telegram:     telegram,
		mux:          http.NewServeMux(),
	}

	server.routes()

	return server.requestLogger(server.securityHeaders(server.cors(server.mux)))
}

func (s *Server) routes() {
	s.mountStatic()

	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /readyz", s.readyz)
	s.mux.HandleFunc("GET /app-info", s.appInfo)

	telegramSessionCrypto := auth.NewTelegramSessionCrypto(s.secrets)
	var rateLimitStore auth.RateLimitStore
	if s.cfg.AuthRateLimitEnabled && s.cfg.ValkeyAddr != "" {
		rateLimitStore = auth.NewValkeyRateLimitStore(valkey.NewClient(s.cfg.ValkeyAddr), "t2d:auth_rate_limit")
	}
	authHandler := auth.NewHandlerWithRateLimiter(s.cfg, s.logger, s.db, telegramSessionCrypto, s.telegram, rateLimitStore)
	s.mux.HandleFunc("POST /auth/telegram/send-code", authHandler.SendTelegramCode)
	s.mux.HandleFunc("POST /auth/telegram/login", authHandler.LoginWithTelegram)
	s.mux.HandleFunc("POST /auth/telegram/qr/start", authHandler.StartTelegramQRLogin)
	s.mux.HandleFunc("POST /auth/telegram/qr/complete", authHandler.CompleteTelegramQRLogin)
	s.mux.Handle("POST /auth/refresh", authHandler.RequireCSRF(http.HandlerFunc(authHandler.Refresh)))
	s.mux.Handle("POST /auth/logout", authHandler.RequireCSRF(http.HandlerFunc(authHandler.Logout)))
	s.mux.Handle("GET /me", authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)))

	filesHandler := files.NewHandler(s.db, telegramSessionCrypto, s.ageIdentity, s.telegram)
	s.mux.Handle("GET /files", authHandler.RequireAuth(http.HandlerFunc(filesHandler.List)))
	s.mux.Handle("GET /shared", authHandler.RequireAuth(http.HandlerFunc(filesHandler.ListSharedWithMe)))
	s.mux.Handle("POST /folders", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.CreateFolder))))
	s.mux.Handle("GET /files/{id}", authHandler.RequireAuth(http.HandlerFunc(filesHandler.Get)))
	s.mux.Handle("PATCH /files/{id}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.Patch))))
	s.mux.Handle("DELETE /files/{id}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.Delete))))
	s.mux.Handle("PATCH /files/bulk-move", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.BulkMove))))
	s.mux.Handle("POST /files/bulk-delete", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.BulkDelete))))
	s.mux.Handle("GET /files/{id}/download", authHandler.RequireAuth(http.HandlerFunc(filesHandler.Download)))
	s.mux.Handle("GET /files/{id}/shares", authHandler.RequireAuth(http.HandlerFunc(filesHandler.ListShares)))
	s.mux.Handle("POST /files/{id}/shares", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.CreateShare))))
	s.mux.Handle("DELETE /files/{id}/shares/{share_id}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.RevokeShare))))
	s.mux.Handle("GET /files/{id}/public-links", authHandler.RequireAuth(http.HandlerFunc(filesHandler.ListPublicLinks)))
	s.mux.Handle("POST /files/{id}/public-links", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.CreatePublicLink))))
	s.mux.Handle("DELETE /files/{id}/public-links/{link_id}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.RevokePublicLink))))
	s.mux.HandleFunc("GET /public/{token}", filesHandler.PublicMetadata)
	s.mux.HandleFunc("GET /public/{token}/download", filesHandler.PublicDownload)
	s.mux.HandleFunc("POST /public/{token}/download", filesHandler.PublicDownload)

	adminSettingsStore := adminsettings.NewStore(s.db, s.cfg)
	uploadsHandler := uploads.NewHandler(s.db, s.ageRecipient, telegramSessionCrypto, s.telegram, uploads.Settings{
		PartSize:   s.cfg.UploadPartSizeBytes,
		StagingDir: s.cfg.UploadStagingDir,
		EffectiveSettingsProvider: func(ctx context.Context, userID string) (uploads.EffectiveSettings, error) {
			settings, err := adminSettingsStore.EffectiveUploadSettings(ctx, userID)
			if err != nil {
				return uploads.EffectiveSettings{}, err
			}
			return uploads.EffectiveSettings{
				PartSize:                     settings.UploadPartSizeBytes,
				MaxParallelUploads:           settings.MaxParallelUploads,
				TargetUploadBytesPerSecond:   settings.TargetUploadBytesPerSecond,
				CooldownBetweenPartsMillisec: settings.CooldownBetweenPartsMillisec,
			}, nil
		},
	})
	s.mux.Handle("POST /uploads", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.Create))))
	s.mux.Handle("GET /uploads/{id}", authHandler.RequireAuth(http.HandlerFunc(uploadsHandler.Get)))
	s.mux.Handle("POST /uploads/{id}/parts/{part_number}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.UploadPart))))
	s.mux.Handle("POST /uploads/{id}/complete", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.Complete))))
	s.mux.Handle("DELETE /uploads/{id}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.Cancel))))

	recoveryHandler := recovery.NewHandler(s.db, s.secrets)
	s.mux.Handle("POST /recovery/export", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(recoveryHandler.Export))))
	s.mux.Handle("POST /recovery/import", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(recoveryHandler.Import))))

	adminHandler := adminsettings.NewHandler(s.db, s.cfg)
	s.mux.Handle("GET /admin/settings", authHandler.RequireAuth(authHandler.RequireAdmin(http.HandlerFunc(adminHandler.GetSettings))))
	s.mux.Handle("PATCH /admin/settings/upload", authHandler.RequireAuth(authHandler.RequireAdmin(authHandler.RequireCSRF(http.HandlerFunc(adminHandler.PatchUploadSettings)))))
	s.mux.Handle("PATCH /admin/telegram-accounts/{user_id}/limits", authHandler.RequireAuth(authHandler.RequireAdmin(authHandler.RequireCSRF(http.HandlerFunc(adminHandler.PatchTelegramAccountLimit)))))
}

func (s *Server) appInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":  "TeleVault",
		"debug": s.cfg.AppDebug,
		"build": buildinfo.Info(),
	})
}

func (s *Server) mountStatic() {
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(staticRoot))
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", fileServer))
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFileFS(w, r, staticRoot, "index.html")
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(r.Context(), s.db); err != nil {
		s.logger.Warn("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(s.cfg.CORSAllowedOrigins))
	for _, origin := range s.cfg.CORSAllowedOrigins {
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				if s.cfg.CredentialsCORSMode {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}
