// Command dinks serves the dinks backend API: authentication, cycle and symptom
// records, statistics, partner sharing, and account management.
package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"dinks/internal/config"
	"dinks/internal/db"
	"dinks/internal/handlers"
	"dinks/internal/httpx"
	"dinks/internal/logging"
	"dinks/internal/middleware"
	"dinks/internal/repository"
	"dinks/internal/service"
)

const sessionCookieName = "dinks_session"

func main() {
	logging.Init()
	cfg := config.Load()
	logConfig(cfg)

	ctx := context.Background()
	client, err := db.Connect(ctx, cfg.MongoURI)
	if err != nil {
		logging.Fatal("mongo connection failed", "err", err)
	}

	repo := repository.New(client, cfg.MongoDB)
	if err := repo.Migrate(ctx); err != nil {
		logging.Fatal("mongo migrate failed", "err", err)
	}
	logging.Log.Info("mongo schema migrated", "db", cfg.MongoDB)

	sessions := scs.New()
	sessions.Store = repository.NewSessionStore(client, cfg.MongoDB)
	sessions.Lifetime = 7 * 24 * time.Hour
	sessions.Cookie.Name = sessionCookieName
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = cfg.SecureCookies()
	sessions.Cookie.SameSite = http.SameSiteLaxMode

	h, err := handlers.New(repo, service.NewStats(), sessions, cfg)
	if err != nil {
		logging.Fatal("handler init failed", "issuer", cfg.OIDCIssuer, "err", err)
	}

	auth := middleware.NewAuth(sessions, h.MobileVerifier(), cfg.OIDCMobileClientID,
		func(r *http.Request, subject, name string) error {
			return repo.UpsertMember(r.Context(), subject, name)
		})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           routes(h, auth, sessions),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logging.Log.Info("dinks backend listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logging.Fatal("http server stopped", "err", err)
	}
}

// routes wires the mux and wraps it in the middleware that applies to every
// request. The session wrapper must be outermost of the three, since the auth
// middleware reads session data from the request context.
func routes(h *handlers.Handler, auth *middleware.Auth, sessions *scs.SessionManager) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Public: establishing or ending a session.
	mux.HandleFunc("GET /auth/login", h.Login)
	mux.HandleFunc("GET /auth/callback", h.Callback)
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.LoginPassword)
	mux.HandleFunc("POST /auth/logout", h.Logout)

	// Authenticated.
	guard := auth.Handler
	mux.Handle("GET /api/me", guard(http.HandlerFunc(h.Me)))
	mux.Handle("DELETE /api/me", guard(http.HandlerFunc(h.DeleteMe)))
	mux.Handle("GET /api/dashboard", guard(http.HandlerFunc(h.Dashboard)))
	mux.Handle("GET /api/prediction", guard(http.HandlerFunc(h.Prediction)))
	mux.Handle("GET /api/export", guard(http.HandlerFunc(h.Export)))
	mux.Handle("GET /api/stats", guard(http.HandlerFunc(h.Stats)))
	mux.Handle("POST /api/stats/query", guard(http.HandlerFunc(h.StatsQuery)))
	mux.Handle("POST /api/periods", guard(http.HandlerFunc(h.CreatePeriod)))
	mux.Handle("PATCH /api/periods/{id}", guard(http.HandlerFunc(h.UpdatePeriod)))
	mux.Handle("POST /api/symptoms", guard(http.HandlerFunc(h.CreateSymptom)))
	mux.Handle("PATCH /api/symptoms/{id}", guard(http.HandlerFunc(h.UpdateSymptom)))
	mux.Handle("DELETE /api/symptoms/{id}", guard(http.HandlerFunc(h.DeleteSymptom)))
	mux.Handle("POST /api/partners/invite", guard(http.HandlerFunc(h.CreateInvite)))
	mux.Handle("POST /api/partners/link", guard(http.HandlerFunc(h.RedeemInvite)))
	mux.Handle("GET /api/partners", guard(http.HandlerFunc(h.ListPartners)))
	mux.Handle("DELETE /api/partners/{subject}", guard(http.HandlerFunc(h.RevokePartner)))
	mux.Handle("GET /api/partners/status", guard(http.HandlerFunc(h.PartnerStatuses)))

	return sessions.LoadAndSave(httpx.RecoverPanic(logging.RequestLog(logging.Headers(mux))))
}

// logConfig dumps the effective configuration at boot so a misconfigured
// deployment is obvious in the first lines of the log. Secrets are reduced to a
// presence flag, and the Mongo URI is stripped of its password.
func logConfig(cfg config.Config) {
	logging.Log.Info("config http", "port", cfg.Port, "base_url", cfg.BaseURL, "dev_auth", cfg.DevAuth)
	logging.Log.Info("config oidc", "issuer", cfg.OIDCIssuer, "client_id", cfg.OIDCClientID,
		"mobile_client_id", cfg.OIDCMobileClientID, "redirect_url", cfg.OIDCRedirectURL,
		"client_secret", logging.Set(cfg.OIDCClientSecret))
	logging.Log.Info("config mongo", "uri", logging.RedactURI(cfg.MongoURI), "db", cfg.MongoDB)
	if cfg.DevAuth {
		logging.Log.Warn("DEV_AUTH is enabled in a non-local deployment; authentication is bypassed",
			"hint", "DEV_AUTH must never be true outside local development")
	}
	if strings.TrimSpace(cfg.OIDCIssuer) == "" && !cfg.DevAuth {
		logging.Log.Warn("OIDC_ISSUER is empty but DEV_AUTH is off; every login will fail")
	}
}
