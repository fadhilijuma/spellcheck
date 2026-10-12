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

// Config is what the handlers need.
type Config struct {
	DefaultLanguage spell.Language
	Build           string
	CORSOrigins     []string // browser origins allowed to call the API; empty turns CORS off
	Checkers        map[spell.Language]*spell.Checker
}

type app struct {
	defaultLanguage spell.Language
	log             *slog.Logger
	build           string
	checkers        map[spell.Language]*spell.Checker
}

// New returns the service's HTTP handler with all routes and middleware.
// Health probes are not logged: the container runtime calls them every few
// seconds.
func New(log *slog.Logger, cfg Config) http.Handler {
	a := app{
		defaultLanguage: cmp.Or(cfg.DefaultLanguage, spell.EnglishUS),
		log:             log,
		build:           cfg.Build,
		checkers:        cfg.Checkers,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/config", a.configuration)
	mux.HandleFunc("GET /v1/liveness", a.liveness)
	mux.HandleFunc("GET /v1/readiness", a.readiness)
	mux.Handle("POST /v1/check", logRequests(log, http.HandlerFunc(a.check)))

	h := recoverPanics(log, mux)
	h = cors(cfg.CORSOrigins, h)
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

// check returns the misspelled words among those sent, each with its
// suggestions; valid words are omitted.
func (a *app) check(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCheck(w, r)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	words, err := toWords(req)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	language, err := spell.ParseLanguage(cmp.Or(req.Language, string(a.defaultLanguage)))
	if err != nil {
		a.respondError(w, r, badRequest(err))
		return
	}
	checker := a.checkers[language]
	if checker == nil {
		a.respondError(w, r, fmt.Errorf("checker for %s is not configured", language))
		return
	}
	results, err := checker.Check(r.Context(), words)
	if err != nil {
		a.respondError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toCheckResponse(results))
}
