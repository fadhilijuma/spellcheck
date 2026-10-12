# Spellcheck Project Structure

Use this guide when planning, implementing or reviewing Spellcheck. It describes
this checkout, not a general service framework. Read [AGENTS.md](../AGENTS.md)
for project rules and [README.md](../README.md) for behavior and configuration.
Inspect the implementation before changing it; documentation is not a substitute
for reading callers and tests.

## Overview

One Go API checks English words. A Vue/Vuetify editor handles typing, inline
corrections and browser-local preferences. There is no database or account service.

```text
spellcheck/
├── cmd/spellcheck/main.go       # Configuration, wiring, startup and shutdown
├── internal/
│   ├── spell/
│   │   ├── spell.go            # Checker, index construction and suggestions
│   │   ├── word.go             # Word value and input parsing
│   │   ├── dict.go             # Language, embedded lists and shared parser
│   │   ├── *_test.go           # Adjacent table-driven domain tests
│   │   └── languages/          # US/British word lists and acronym allowlist
│   └── httpapi/
│       ├── httpapi.go          # Constructor, routes and check handler
│       ├── request.go          # JSON decoding and word validation
│       ├── response.go         # JSON response models and encoding
│       ├── middleware.go       # HTTP errors, panic recovery, CORS and paths
│       └── httpapi_test.go     # Table-driven HTTP contract tests
├── web/
│   ├── src/
│   │   ├── App.vue             # Editor state, language and personal dictionary
│   │   ├── components/         # WritingEditor and inline correction controls
│   │   ├── api.js              # Configuration, batching and response validation
│   │   ├── session.js          # Debounce, cancellation and stale-result handling
│   │   ├── tokenize.js         # Word extraction and exact replacement
│   │   ├── hover.js            # Correction-popup interaction helpers
│   │   ├── dictionary.js       # Browser-local accepted-word normalization
│   │   └── *.test.js           # Adjacent frontend unit tests
│   └── tests/editor.spec.js    # Browser tests against the real API
├── prompts/                    # Reusable task prompts and this guide
├── .agents/skills/              # Installed agent skills
├── .github/workflows/           # CI checks
├── AGENTS.md
├── README.md
├── Makefile
├── Dockerfile
├── compose.yaml
├── go.mod
└── go.sum
```

## Dependency Direction

```text
cmd/spellcheck → internal/httpapi → internal/spell
              └─────────────────→ internal/spell
web/src       → /v1 JSON contract
```

The command constructs checkers and injects them into the HTTP adapter.
Spelling never imports HTTP or the command. The browser does not own dictionary
ranking; its personal dictionary filters warnings locally. Keep these boundaries
without adding a service layer or interfaces for single concrete implementations.

## Command Entry Point

`main` creates a background context and a stdout JSON `log/slog` logger with
service `SPELL`. It calls `run(ctx context.Context, log *slog.Logger) error`.
Failures are logged at that boundary and exit with status 1.

Keep `run` sections in this order:

1. **Configuration:** the anonymous struct embeds `conf.Version` and uses
   `ardanlabs/conf/v3` with the `SPELL` prefix. Help/version sentinels share
   a branch that prints the requested output and returns success.
2. **App Starting:** log the build and parsed configuration.
3. **Load the dictionaries:** construct one checker for `en-US` and one for
   `en-GB`, with shared spelling settings. Construction errors stop startup.
4. **Start API Service:** pass settings and checkers directly to `httpapi.New`,
   configure the HTTP server, register SIGINT/SIGTERM and start serving in one
   goroutine reporting to a buffered error channel.
5. **Shutdown:** select server failure or signal. On a signal, call `Shutdown`
   with the configured timeout; force-close if graceful shutdown fails.

Use stage separator comments only in `main.go`. Keep spelling, request parsing
and editor rules out of the entry point. There is no debug service.

## Spelling Models and Index Lifecycle

| Type | Responsibility |
|---|---|
| `Language` | Supported English convention, parsed from text |
| `Word` | Validated spelling input with a private value and meaningful `String()` |
| `Config` | Suggestion count, edit-distance limit and indexed prefix length |
| `Checker` | Read-only spelling index and acronym allowlist after construction |
| `Result` | Input word, validity and concrete suggested words |

`NewEnglish` selects directly embedded US/British text and the shared acronym
list. `New` parses word/frequency entries and builds an `eskriett/spell`
SymSpell index. Dictionary and acronym parsing share the word-list scanner.
Do not hand-edit generated `languages/en-*.txt`; maintain acronyms deliberately.

Build both indexes once at startup, not during a request. Each request selects
an existing checker. `net/http` supplies request concurrency; immutable checkers
can be shared. Do not introduce per-word goroutines or mutate shared indexes.

`Check` consumes validated `[]Word` and returns `[]Result` in input order:

