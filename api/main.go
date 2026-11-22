// Command api is the Cotinha HTTP API. It also serves the built web app.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Config struct {
	Addr           string
	DatabaseURL    string
	PublicURL      string
	StaticDir      string
	CookieSecure   bool
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	MailFrom       string
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3Bucket       string
	S3UseSSL       bool
	S3Region       string
	S3CreateBucket bool
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() (Config, error) {
	c := Config{
		Addr:           env("ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		PublicURL:      strings.TrimRight(env("PUBLIC_URL", "http://localhost:5173"), "/"),
		StaticDir:      os.Getenv("STATIC_DIR"),
		CookieSecure:   env("COOKIE_SECURE", "false") == "true",
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPPort:       env("SMTP_PORT", "587"),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPass:       os.Getenv("SMTP_PASS"),
		MailFrom:       os.Getenv("MAIL_FROM"),
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:    os.Getenv("S3_SECRET_KEY"),
		S3Bucket:       env("S3_BUCKET", "cotinha"),
		S3UseSSL:       env("S3_USE_SSL", "true") == "true",
		S3Region:       env("S3_REGION", "auto"),
		S3CreateBucket: env("S3_CREATE_BUCKET", "false") == "true",
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.S3Endpoint == "" || c.S3AccessKey == "" || c.S3SecretKey == "" {
		return c, errors.New("S3_ENDPOINT, S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}
	return c, nil
}

type App struct {
	cfg   Config
	db    *pgxpool.Pool
	store *Storage
	mail  *Mailer
	log   *slog.Logger
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := connectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrate(ctx, db, log); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	store, err := NewStorage(ctx, cfg)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	app := &App{cfg: cfg, db: db, store: store, mail: NewMailer(cfg, log), log: log}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		db, err := pgxpool.New(ctx, url)
		if err == nil {
			if err = db.Ping(ctx); err == nil {
				return db, nil
			}
			db.Close()
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("connect to database: %w", lastErr)
}

// migrate applies every embedded migrations/*.sql file not yet recorded, in name order.
func migrate(ctx context.Context, db *pgxpool.Pool, log *slog.Logger) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Info("migration applied", "name", name)
	}
	return nil
}

func (a *App) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"ok": "true"}) })
		r.Post("/auth/request-code", a.handleRequestCode)
		r.Post("/auth/verify", a.handleVerifyCode)
		r.Post("/auth/logout", a.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(a.requireUser)
			r.Get("/me", a.handleMe)
			r.Patch("/me", a.handleUpdateMe)
			r.Post("/me/avatar", a.handleUploadAvatar)
			r.Get("/handles/available", a.handleHandleAvailable)

			r.Get("/users/search", a.handleSearchUsers)
			r.Get("/users/by-handle/{handle}", a.handleUserByHandle)
			r.Get("/users/{id}/pix", a.handlePixCode)

			r.Get("/friends", a.handleListFriends)
			r.Delete("/friends/{id}", a.handleRemoveFriend)
			r.Get("/friend-requests", a.handleListRequests)
			r.Post("/friend-requests", a.handleSendRequest)
			r.Post("/friend-requests/{id}/{action}", a.handleRequestAction)
			r.Delete("/request-blocks/{id}", a.handleUnblock)

			r.Get("/cotinhas", a.handleListCotinhas)
			r.Post("/cotinhas", a.handleCreateCotinha)
			r.Get("/cotinhas/{id}", a.handleGetCotinha)
			r.Post("/cotinhas/{id}/photos", a.handleUploadPhoto)
			r.Delete("/cotinhas/{id}/photos/{photoId}", a.handleDeletePhoto)

			r.Get("/balances", a.handleBalances)
			r.Get("/balances/{id}", a.handleBalanceWith)
			r.Post("/balances/{id}/compensate", a.handleCompensate)

			r.Post("/payments", a.handleCreatePayment)
			r.Get("/payments/to-confirm", a.handlePaymentsToConfirm)
			r.Get("/payments/{id}", a.handleGetPayment)
			r.Post("/payments/{id}/confirm", a.handleConfirmPayment)
			r.Post("/payments/{id}/reject", a.handleRejectPayment)

			r.Get("/stats", a.handleStats)

			r.Get("/notifications", a.handleListNotifications)
			r.Get("/notifications/unread", a.handleUnreadCount)
			r.Post("/notifications/read", a.handleReadNotifications)
		})
		r.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "not found") })
	})

	if a.cfg.StaticDir != "" {
		r.Handle("/*", spaHandler(a.cfg.StaticDir))
	}
	return r
}

// spaHandler serves the built web app and falls back to index.html for client routes.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Clean("/" + r.URL.Path)
		if info, err := os.Stat(filepath.Join(dir, p)); err == nil && !info.IsDir() {
			if strings.HasPrefix(p, "/_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
