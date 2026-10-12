// Package httpapi exposes the spellcheck service over HTTP: routes,
// middleware, JSON bodies and error responses. Spelling rules live in package
// spell; this package only translates between HTTP and Go values.
package httpapi

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/fadhilijuma/spellcheck/internal/spell"
)

type app struct {
	defaultLanguage spell.Language
	build           string
	checkers        map[spell.Language]*spell.Checker
}

// New returns the service's HTTP handler with all routes and middleware.
// Health probes are not logged: the container runtime calls them every few
// seconds. An empty corsOrigins list disables CORS.
func New(log *slog.Logger, defaultLanguage spell.Language, build string, corsOrigins []string, checkers map[spell.Language]*spell.Checker) http.Handler {
	a := app{
		defaultLanguage: cmp.Or(defaultLanguage, spell.EnglishUS),
		build:           build,
		checkers:        checkers,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/config", a.configuration)
	mux.HandleFunc("GET /v1/liveness", a.liveness)
	mux.HandleFunc("GET /v1/readiness", a.readiness)
	mux.Handle("POST /v1/check", handleRequest(log, a.check))

	h := recoverPanics(log, mux)
	h = cors(corsOrigins, h)
	h = cleanPath(h)

	return h
}

// =============================================================================

// configuration exposes the initial editor convention, not operational settings.
func (a *app) configuration(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, configurationResponse{DefaultLanguage: a.defaultLanguage})
}

// liveness reports that the process is up, which build it runs and where.
func (a *app) liveness(w http.ResponseWriter, _ *http.Request) {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	writeJSON(w, http.StatusOK, livenessResponse{
		Status: "up",
		Build:  a.build,
		Host:   host,
	})
}

// readiness reports that the service can take traffic. The dictionary and its
// index are built before the server starts, so a running server is ready.
func (a *app) readiness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// check writes misspelled words and suggestions, omitting valid words.
// Failures are returned with an HTTP status for the request boundary to handle.
func (a *app) check(w http.ResponseWriter, r *http.Request) (int, error) {
	req, err := decodeCheck(w, r)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decode check request: %w", err)
	}

	words, err := toWords(req)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("parse check words: %w", err)
	}

	language, err := spell.ParseLanguage(cmp.Or(req.Language, string(a.defaultLanguage)))
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("parse check language: %w", err)
	}
	checker := a.checkers[language]
	if checker == nil {
		return http.StatusInternalServerError, fmt.Errorf("checker for %s is not configured", language)
	}
	results, err := checker.Check(r.Context(), words)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("check words: %w", err)
	}

	writeJSON(w, http.StatusOK, toCheckResponse(results))
	return http.StatusOK, nil
}
