# Spellcheck

## Description

English spell checker with a Go JSON API and a Vue/Vuetify editor. Checks run
300 ms after typing stops. Hover, tap or press Alt+↓ on a misspelled word to
choose a correction. Corrections replace exact, same-case occurrences and
support Undo.

## Index

- [Installation](#installation)
- [Running the Project](#running-the-project)
- [Decisions and Tradeoffs](#decisions-and-tradeoffs)
- [Agent Skills](#agent-skills)
- [Verification](#verification)
- [Structure](#structure)
- [API](#api)
- [Configuration](#configuration)
- [Limits and Data](#limits-and-data)
- [Benchmarks](#benchmarks)
- [Improvements with More Time](#improvements-with-more-time)
- [Attribution](#attribution)

## Installation

Requires Go 1.26+ (toolchain pinned in `go.mod`), Node 22.12+ and Make.
Race tests also require a C compiler. From the checkout root:

```bash
make setup
```

This downloads Go modules and installs locked npm dependencies. Test browsers
are installed separately; see [Verification](#verification).

## Running the Project

```bash
# Start the API: http://localhost:3000
make run

# In a second terminal, start the editor: http://localhost:3001
make dev
```

The editor's development server proxies `/v1` to the API.

### Containers

Requires Podman with `podman-compose`, or Docker with Compose.

```bash
# Build and start: http://localhost:8081
make up

# Inspect logs
make logs

# Stop and remove containers
make down
```

For Docker, pass `COMPOSE="docker compose"`. Set `WEB_PORT` to change the
editor port, for example `WEB_PORT=8083 make up`. Only nginx is published;
the Go API stays on the private container network.

## Decisions and Tradeoffs

| Decision | Reason and tradeoff |
|---|---|
| SymSpell via `eskriett/spell` | Reuses an indexed spelling implementation instead of scanning the dictionary or writing edit distance. Suggestions are word-based, not context-aware. |
| Embedded frequency dictionary | Runs without an external service. Coverage is fixed; names and specialist terms may be flagged. |
| Vue editor with a Go JSON API | Browser code handles typing, marks and corrections; Go owns spelling. This requires separate frontend and backend builds. |
| Debounced, cancellable checks | Reduces requests and prevents stale results. Feedback arrives after a 300 ms typing pause. |
| Stateless prototype | No accounts or saved drafts. Focus stays on the writing-and-correction flow rather than persistence or production operations. |

## Agent Skills

[AGENTS.md](AGENTS.md) defines project rules and routes work to the skills in
[.agents/skills](.agents/skills). Compatible agents discover skills by task;
they can also be requested by name.

- [use-modern-go](.agents/skills/use-modern-go/SKILL.md) — Go features supported
  by `go.mod`, without changing contracts or dependencies.
- [branching-logic-flow](.agents/skills/branching-logic-flow/SKILL.md) — clear
  control flow in spelling checks, request validation and error responses.
- [test-first](.agents/skills/test-first/SKILL.md) — regression tests before
  behavior changes.
- [build-editor](.agents/skills/build-editor/SKILL.md) — editor behavior and
  real-browser verification.
- [review-pr](.agents/skills/review-pr/SKILL.md) — read-only review of
  correctness, errors, contracts, tests and documentation.

### How to Use

Ask the agent to load project rules, then request the task:

```text
Read AGENTS.md and load the skills relevant to this task.
Use review-pr to review internal/httpapi and web/src. Report findings; do not edit.
```

For an existing PR or Git checkout, name the PR or diff range. Without Git
metadata, specify files; the agent must not invent a diff or PR history.

## Verification

```bash
npm --prefix web run browser:install  # Once: install test browsers
make fmt         # Format Go code
make lint        # Check formatting, go vet and staticcheck
make build       # Write ./spellcheck and web/dist/
make check       # Lint, build, Go race tests, frontend unit and browser tests
make test        # make check plus Go and npm vulnerability scans
```

Go tools are pinned in `go.mod`. Browser tests start the real API and editor
on ports 3015 and 3016; those ports must be free. Coverage includes Chromium,
Firefox, WebKit, touch, keyboard navigation, reduced motion, forced colors
in Chromium and simulated composition events. Manual screen-reader and
OS-specific IME testing are not included. CI runs `make test`.

## Structure

| Path | Responsibility |
|---|---|
| `cmd/spellcheck` | Configuration, dependency wiring, startup and shutdown |
| `internal/spell` | Word validation, embedded dictionary and ranked suggestions |
| `internal/httpapi` | Routing, JSON validation and middleware |
| `web/src` | Editor, tokenization and request lifecycle |
| `web/tests` | Browser regression tests against the real API |

`main` constructs `spell.Checker` and passes it to `httpapi.New`. Spelling
does not depend on HTTP. `eskriett/spell` provides the SymSpell index and
optimal string alignment distance; `ardanlabs/conf` provides configuration.

## API

| Method | Endpoint | Response |
|---|---|---|
| POST | `/v1/check` | Misspelled words and suggestions |
| GET | `/v1/liveness` | 200, process and build details |
| GET | `/v1/readiness` | 204, ready to accept traffic |

```bash
curl -sS http://localhost:3000/v1/check \
  -H 'Content-Type: application/json' \
  -d '{"words":["quick","recieve"]}'
```

```json
{"misspelled":[{"word":"recieve","suggestions":["receive","relieve"]}]}
```

Valid words are omitted; an all-valid request returns `{"misspelled":[]}`.
Suggestions use the smallest available edit distance, up to two edits, then
frequency descending with alphabetical ties. At most five are returned.

Handler errors use `{"error":"..."}`. Routing errors (404/405) use
`ServeMux`'s standard plain-text responses.

| Status | Condition |
|---|---|
| 400 | Invalid JSON, unknown fields, invalid word or word count outside 1–500 |
| 413 | Body exceeds 129,564 bytes |
| 404 | Unknown path |
| 405 | Unsupported method; includes `Allow` |
| 500 | Unexpected server error; details are logged, not returned |

## Configuration

Run `go run ./cmd/spellcheck -h` for flags. Environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `SPELL_WEB_API_HOST` | `0.0.0.0:3000` | API listen address |
| `SPELL_WEB_READ_TIMEOUT` | `5s` | HTTP read timeout |
| `SPELL_WEB_WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `SPELL_WEB_IDLE_TIMEOUT` | `120s` | Idle connection timeout |
| `SPELL_WEB_SHUTDOWN_TIMEOUT` | `20s` | Graceful shutdown timeout |
| `SPELL_WEB_CORS_ALLOWED_ORIGINS` | Empty | Allowed origins; empty disables CORS |

The CORS origin list is semicolon-separated. The supplied development and
container setups use a same-origin proxy, so CORS is off by default.

The frontend uses `/v1` by default. `SPELL_API_PROXY` changes the development
proxy target; `VITE_SERVICE_API` changes the API base at frontend build time.
`make build BUILD=<tag>` sets the API build identifier.

## Limits and Data

- Embedded US English dictionary: 82,834 words. No grammar or real-word errors.
- Hyphens split words; apostrophes remain inside words. Numbers, URLs and
  emails are skipped by the editor. The API accepts words of up to 64 runes.
- Uppercase words are checked; suggestions preserve uppercase or initial caps.
- Requests contain 1–500 words and at most 129,564 bytes. The editor sends
  deduplicated batches of 200 words.
- No accounts, personal dictionaries, saved drafts, metrics or per-client
  rate limiting. Writing is sent only to the configured spelling API.
- Cancellation is checked between spelling lookups. An active library lookup
  is not interruptible; socket timeouts are not CPU deadlines. The editor's
  request deadline is 10 seconds.

## Benchmarks

Single-word benchmarks exclude dictionary construction; they are not service
capacity or spelling-accuracy measurements.

```bash
go test -run '^$' -bench '^BenchmarkCheck$' -benchmem -count=10 ./internal/spell
```

## Improvements with More Time

Deferred work, in priority order:

1. Evaluate correction accuracy on a pinned set of common typos, names and
   technical terms before changing dictionary coverage or suggestion ranking.
2. Add a personal dictionary so names and specialist terms can be accepted.
   Start browser-local; do not introduce accounts just for this feature.
3. Test real mobile devices, screen readers and OS input methods. Automated
   Chromium, Firefox, WebKit and touch/composition tests already exist.
4. Add measured per-client rate and concurrency limits before public hosting,
   with tests for shared-IP fairness and overload behavior.
5. Add request-rate, latency and saturation metrics without recording writing
   content or client identifiers.
6. Standardize 404/405 errors as JSON while preserving routing and `Allow`
   headers, if other clients need a uniform error contract.

Grammar checking, other languages, accounts and document storage remain
outside this assignment's scope.

## Attribution

Dictionary: [SymSpell](https://github.com/wolfgarbe/SymSpell) (MIT), derived
from [SCOWL](http://wordlist.aspell.net/) (MIT-like) and
[Google Books Ngram](https://storage.googleapis.com/books/ngrams/books/datasetsv2.html)
(CC BY 3.0). `words.txt` is
[frequency_dictionary_en_82_765.txt at commit c239062](https://github.com/wolfgarbe/SymSpell/blob/c239062ae02961df18ab7da1671d01b4388204e0/SymSpell/frequency_dictionary_en_82_765.txt)
with only the byte-order mark removed. SHA-256:
`5eb7d9ca29ba3aee23d11b8b3d3aa048d7e94538b468eeffd9e07f76de7d5125`.

`eskriett/spell`, Vue and Vuetify are MIT-licensed; `ardanlabs/conf` is
Apache-2.0. The API image includes compiled-module licenses and this
dictionary attribution.
