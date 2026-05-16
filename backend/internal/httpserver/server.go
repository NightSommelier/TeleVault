package httpserver

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"filippo.io/age"
	"github.com/televault/TeleVault/backend/internal/auth"
	"github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/crypto/secrets"
	"github.com/televault/TeleVault/backend/internal/db"
	"github.com/televault/TeleVault/backend/internal/files"
	"github.com/televault/TeleVault/backend/internal/uploads"
)

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
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /readyz", s.readyz)

	telegramSessionCrypto := auth.NewTelegramSessionCrypto(s.secrets)
	authHandler := auth.NewHandler(s.cfg, s.logger, s.db, telegramSessionCrypto, s.telegram)
	s.mux.HandleFunc("POST /auth/telegram/send-code", authHandler.SendTelegramCode)
	s.mux.HandleFunc("POST /auth/telegram/login", authHandler.LoginWithTelegram)
	s.mux.Handle("POST /auth/refresh", authHandler.RequireCSRF(http.HandlerFunc(authHandler.Refresh)))
	s.mux.Handle("POST /auth/logout", authHandler.RequireCSRF(http.HandlerFunc(authHandler.Logout)))
	s.mux.Handle("GET /me", authHandler.RequireAuth(http.HandlerFunc(authHandler.Me)))

	filesHandler := files.NewHandler(s.db, telegramSessionCrypto, s.ageIdentity, s.telegram)
	s.mux.Handle("GET /files", authHandler.RequireAuth(http.HandlerFunc(filesHandler.List)))
	s.mux.Handle("POST /folders", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(filesHandler.CreateFolder))))
	s.mux.Handle("GET /files/{id}", authHandler.RequireAuth(http.HandlerFunc(filesHandler.Get)))
	s.mux.Handle("GET /files/{id}/download", authHandler.RequireAuth(http.HandlerFunc(filesHandler.Download)))

	uploadsHandler := uploads.NewHandler(s.db, s.ageRecipient, telegramSessionCrypto, s.telegram)
	s.mux.Handle("POST /uploads", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.Create))))
	s.mux.Handle("POST /uploads/{id}/parts/{part_number}", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.UploadPart))))
	s.mux.Handle("POST /uploads/{id}/complete", authHandler.RequireAuth(authHandler.RequireCSRF(http.HandlerFunc(uploadsHandler.Complete))))
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
