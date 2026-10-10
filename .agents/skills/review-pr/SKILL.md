---
name: review-pr
description: "Read-only review of Spellcheck's Go API, Vue/JavaScript editor, tests and build configuration for correctness, error visibility, documentation, contracts and simplification. Use for PR review, unstaged-diff review, explicit file review, or a targeted review of one of those aspects."
---

# Service Diffguard

Service Diffguard reviews a diff or explicit file scope with six focused
**lenses**, returning **findings** graded on one shared scale. Run the lenses
in-process unless the user explicitly authorizes delegation and the runtime
supports it. Lenses advise only — they never edit,
stage, or commit code, never disable tests, and never bypass a failing check to
make it pass.

Default scope is the **unstaged `git diff`** unless the caller names specific
files, a branch, a commit range, or an existing PR. Without Git metadata, review
explicitly requested files; if none were named, ask for scope. Do not initialize
a repository, invent a diff, or describe a whole-file review as a PR review.

## Gate scale

Every finding carries a Gate. The run's overall Gate is the highest any lens
returns.

| Gate   | Name    | Meaning                                                                                  | Action                                               |
|--------|---------|------------------------------------------------------------------------------------------|------------------------------------------------------|
| **G4** | Stop    | Likely correctness, security, data-integrity, compile, boundary, or reliability failure. | Block merge until fixed.                             |
| **G3** | Repair  | Likely user-visible bug, operational blind spot, real test gap, or misleading contract.  | Fix before the PR is ready.                          |
| **G2** | Tighten | Locally safe but leaves avoidable risk, ambiguity, weak design, or brittleness.          | Address if reasonably scoped; otherwise acknowledge. |
| **G1** | Polish  | Low-risk clarity, naming, comment, or small test refinement.                             | Optional; batch, don't spam.                         |
| **G0** | Clear   | No issue, or a positive observation.                                                     | None.                                                |

Every **G2 or higher** finding must include: `file:line`, **Proof** (evidence from
the reviewed diff or files), why it matters, a **Patch direction** (smallest safe fix), and how to
verify. Weak or speculative observations go under **Open checks**, not as a
finding. Cap **G1** items at five per lens unless exhaustive polish is requested.

## Lens catalog

| Lens                   | Focus                                  | Reference prompt                  |
|------------------------|----------------------------------------|-----------------------------------|
| **Service Steward**    | general code correctness & conventions | `reference/service-steward.md`    |
| **Error Tripwire**     | failure-path visibility                | `reference/error-tripwire.md`     |
| **Doc Drift Check**    | comment truthfulness                   | `reference/doc-drift-check.md`    |
| **Harness Map**        | behavior-to-test mapping               | `reference/harness-map.md`        |
| **Boundary Keeper**    | type, package & HTTP/editor contracts  | `reference/boundary-keeper.md`    |
| **Straight-Line Pass** | behavior-preserving simplification     | `reference/straight-line-pass.md` |

## Languages in this repo

Diffguard reviews a changed file in whatever language it is written in. Each lens
applies its concept (correctness, failure visibility, comment truth, test
coverage, contract design, simplification) through the idioms of that language —
Go is the primary codebase, not the only one.

| Area | Where | Lens focus |
|---|---|---|
| Startup | `cmd/spellcheck` | Configuration, concrete dependency wiring, startup failures and shutdown. No spelling or HTTP business logic. |
| Spelling | `internal/spell` | Word validation, dictionary parsing, SymSpell lookup, deterministic ranking, case preservation and cancellation. No HTTP, JSON or logging. |
| HTTP API | `internal/httpapi` | Routing, request limits, JSON decoding, error/status responses, middleware and context propagation. No duplicated spelling rules. |
| Editor | `web/src` (`.vue`, `.js`, `.css`) | Vue contracts, debounce/cancellation, stale results, Unicode offsets, inline corrections, caret/Undo, accessibility and cleanup. |
| Tests | Adjacent Go/JS tests and `web/tests` | Table-driven behavior coverage and real API/browser interaction regressions. |
| Build/config | `Makefile`, `Dockerfile`, `compose.yaml`, `web/Dockerfile`, `web/nginx.conf`, `web/vite.config.mjs`, `web/playwright.config.js`, `.github/workflows/check.yml` | Actual paths, commands, proxy behavior, pinned dependencies, port ownership, exit codes and secret handling. |

## Repo-local authority

All lenses treat **AGENTS.md** (and any nested AGENTS.md) as authoritative.

Apply these repo skills **only to `.go` files**, where relevant:

- `use-modern-go` — Go implementation style.
- `branching-logic-flow` — defaulting and shallow control flow.

For editor behavior, use `build-editor`; for regression-test requirements, use
`test-first`. **Every test must be table-driven**, as required by AGENTS.md.
Review is read-only: never apply `go fix`, `make fmt`, dependency updates or
other source-rewriting commands during review.

