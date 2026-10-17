---
name: test-first
description: Write failing table-driven tests before implementing a package or behaviour. Use when starting new Go or frontend behaviour, or when asked to write tests first.
---

# Test First

Write the tests and only the stubs needed to compile them. Don't implement.

## Required test structure

Every test MUST be table-driven: one explicit row per behaviour, run by one
loop. Do not replace tables with standalone tests and hand-written asserts.
For Go, use `[]Table` and the package's `run` helper, following
`internal/spell/spell_test.go`. Keep this requirement when updating the skill.

## Steps

1. Read README for the behaviour. If it is unclear, ask; don't invent.
2. Copy the shape of an existing test: `internal/spell/spell_test.go` for a
   function, `internal/httpapi/httpapi_test.go` for the API,
   `web/src/*.test.js` for the frontend.
3. Add stubs that compile and return zero values, marked `// TODO(impl)`.
4. Add one table row per behaviour, in the test file of the code under test.
   For `spell`, use a few dictionary lines in the test, not the full list.
5. Run `go test ./...` (or `npm --prefix web test`). Every new row must fail on its
   assertion, not on compilation. A row that passes against a stub tests
   nothing.
6. Report each test and the line it failed on.

## What to cover

- The normal case, each error the README lists, and the edges (empty, the
  limit, one past it).
- For the API: the status and the exact body.
- Cancellation, where the code takes a context.
