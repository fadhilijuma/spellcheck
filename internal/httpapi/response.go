package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/fadhilijuma/spellcheck/internal/spell"
)

// errorResponse is the JSON body of an HTTP failure, not a Go error type.
type errorResponse struct {
	Error string `json:"error"`
}

// configurationResponse contains the editor convention and request limits.
type configurationResponse struct {
	DefaultLanguage spell.Language `json:"defaultLanguage"`
	MaxWordLen      int            `json:"maxWordLen"`
	MaxWords        int            `json:"maxWords"`
	MaxBodyBytes    int64          `json:"maxBodyBytes"`
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
	Status string `json:"status"`
	Build  string `json:"build"`
	Host   string `json:"host"`
}

// =============================================================================

func toCheckResponse(results []spell.Result) checkResponse {
	resp := checkResponse{Misspelled: []misspelling{}}

	for _, r := range results {
		if r.Valid {
			continue
		}

		resp.Misspelled = append(resp.Misspelled, misspelling{Word: r.Word, Suggestions: r.Suggestions})
	}

	return resp
}

// writeJSON writes v as the JSON body with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// The bodies are plain structs, so encoding cannot fail; a write error
	// means the client has gone and there is no one to tell.
	_ = json.NewEncoder(w).Encode(v)
}