Do not import another repository's layers, converters, decorators or tooling.
Grammar, other languages, accounts, storage, personal dictionaries, metrics
and per-client rate limiting are deferred, not release defects. Never collect
or log draft text, and never edit `words.txt` by hand.

## Diff intake

1. Resolve scope from the caller.
2. If Git metadata is available, inspect the requested diff (unstaged by default):
   - `git status --short`
   - `git diff --name-only`
   - `git diff`
   - For a branch/PR: `git diff --name-only <base>...HEAD`, or `gh pr diff`.
   Without Git metadata, read the explicit file scope and report that limitation.
3. Read README's behavior and API contracts. Build a short change inventory:
   which languages/areas changed (see *Languages in this repo*); production vs
   test files; startup/spelling/HTTP/editor files; comments
   or doc strings; error/failure handling; new or modified types, interfaces,
   request/response shapes, component contracts, or build configuration.

## Lens routing

Route by what changed unless the caller asks for "full", "all", or "PR-ready".
For explicit file reviews without a diff, route by the code and contracts in scope.

- **Always** (any production code changed, any language): Service Steward.
- **Production code or tests changed, or coverage asked:** Harness Map.
- **Diff touches any failure path** — Go `err`/`errors.Is/As`/wrapping/`defer`/
  `recover`/context, JS `try/catch`/promises/`await`/`fetch`, or script exit codes;
  plus logging and fallback/default
  behavior: Error Tripwire.
- **Any contract or shape changed** — Go structs/interfaces/constructors,
  HTTP request/response shapes, word validation, suggestion results, Unicode
  offsets or Vue `props`/`emits`: Boundary Keeper.
- **Comments, doc strings (Go doc, JSDoc), examples, README snippets, or
  TODO/FIXME changed:** Doc Drift Check.
- **Polish pass:** Straight-Line Pass — run in a second wave after G4/G3 findings are
  resolved, or in the first wave only when simplification is explicitly requested.

## Execution

Read only the reference prompts selected by the routing rules and apply them
in-process. Missing delegation tools do not block review. If the user explicitly
authorizes available subagents, use the same read-only scope and prompts;
do not delegate by default.

Default is a two-wave plan:

- **Wave 1 (risk):** Service Steward, Harness Map, plus Error Tripwire, Boundary
  Keeper, and Doc Drift Check where applicable.
- **Wave 2 (polish, optional):** Straight-Line Pass, after high-risk findings are
  addressed or when requested.

For an authorized delegated lens, use a prompt of this shape:

```text
Use `.agents/skills/review-pr/reference/<lens>.md` as your review instructions.

Scope: <exact scope>
Diff source: <unstaged git diff | PR diff | branch range | file list>
Languages in scope: <Go; Vue/JavaScript/CSS; build/config>
Repo context: Spellcheck's cmd/spellcheck, internal/spell, internal/httpapi and web.
Follow AGENTS.md; apply Go skills to .go files only. Do not introduce layers or deferred features.
Mode: advisory/read-only — do not modify files.

Return a Gate report only, using the G0-G4 scale.
```

## Aggregation

Merge lens reports into one Diffguard report. Deduplicate overlapping findings by
failure mode and location; on disagreement keep the higher Gate.

```markdown
# Service Diffguard Report

Scope: <scope reviewed>
Overall Gate: G[0-4] — <Clear/Polish/Tighten/Repair/Stop>

## Run matrix
| Lens               | Result        | Notes |
|--------------------|---------------|-------|
| Service Steward    | Gx            | ...   |
| Error Tripwire     | Gx / skipped  | ...   |
| Doc Drift Check    | Gx / skipped  | ...   |
| Harness Map        | Gx / skipped  | ...   |
| Boundary Keeper    | Gx / skipped  | ...   |
| Straight-Line Pass | Gx / deferred | ...   |

## Fix queue
### G4 Stop
- [lens] <title> — `file.go:line`
  - Proof:
  - Patch direction:
  - Verify:
### G3 Repair
### G2 Tighten
### G1 Polish

## Clear signals
- <What looks solid and why.>

## Open checks
- <Anything not reviewed, unavailable, or needing human confirmation.>

## Suggested verification
- Go: `CGO_ENABLED=1 go test -race -count=1 ./internal/spell ./internal/httpapi`.
- Frontend unit tests: `npm --prefix web test`.
- Real API/browser tests: `npm --prefix web run test:browser` (ports 3015/3016 must be free).
- Full project gate: `make check`; vulnerability scans: `make test`.
- Say which checks ran, passed, failed or were blocked. Do not claim manual
  screen-reader, real-device or OS input-method coverage from browser emulation.
- Re-run affected lenses after fixes.
```
