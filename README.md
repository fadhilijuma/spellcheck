# Spellcheck

## Description

English spell checker with a Go JSON API and a Vue/Vuetify editor. Checks run
300 ms after typing stops. Hover, tap or press Alt+↓ on a misspelled word to
choose a correction. Corrections replace exact, same-case occurrences and
support Undo.
Choose English (US) or English (UK) in the header. The default is configurable;
the browser remembers your choice and rechecks the current draft when it changes.

## Index

- [Installation](#installation)
- [Running the Project](#running-the-project)
- [Decisions and Tradeoffs](#decisions-and-tradeoffs)
- [Agent Skills](#agent-skills)
- [Prompts](#prompts)
- [Verification](#verification)
- [Structure](#structure)
- [API](#api)
- [Configuration](#configuration)
- [Limits and Data](#limits-and-data)
- [Benchmarks](#benchmarks)
- [Improvements with More Time](#improvements-with-more-time)
- [Attribution](#attribution)

## Installation

Running requires Make and Podman with `podman-compose`, or Docker with Compose.
Go and Node dependencies are installed inside the container build stages.
From the checkout root:

```bash
make setup
```

This builds both container images. For Docker, append `COMPOSE="docker compose"`
to Make commands. Host checks have separate prerequisites; see
[Verification](#verification).

## Running the Project

```bash
# Start both with Compose: http://localhost:8081
make run

# Stop and remove the stack
make down
```

nginx serves the editor and proxies `/v1` to `app:3000` on the Compose network.
Only the editor port is published; no CORS configuration or separate backend
port is needed.

For background operation:

```bash
# Build and start: http://localhost:8081
make up

# Inspect logs
make logs

# Stop and remove containers
make down
```

Set `WEB_PORT` to change the editor port, for example `WEB_PORT=8083 make run`.
For Docker: `make run COMPOSE="docker compose"`.

### Request Replays

```bash
# Print JSON responses through the running Compose stack.
make request-us request-uk
make request-invalid  # Expected HTTP 400; Make exits nonzero.

# Replay against the running IntelliJ debugger; pauses can last up to 10 minutes.
make request-us API_URL=http://127.0.0.1:3005
```

Use `WEB_PORT=8083` for a Compose stack on that port, or override `API_URL`
for another endpoint. `REQUEST_TIMEOUT` is in seconds and defaults to `600`
to accommodate breakpoints.

## Decisions and Tradeoffs

| Decision | Reason and tradeoff |
|---|---|
| SymSpell via `eskriett/spell` | Reuses an indexed spelling implementation instead of scanning the dictionary or writing edit distance. Suggestions are word-based, not context-aware. |
| Embedded English spelling lists | ESDB/SCOWL size 60 for US and traditional British spelling, ranked with word frequencies. Each is indexed once at startup; both indexes remain in memory. |
| Vue editor with a Go JSON API | Browser code handles typing, marks and corrections; Go owns spelling. This requires separate frontend and backend builds. |
| Debounced, cancellable checks | Reduces requests and prevents stale results. Feedback arrives after a 300 ms typing pause. |
| Stateless prototype | No accounts or saved drafts. Focus stays on the writing-and-correction flow rather than persistence or production operations. |

The library owns dictionary membership, edit distance, candidate deduplication
and closest-match selection. Our adapter reads the embedded word lists into
library `Entry` values, applies case/apostrophe handling and acronym rules, and
adds alphabetical tie-breaking and the suggestion limit. It uses `GetWords` to
extract suggestions. The library's optional `Save`/`Load` format stores words
and settings, not a prebuilt deletion index: `Load` calls `AddEntry` again.
Embedding the text lists avoids an extra generated dictionary file and disk I/O.

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

## Prompts

Reusable task prompts live in [prompts/](prompts). Read `AGENTS.md` first,
then load the architecture guide and the prompt for the task.

- [codereview.txt](prompts/codereview.txt) — read-only review of spelling,
  HTTP contracts, editor behavior, tests and error handling.
- [example_project.txt](prompts/example_project.txt) — build or extend the API
  and editor with the established entry-point and package design.
- [newdomain.txt](prompts/newdomain.txt) — implement a spelling rule using
  concrete domain models and the existing validation boundaries.
- [project-structure.md](prompts/project-structure.md) — package responsibilities,
  index lifecycle, request flow, editor interactions and verification.

```text
Read AGENTS.md and prompts/project-structure.md.
Use prompts/codereview.txt to review the current spelling API and editor. Do not edit.
```

The four filenames match the [reference prompt set](https://github.com/ardanlabs/service/tree/007e502c948bb6ced690ab4f0744da6aeb626a05/zarf/prompts).
Their contents are rewritten for Spellcheck; database and deployment scaffolding
from that reference do not apply here.

## Verification

Host checks require Go 1.27.2+ (minimum version in `go.mod`), Node 22.12+ and
a C compiler for race tests. Install their dependencies before running checks:

```bash
go mod download
npm --prefix web ci
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
| `internal/spell` | String lookups, language selection and ranked suggestions |
| `internal/spell/languages` | Embedded US/British dictionaries and a shared acronym allowlist |
| `internal/httpapi` | Routing, JSON validation and middleware |
| `web/src` | Editor, tokenization and request lifecycle |
| `web/tests` | Browser regression tests against the real API |

`main` constructs one `spell.Checker` per language and passes them to `httpapi.New`. Spelling
does not depend on HTTP. `eskriett/spell` provides the SymSpell index and
optimal string alignment distance; `ardanlabs/conf` provides configuration.
`go-playground/validator` checks index settings when constructing each checker
and positive word limits when constructing the HTTP handler, using struct tags.

## API

| Method | Endpoint | Response |
|---|---|---|
| POST | `/v1/check` | Misspelled words and suggestions |
| GET | `/v1/config` | Default English convention, word length, word count and body-byte limits |
| GET | `/v1/liveness` | 200, process and build details |
| GET | `/v1/readiness` | 204, ready to accept traffic |

```bash
make request-us
```

```json
{"misspelled":[{"word":"recieve","suggestions":["receive","relieve"]},{"word":"speling","suggestions":["spelling","spewing","spieling"]}]}
```

Valid words are omitted; an all-valid request returns `{"misspelled":[]}`.
Empty, omitted or null word lists and blank entries also return no spelling
warnings. Blank entries in a mixed batch are ignored without changing other words.
Suggestions use the smallest available edit distance within the configured
maximum, then frequency descending with alphabetical ties. Defaults are two
edits and at most five suggestions per word.

`language` accepts `en-US` or `en-GB`. Omitted or empty uses the configured default.
British English uses traditional `-ise` spelling: `colour`, `centre`,
`organise` and `analyse`. Unsupported language values return 400.

Handler errors use `{"error":"..."}`. Routing errors (404/405) use
`ServeMux`'s standard plain-text responses.

| Status | Condition |
|---|---|
| 400 | Invalid JSON/types, unsupported language, exceeded input length or more than `MaxWords` entries |
| 413 | JSON request body exceeds the internal 1 MiB limit |
| 404 | Unknown path |
| 405 | Unsupported method; includes `Allow` |
| 500 | Unexpected server error; details are logged, not returned |

Requests contain one complete JSON object. Unknown fields are ignored;
malformed JSON and trailing data are rejected.

## Configuration

Run `go run ./cmd/spellcheck -h` for flags. Environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `SPELL_DEFAULT_LANGUAGE` | `en-US` | Default English convention: `en-US` or `en-GB` |
| `SPELL_MAX_WORD_LEN` | `64` | Maximum input length in Unicode code points; must be positive |
| `SPELL_MAX_WORDS` | `500` | Maximum words per request; must be positive |
| `SPELL_MAX_SUGGESTIONS` | `5` | Maximum suggestions per word; must be positive |
| `SPELL_MAX_DISTANCE` | `2` | Maximum edits; `0` allows exact matches only |
| `SPELL_PREFIX_LENGTH` | `7` | Indexed runes per word; must exceed maximum distance |
| `SPELL_WEB_API_HOST` | `0.0.0.0:3000` | API listen address |
| `SPELL_WEB_READ_TIMEOUT` | `5s` | HTTP read timeout |
| `SPELL_WEB_WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `SPELL_WEB_IDLE_TIMEOUT` | `120s` | Idle connection timeout |
| `SPELL_WEB_SHUTDOWN_TIMEOUT` | `20s` | Graceful shutdown timeout |
| `SPELL_WEB_CORS_ALLOWED_ORIGINS` | Empty | Allowed origins; empty disables CORS |

The CORS origin list is semicolon-separated. The supplied development and
container setups use a same-origin proxy, so CORS is off by default.

Set British English with `SPELL_DEFAULT_LANGUAGE=en-GB make run` or
`go run ./cmd/spellcheck --default-language=en-GB`. Unsupported values fail startup.
The container setup also accepts `SPELL_DEFAULT_LANGUAGE`.
The editor reads `/v1/config` for its initial selection and request limits,
including when a language preference is saved. An explicitly saved user choice
takes precedence over the default language. Both dictionaries remain available
for switching.

Spelling settings apply to both checkers at startup, including in containers.
For example, `go run ./cmd/spellcheck --max-suggestions=3 --max-distance=1 --prefix-length=5`.
Set the input length with `SPELL_MAX_WORD_LEN=80 make run` or
`go run ./cmd/spellcheck --max-word-len=80`; Compose accepts the same variable.
Reducing this limit does not truncate the embedded dictionaries or delete
previously accepted personal words.
Set the batch word limit with `go run ./cmd/spellcheck --max-words=100`
or `SPELL_MAX_WORDS`; Compose accepts the same variable. The HTTP package
enforces a fixed 1 MiB limit on the complete JSON body. It is not a flag or
environment setting. The editor reads this limit from `/v1/config` and sizes
requests automatically; users do not enter byte limits.
Invalid limits fail startup. A larger edit distance increases index-building
work and memory usage; longer indexed prefixes also increase index size.
Prefix length limits candidate indexing, not the whole-word distance check.

The API constructor validates request limits once. HTTP enforces input length
and batch limits, then passes strings directly to the checker. The checker does
not impose letters-only or apostrophe-placement rules. Blank entries are ignored;
other strings, including digits and punctuation, are passed to the library.
A lookup without suggestions is still an unknown input, not proof of correct spelling.
The editor validates configuration when fetched, not on every typing check.
Each English checker loads the acronym list during construction.
Dictionary membership and suggestions use a single lookup.

The frontend uses `/v1` by default. `SPELL_API_PROXY` changes the development
proxy target; `VITE_SERVICE_API` changes the API base at frontend build time.
`make build BUILD=<tag>` sets the API build identifier.

## Limits and Data

- Embedded dictionaries: 107,901 US words and 107,555 British words.
  No grammar or real-word errors.
- Hyphens split words; apostrophes remain inside words. Numbers, URLs and
  emails are skipped by the editor. The API accepts words up to the configured
  `SPELL_MAX_WORD_LEN` (64 Unicode code points by default).
- `languages/acronyms.txt` contains one known uppercase acronym per line,
  shared by both dictionaries. Acronyms are accepted as written and are not
  added to correction candidates. Unlisted uppercase words are still checked;
  suggestions preserve uppercase or initial caps.
- Hover over an underlined word (or press Alt+ArrowDown with the caret in it)
  and choose **Add to dictionary** to accept it. Use **Dictionary → Remove**
  to undo an accepted word. Personal words apply to both English variants;
  matching ignores case and treats straight/curly apostrophes alike, but does
  not accept substrings or other inflections.
- The language preference and personal words are saved in this browser's
  `localStorage`; drafts are not saved. Clearing browser storage removes them.
  If storage is unavailable, personal words last only for the current session.
  Personal words filter spelling warnings locally; they do not change the
  embedded dictionaries or the API's suggestions. Draft words are still sent
  to the configured spelling API.
- Requests contain up to `MaxWords` entries (500 by default), within the internal
  1 MiB body limit. The editor
  deduplicates words and sizes batches against both limits using actual UTF-8
  JSON bytes. Words above `MaxWordLen` are reported as unchecked. A word that
  cannot fit the body budget reports **Check unavailable**, never **All clear**.
- No accounts, server-side personal dictionaries, saved drafts, metrics or per-client
  rate limiting. Writing is sent only to the configured spelling API.
- Cancellation is checked between spelling lookups. An active library lookup
  is not interruptible; socket timeouts are not CPU deadlines. The editor's
  request deadline is 10 seconds.

## Benchmarks

Single-word benchmarks exclude dictionary construction. Parser benchmarks cover
the embedded US/British lists and acronyms without building the index. Neither
measures service capacity or spelling accuracy.

```bash
go test -run '^$' -bench '^BenchmarkCheck$' -benchmem -count=10 ./internal/spell
go test -run '^$' -bench '^BenchmarkParseWordList$' -benchmem -count=10 ./internal/spell
```

## Improvements with More Time

Deferred work, in priority order:

1. Evaluate correction accuracy on a pinned set of common typos, names and
   technical terms before changing dictionary coverage or suggestion ranking.
2. Test real mobile devices, screen readers and OS input methods. Automated
   Chromium, Firefox, WebKit and touch/composition tests already exist.
3. Add measured per-client rate and concurrency limits before public hosting,
   with tests for shared-IP fairness and overload behavior.
4. Add request-rate, latency and saturation metrics without recording writing
   content or client identifiers.
5. Standardize 404/405 errors as JSON while preserving routing and `Allow`
   headers, if other clients need a uniform error contract.

Grammar checking, other languages, accounts and document storage remain
outside this assignment's scope.

## Attribution

The spelling vocabularies are the US and British size-60 lists from
[English Speller Database (ESDB)](https://github.com/en-wl/wordlist), release
[`rel-2026.02.25`](https://github.com/en-wl/wordlist/releases/tag/rel-2026.02.25),
generated from commit `7e99edab8e32f9f9ea2b15f249ca8d4d67237410`. Source lists:
[en_US.txt](https://github.com/en-wl/wordlist-diff/blob/71d7dd07676edb60ade43552e10b41314b7e9287/en_US.txt)
and [en_GB-ise.txt](https://github.com/en-wl/wordlist-diff/blob/71d7dd07676edb60ade43552e10b41314b7e9287/en_GB-ise.txt).
The embedded files in `internal/spell/languages` normalize case and apostrophes,
remove duplicate forms and retain only tokens accepted by this application.
Existing SymSpell/Google Books frequencies are retained for
overlapping words. New words receive the median existing frequency of their
SCOWL size tier (35, 50 or 60); these are ranking estimates, not measured
word-specific frequencies:

| List | Size 35 | Size 50 | Size 60 |
|---|---:|---:|---:|
| US | 758,778 | 185,608 | 84,142 |
| British | 742,515 | 184,413 | 83,361 |

The tiers can be reproduced from the pinned ESDB source with
`./scowl word-list SIZE A 1 --deaccent` for US and
`./scowl word-list SIZE B 1 --deaccent` for British spelling, where SIZE is
35, 50 or 60. Size-50 and size-60 medians exclude words in the lower tiers.
The source frequency data is
[SymSpell's frequency dictionary](https://github.com/wolfgarbe/SymSpell/blob/c239062ae02961df18ab7da1671d01b4388204e0/SymSpell/frequency_dictionary_en_82_765.txt),
derived from [Google Books Ngram](https://storage.googleapis.com/books/ngrams/books/datasetsv2.html)
(CC BY 3.0). Embedded file SHA-256 checksums:

```text
ec7dad1564bc1fcc78799cd2a151f25c5919ceb41b010694bf539c729452cc7d  en-US.txt
9cafa27596dfefb9153ecac5bf2af6debca6137e895b601902da2c86c14c15f9  en-GB.txt
```

The shared acronym allowlist is maintained by this project.

Copyright 2000-2026 by Kevin Atkinson

Permission to use, copy, modify, distribute, and sell any part of SCOWLv2, or
word lists created from it, is hereby granted without fee, provided that the
above copyright notice appears in all copies and that both the above copyright
notice and this notice appear in supporting documentation. Kevin Atkinson
makes no representations about the suitability of this database for any
purpose. It is provided "as is" without express or implied warranty.

`eskriett/spell`, Vue and Vuetify are MIT-licensed; `ardanlabs/conf` is
Apache-2.0. The API image includes compiled-module licenses and this
dictionary attribution.
