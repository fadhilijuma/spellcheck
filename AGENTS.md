# AGENTS.md

Rules for any coding agent in this repo. Humans follow them too.

## The project

A web editor that flags misspelled English words and suggests fixes: a Go API
(`cmd/spellcheck`, `internal/`) and a Vue frontend (`web/`). It is a
small application: keep it focused. README documents the behavior.

| Path | Holds |
|---|---|
| `cmd/spellcheck` | `main`: config, wiring, shutdown. No logic |
| `internal/spell` | spelling: `Checker`, `Word`, the embedded word list. No HTTP |
| `internal/httpapi` | routes, handlers, request/response, middleware. No spelling rules |
| `web` | the Vue editor and its nginx container |

`main` builds the `spell.Checker` and passes it to `httpapi.New`. `spell`
imports nothing of ours.

## Skills

Read the whole skill before working.

| Skill | When |
|---|---|
| `use-modern-go` | any `.go` work: features supported by `go.mod` |
| `branching-logic-flow` | shallow Go conditionals and defaulting |
| `review-pr` | read-only review of correctness, errors, contracts, tests and documentation |
| `test-first` | a new package or behaviour: failing tests first |
| `build-editor` | any change in `web/` |

These skills form one set. The project rules below take precedence over
repo-specific assumptions in a skill. Only installed skills apply; references
to removed or excluded skills do not authorize restoring them.
Skills do not authorize new packages,
frameworks, dependencies, account storage or a layout migration. Read only
guidance relevant to the requested scope; review in-process unless delegation
is available and authorized.

## Rules

- **Least code that works.** No interfaces with one implementation, no layers,
  no helpers used once, nothing "for later".
- **Dependencies:** the standard library, `eskriett/spell`, `ardanlabs/conf`,
  and `go-cmp` in tests. The frontend uses Vue/Vuetify and Playwright for
  development-only browser tests. Ask before adding anything.
- **Tests are the spec, and every test is table-driven.** Never weaken,
  skip or delete one to get green.
- **Out of scope:** grammar, other languages, accounts, document storage,
  personal dictionaries, metrics and per-client rate limiting. These are
  future improvements, not prototype features. Never collect drafts.
- **Never edit `words.txt`** by hand.
- **Never run `git commit`, `push`, `reset`, `rebase`** or anything that
  changes history; print the commands for the user.

## Working with the user

- For changes across packages or to the API, list the files and tests first
  and wait for "go". Small edits go straight ahead.
- When the spec is silent, ask one question with your recommended answer.
- Reply briefly: what changed, what the checks said.

## Commands

| Command | Does |
|---|---|
| `make run` / `make dev` | API on :3000 / Vue on :3001 |
| `make check` | formatting, vet, staticcheck, Go/frontend builds, race tests and frontend unit/browser tests |
| `make test` | `make check` plus vulnerability scans |

Done means `make check` passes and README is still true.
