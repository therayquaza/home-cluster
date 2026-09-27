package main

import (
	"context"
	"embed"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"kommande/internal/config"
	dbpkg "kommande/internal/db"
	"kommande/internal/handlers"
	"kommande/internal/logging"
	"kommande/internal/middleware"
)

//go:embed templates static
var files embed.FS

func main() {
	logging.Init()
	cfg := config.Load()
	logConfig(cfg)

	client, err := dbpkg.Connect(cfg.MongoURI)
	if err != nil {
		logging.Fatal("mongodb connection failed", "uri", logging.RedactURI(cfg.MongoURI), "err", err)
	}

	database := client.Database(cfg.DBName)
	ctx := context.Background()

	ensureIndex(ctx, database, "users", "users_email_unique", mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	ensureIndex(ctx, database, "orders", "orders_email_date", mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}, {Key: "date", Value: -1}},
	})

	h, err := handlers.New(database, files, cfg)
	if err != nil {
		logging.Fatal("handler init failed", "err", err)
	}

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServer(http.FS(files)))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// OIDC auth flow
	mux.HandleFunc("GET /auth/login", h.OIDCLoginRedirect)
	mux.HandleFunc("GET /auth/callback", h.OIDCCallback)
	mux.HandleFunc("POST /logout", h.Logout)

	// User routes (require auth)
	mux.Handle("GET /", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.Index)))
	mux.Handle("GET /order", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.OrderPage)))
	mux.Handle("POST /order", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.SubmitOrder)))
	mux.Handle("GET /orders", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.MyOrders)))
	mux.Handle("GET /profile", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.ProfilePage)))
	mux.Handle("POST /profile", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.UpdateProfile)))
	mux.Handle("GET /images/{id}", middleware.RequireAuth(cfg.JWTSecret, http.HandlerFunc(h.ServeImage)))

	// Admin routes (require admin role)
	mux.Handle("GET /admin", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminDashboard)))
	mux.Handle("GET /admin/articles", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminArticles)))
	mux.Handle("GET /admin/articles/new", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminNewArticle)))
	mux.Handle("POST /admin/articles", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminCreateArticle)))
	mux.Handle("GET /admin/articles/{id}/edit", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminEditArticle)))
	mux.Handle("POST /admin/articles/{id}", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminUpdateArticle)))
	mux.Handle("POST /admin/articles/{id}/delete", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminDeleteArticle)))
	mux.Handle("GET /admin/categories", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminCategories)))
	mux.Handle("POST /admin/categories", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminCreateCategory)))
	mux.Handle("GET /admin/categories/{id}/edit", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminEditCategory)))
	mux.Handle("POST /admin/categories/{id}", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminUpdateCategory)))
	mux.Handle("POST /admin/categories/{id}/delete", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminDeleteCategory)))
	mux.Handle("GET /admin/orders", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminOrders)))
	mux.Handle("POST /admin/orders/{id}/respond", middleware.RequireAdmin(cfg.JWTSecret, http.HandlerFunc(h.AdminRespondOrder)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      logging.RecoverPanic(logging.RequestLog(logging.Headers(mux))),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logging.Log.Info("kommande listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Fatal("server stopped", "err", err)
		}
	}()

	<-sigCtx.Done()
	logging.Log.Info("shutdown signal received, draining connections")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logging.Log.Error("graceful shutdown failed", "err", err)
	}
	if err := client.Disconnect(shutCtx); err != nil {
		logging.Log.Error("mongodb disconnect failed", "err", err)
	}
	logging.Log.Info("shutdown complete")
}

// ensureIndex creates an index and records the outcome. These calls used to
// discard their errors, so a failed unique index on users.email was invisible.
func ensureIndex(ctx context.Context, db *mongo.Database, collection, index string, model mongo.IndexModel) {
	if _, err := db.Collection(collection).Indexes().CreateOne(ctx, model); err != nil {
		logging.Log.Error("index creation failed", "collection", collection, "index", index, "err", err)
		return
	}
	logging.Log.Debug("index ensured", "collection", collection, "index", index)
}

// logConfig records the effective configuration at boot. Secrets are reduced to
// a presence marker and the Mongo URI is stripped of its password.
func logConfig(cfg *config.Config) {
	logging.Log.Info("config http", "port", cfg.Port, "base_url", cfg.BaseURL)
	logging.Log.Info("config mongo", "uri", logging.RedactURI(cfg.MongoURI), "db", cfg.DBName)
	logging.Log.Info("config oidc", "issuer", cfg.OIDCIssuer, "client_id", cfg.OIDCClientID,
		"redirect_url", cfg.OIDCRedirectURL, "client_secret", logging.Set(cfg.OIDCClientSecret))
	logging.Log.Info("config smtp", "host", cfg.SMTPHost, "port", cfg.SMTPPort,
		"from", cfg.SMTPFrom, "user", cfg.SMTPUser, "password", logging.Set(cfg.SMTPPassword))
	logging.Log.Info("config auth", "jwt_secret", logging.Set(cfg.JWTSecret), "admin_email", cfg.AdminEmail)
	if cfg.JWTSecret == config.DefaultJWTSecret {
		logging.Log.Warn("JWT_SECRET is still the built-in default; every session cookie is forgeable")
	}
}
