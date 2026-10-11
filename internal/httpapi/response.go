package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/fadhilijuma/spellcheck/internal/spell"
)

// configurationResponse contains the initial editor convention.
type configurationResponse struct {
	DefaultLanguage spell.Language `json:"defaultLanguage"`
}

// misspelling is a misspelled word and its suggestions.
type misspelling struct {
	Word        string   `json:"word"`
	Suggestions []string `json:"suggestions"`
}

// checkResponse is the body of a 200 from POST /v1/check. Valid words are
// omitted; Misspelled is [] when all words are valid, never null.
type checkResponse struct {
	Misspelled []misspelling `json:"misspelled"`
}

// livenessResponse is the body of GET /v1/liveness.
type livenessResponse struct {
	Status     string `json:"status"`
	Build      string `json:"build"`
	Host       string `json:"host"`
	GOMAXPROCS int    `json:"GOMAXPROCS"`
}

// errorResponse is the body of every error the API answers with.
type errorResponse struct {
	Error string `json:"error"`
}

// =============================================================================

// badRequestError is an error the client caused; its message is sent back.
type badRequestError struct {
	err error
}

func toCheckResponse(results []spell.Result) checkResponse {
	resp := checkResponse{Misspelled: []misspelling{}}

	for _, r := range results {
		if r.Valid {
			continue
		}

		suggestions := make([]string, 0, len(r.Suggestions))
		for _, s := range r.Suggestions {
			suggestions = append(suggestions, s.String())
		}

		resp.Misspelled = append(resp.Misspelled, misspelling{Word: r.Word.String(), Suggestions: suggestions})
	}

	return resp
}

func (e *badRequestError) Error() string {
	return e.err.Error()
}

func (e *badRequestError) Unwrap() error {
	return e.err
}

// badRequest marks err as caused by the request, so respondError answers 400.
func badRequest(err error) error {
	return &badRequestError{err: err}
}

// respondError is the one place that turns an error into a response:
//
//	400 the request is invalid; the message says why
//	413 the body is over maxBodyBytes
//	nothing when the client has gone
//	500 anything else; logged, details kept out of the response
func (a *app) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		badReq   *badRequestError
		tooLarge *http.MaxBytesError
	)

	switch {
	case errors.As(err, &badReq):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: badReq.Error()})

	case errors.As(err, &tooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: fmt.Sprintf("request body is larger than %d bytes", tooLarge.Limit)})

	case errors.Is(err, context.Canceled):
		// The client left; there is no one to answer.

	default:
		a.log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

// writeJSON writes v as the JSON body with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The bodies are plain structs, so encoding cannot fail; a write error
	// means the client has gone and there is no one to tell.
	_ = json.NewEncoder(w).Encode(v)
}
