// Command spellcheck serves the spellcheck HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/fadhilijuma/spellcheck/internal/httpapi"
	"github.com/fadhilijuma/spellcheck/internal/spell"
)

// tag is set at link time: go build -ldflags "-X main.tag=$(git rev-parse --short HEAD)".
var tag = "develop"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "SPELL")

	ctx := context.Background()

	if err := run(ctx, log); err != nil {
		log.Error("run", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {

	// -------------------------------------------------------------------------
	// Configuration

	cfg := struct {
		conf.Version
		DefaultLanguage spell.Language `conf:"default:en-US"`
		MaxSuggestions  int            `conf:"default:5"`
		MaxDistance     uint32         `conf:"default:2"`
		PrefixLength    uint32         `conf:"default:7"`
		Web             struct {
			ReadTimeout        time.Duration `conf:"default:5s"`
			WriteTimeout       time.Duration `conf:"default:10s"`
			IdleTimeout        time.Duration `conf:"default:120s"`
			ShutdownTimeout    time.Duration `conf:"default:20s"`
			APIHost            string        `conf:"default:0.0.0.0:3000"`
			CORSAllowedOrigins []string      // empty: CORS off; the editor is served through a /v1 proxy
		}
	}{
		Version: conf.Version{
			Build: tag,
			Desc:  "Spellcheck API",
		},
	}

	const prefix = "SPELL"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) || errors.Is(err, conf.ErrVersionWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	// -------------------------------------------------------------------------
	// App Starting

	log.Info("starting service", "version", cfg.Build)

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Info("startup", "config", out)

	// -------------------------------------------------------------------------
	// Load the dictionaries

	log.Info("startup", "status", "loading dictionaries")

	checkers := make(map[spell.Language]*spell.Checker)
	checkerConfig := spell.Config{
		MaxSuggestions: cfg.MaxSuggestions,
		MaxDistance:    cfg.MaxDistance,
		PrefixLength:   cfg.PrefixLength,
	}
	for _, language := range []spell.Language{spell.EnglishUS, spell.EnglishGB} {
		checker, err := spell.NewEnglish(language, checkerConfig)
		if err != nil {
			return fmt.Errorf("load dictionary %s: %w", language, err)
		}
		checkers[language] = checker
		log.Info("startup", "status", "dictionary loaded", "language", language)
	}

	// -------------------------------------------------------------------------
	// Start API Service

	log.Info("startup", "status", "initializing V1 API support")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	handler := httpapi.New(log, cfg.DefaultLanguage, cfg.Build, cfg.Web.CORSAllowedOrigins, checkers)

	api := http.Server{
		Addr:         cfg.Web.APIHost,
		Handler:      handler,
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Info("startup", "status", "api started", "host", api.Addr)

		serverErrors <- api.ListenAndServe()
	}()

	// -------------------------------------------------------------------------
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info("shutdown", "status", "shutdown started", "signal", sig)

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

		if err := api.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown server: %w", errors.Join(err, api.Close()))
		}

		log.Info("shutdown", "status", "shutdown complete")
	}

	return nil
}
