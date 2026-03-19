package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drujensen/calorie-count/internal/config"
	"github.com/drujensen/calorie-count/internal/db"
	"github.com/drujensen/calorie-count/internal/handlers"
	"github.com/drujensen/calorie-count/internal/middleware"
	"github.com/drujensen/calorie-count/internal/migrations"
	"github.com/drujensen/calorie-count/internal/repositories"
	"github.com/drujensen/calorie-count/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	// Configure structured logging.
	var handler slog.Handler
	if cfg.Env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(handler))

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("opening database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.RunMigrations(database, migrations.FS); err != nil {
		slog.Error("running migrations", "error", err)
		os.Exit(1)
	}

	// Dependency injection
	userRepo := repositories.NewUserRepository(database)
	sessionRepo := repositories.NewSessionRepository(database)
	logRepo := repositories.NewLogRepository(database)
	weightRepo := repositories.NewWeightRepository(database)
	authSvc := services.NewAuthService(userRepo, sessionRepo)
	logSvc := services.NewLogService(logRepo, userRepo)
	weightSvc := services.NewWeightService(weightRepo, userRepo)

	isProduction := cfg.Env == "production"

	// Create a root context cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	aiSvc := services.NewAIService(cfg, ctx)

	// Rate limiter for auth routes (10 req/min per IP).
	rateLimiter := middleware.NewRateLimiter(ctx, 10)

	mux := http.NewServeMux()
	handlers.SetupRoutes(mux, authSvc, logSvc, aiSvc, weightSvc, userRepo, isProduction, rateLimiter)
	httpHandler := middleware.ApplyMiddleware(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Background session cleanup — runs every hour.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sessionRepo.DeleteExpired(context.Background()); err != nil {
					slog.Error("deleting expired sessions", "error", err)
				}
			}
		}
	}()

	go func() {
		slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}
