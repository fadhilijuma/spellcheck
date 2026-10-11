---
name: build-editor
description: Build or change the Vue editor in web/ (Vue 3, Vuetify, Vite, plain JavaScript) against the /v1/check API. Use for editor behavior, helper modules and browser tests.
---

# Build Editor

Keep editor code in `web/`. Diagnose API behavior at the HTTP boundary;
use `use-modern-go` for Go changes authorized by the same task rather than
duplicating spelling rules or response workarounds in the frontend.

## API

```text
POST /v1/check   {"words": ["Teh", "quick"], "language": "en-US"}
200              {"misspelled": [{"word": "Teh", "suggestions": ["The", "Ten"]}]}
anything else    show "Check unavailable"
```

Words not in `misspelled` are valid only after a successful, validated response.
At most 500 words per request, each at most 64 Unicode code points.
Language is `en-US` or `en-GB` (traditional British spelling). Omitted or empty
uses the server's configured default. Read `GET /v1/config` for the initial editor
selection; an explicitly saved user choice takes precedence. Do not save the
server default as a user preference.
Switching language rechecks the draft and cancels obsolete responses. Save only
the language preference and accepted personal words in browser storage, never the draft.

## Files

| File | Does |
|---|---|
| `src/tokenize.js` | finds words with their offsets; applies a correction |
| `src/api.js` | `checkDocument`: dedupes words, skips long ones, sends batches of 200 with the selected language |
| `src/session.js` | 300 ms debounce, cancels superseded checks, 10 s deadline |
| `src/hover.js` | which word is under the pointer; where the popup goes |
| `src/dictionary.js` | validates and normalizes browser-local personal words |
| `src/App.vue`, `src/components/WritingEditor.vue` | the page and the editor |

## Rules

- Words are Unicode letters with inner apostrophes. Split on hyphens. Skip
  numbers, URLs and emails. Check uppercase words too; case alone does not
  establish that a word is an acronym. Keep this in step with
  `spell.ParseWord` in Go.
- A failed check keeps the text, clears old marks and never blocks typing.
- Test pure helper behavior in adjacent `*.test.js` files and interactions in
  `web/tests/`. No router, store or extra UI library.
- Personal words apply to both English variants, matching whole words regardless
  of case or apostrophe style. Add them from the correction popup and allow removal.
  Invalid saved data must not break editing; unavailable storage keeps changes
  in memory for the current session. Do not add accounts or draft persistence.

Done means `npm --prefix web test`, `npm --prefix web run test:browser` and
`npm --prefix web run build` pass. `make check` runs all three with Go checks.
Browser regressions in `web/tests/` start the real Go API and Vue app on
isolated ports; successful checks must not be replaced by mocked responses.
Run the shared flows in Chromium, Firefox, WebKit and touch-enabled Chromium.
Cover keyboard focus, accessible instructions and composition commit handling;
forced-color emulation belongs to the Chromium projects.
