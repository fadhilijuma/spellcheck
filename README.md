# Spellcheck

English spelling engine using an embedded frequency dictionary and SymSpell.
The HTTP API and browser editor are added in subsequent commits.

## Development

Requires Go 1.26+; the toolchain is pinned in `go.mod`.

```bash
go test ./internal/spell
```

`internal/spell` owns word validation, dictionary loading, case preservation
and deterministic suggestions. Tests use small injected dictionaries.

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
