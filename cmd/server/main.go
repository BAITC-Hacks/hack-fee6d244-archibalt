// Command server — Archibalt: JSON API + раздача фронта.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	httpapi "github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/http"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/rating"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

type config struct {
	port, databaseURL, openAIKey, openAIModel, seedDir, staticDir, demoOTP string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	return config{
		port:        env("PORT", "8080"),
		databaseURL: env("DATABASE_URL", "postgres://postgres:hack@localhost:5432/hack?sslmode=disable"),
		openAIKey:   os.Getenv("OPENAI_API_KEY"),
		openAIModel: env("OPENAI_MODEL", "gpt-4.1-mini"),
		seedDir:     env("SEED_DIR", "./seed"),
		staticDir:   env("STATIC_DIR", "./web/dist"),
		demoOTP:     env("DEMO_OTP", httpapi.DefaultDemoOTP), // реальной отправки кода нет
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}
	if err := st.SeedIfEmpty(ctx, cfg.seedDir, rating.Compute); err != nil {
		return err
	}
	if n, err := st.RecomputeRatings(ctx, rating.Compute); err != nil {
		return err
	} else {
		slog.Info("рейтинги пересчитаны текущей формулой", "tasks", n)
	}

	aiClient := ai.New(cfg.openAIKey, cfg.openAIModel) // без ключа — mock-режим

	srv := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           httpapi.NewHandler(st, aiClient, rating.Compute, cfg.staticDir, cfg.demoOTP),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      90 * time.Second, // AI-вызовы до ~10 с с повтором
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "ai_mode", aiClient.Mode(), "static", cfg.staticDir, "seed", cfg.seedDir)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shCtx)
}