1. Honor context cancellation between lookups and reject a zero-value word.
2. Accept exact curated acronyms as written.
3. Normalize case and apostrophes for dictionary membership.
4. For unknown words, query the deletion index for closest-distance candidates.
5. Rank those candidates by frequency descending, then alphabetically for ties;
   apply the suggestion limit and preserve uppercase or initial-capital style.

The package supplies optimal string alignment distance, not a handwritten
Levenshtein implementation or an exhaustive dictionary scan. Prefix length
limits candidate indexing; the final distance check uses the complete word.
Only the nearest available distance within the limit is returned, not every
candidate at every distance. Suggestions are not context-aware grammar checks.
Cancellation is checked between lookups; an in-progress library lookup cannot
be interrupted.

## HTTP Boundary and Errors

`httpapi.New` receives the logger, default language, build, CORS origins, request
limits and checker map. It validates the default language and positive request
limits at construction. Do not introduce a duplicate HTTP config type.

| Endpoint | Responsibility |
|---|---|
| `GET /v1/config` | Default language and word-length/count/body-byte limits |
| `POST /v1/check` | Validated words to misspellings and ranked suggestions |
| `GET /v1/liveness` | Process/build information |
| `GET /v1/readiness` | Ready after indexes have been constructed |

Decode one JSON object, reject unknown fields and trailing data, limit body bytes
and enforce word count. `ParseWord` checks Unicode length and spelling syntax at
the input boundary; the checker does not repeat the length scan. Empty/omitted
request language resolves to the configured default; unsupported values fail.

Keep short operation context with `%w`. Spelling code returns errors, not HTTP
statuses or logs. HTTP handler adaptation maps errors to responses, logs failures
once, suppresses responses for canceled clients and redacts internal details.
Oversized bodies return 413; invalid input returns 400. Successful all-valid
checks return an empty JSON array, not null. Standard `ServeMux` 404/405 responses
remain plain text. No additional error framework is needed for this contract.

CORS is off by default; development and container setups proxy `/v1` through
the editor's origin. Do not log submitted words or draft text.

## Editor Request and Correction Lifecycle

The editor fetches `/v1/config` and validates settings once before checking.
A saved language choice overrides the configured default; both variants remain
available. User-facing limits live in Go configuration and reach the browser
through the existing contract, not duplicated frontend defaults.

Typing updates a debounced session. New edits cancel pending work and invalidate
older responses, including responses whose transport ignores cancellation.
Composition waits for commit. Checks also have a deadline. Tokenization skips
URLs, emails and numbers, splits hyphens and retains internal apostrophes.

Unique words are batched by count and actual UTF-8 encoded JSON size, including
language and punctuation. Batches are checked sequentially with cancellation
between calls. Validate response shape and membership before displaying results.
Oversized words are reported as unchecked; failed or unencodable checks must
not display All clear.

Suggestions appear at the underlined word through hover, tap or keyboard access.
Corrections replace exact same-case occurrences, preserve selection and support
Undo. Keep input rendered as text, never trusted HTML. Avoid a separate
suggestions list, submit button or decorative explanatory sections.

Add to dictionary stores accepted words in browser `localStorage`; Dictionary
allows removal. Matching is whole-word, case-insensitive and normalizes curly
apostrophes. It applies across both English variants, but does not modify server
indexes or accept arbitrary inflections. When storage fails, accepted words can
remain session-local. Persist accepted words and the language preference only,
never drafts. Words still go to the configured API for checking.

## Tests and Verification

Keep relevant Go tests beside their internal packages, following the existing
`[]Table`, `run` and `cmpDiff` pattern. Use concrete fixtures for word syntax,
language, ranking, acronym behavior, configuration, cancellation and HTTP
contracts. Do not add tests beside `main.go` or generic error-wrapper suites.

Frontend helpers have adjacent unit tests. Browser regressions exercise the
real API/editor for typing, correction, caret, Undo, language, personal words
and keyboard/touch behavior. Chromium, Firefox and WebKit coverage is automated;
manual screen-reader and OS-specific input-method testing remain distinct.

Run `make check` for formatting, vet, staticcheck, builds, Go race tests and
frontend unit/browser tests. Browser tests use ports 3015/3016. `make test` also
runs vulnerability scans. Report checks actually executed and any limitations;
a build alone is not browser verification. Build outputs are generated artifacts,
not source files to commit.

## Change Checklist

- State the requested behavior and owning package before changing contracts.
- Follow the cross-package/API go approval rule in `AGENTS.md`.
- Add a relevant failing regression before implementing a behavior change.
- Validate at the owning boundary; avoid duplicated hot-path checks.
- Preserve index reuse, deterministic ranking and cancellation behavior.
- Preserve byte/count limits, stale-result protection, selection and privacy.
- Update README when behavior, settings or tradeoffs change.
- Keep grammar, accounts, saved drafts, server personal dictionaries, metrics
  and per-client rate limiting out of this prototype.
- Do not commit, push or change repository access without an explicit request.
