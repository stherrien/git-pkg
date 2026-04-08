package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stherrien/git-pkg/internal/cache"
	"github.com/stherrien/git-pkg/internal/github"
	"github.com/stherrien/git-pkg/internal/handler"
	"github.com/stherrien/git-pkg/internal/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	cacheTTL := 5 * time.Minute

	gh := github.NewClient(ghToken)
	c := cache.New(cacheTTL)

	mux := http.NewServeMux()
	h := handler.New(gh, c, logger)
	h.RegisterRoutes(mux)

	// 100 requests/min per IP, burst of 20
	limiter := ratelimit.New(100.0/60.0, 20)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      limiter.Middleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	logger.Info("server stopped")
}
