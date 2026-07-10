# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`jaxon` (module `github.com/tucats/jaxon`) is a single-package Go library (Go
1.25.1, no subpackages) that runs a small dot/bracket query language against
arbitrary JSON text or Go values, returning results as strings. There is no
`main` package, no CLI, no external dependencies beyond the standard
library — it's a library other projects import.

## Commands

```sh
go build ./...                      # build (there's nothing to run - it's a library)
go vet ./...                        # vet
gofmt -l .                          # list files that need formatting (empty output = clean)
gofmt -w .                          # apply formatting
go test ./...                       # run the full test suite
go test -v ./...                    # ...with per-test output
go test -run TestGetItems .         # run one top-level test function
go test -run 'TestGetItems/wildcard_on_an_object_collects_every_value_in_key_order' .
                                     # run one specific subtest (table-driven tests use t.Run per case)
go test -race ./...                 # race detector
go test -coverprofile=/tmp/c.out ./... && go tool cover -func=/tmp/c.out
                                     # coverage, function-by-function
go test -run '^$' -bench . -benchmem .
                                     # run only the benchmarks in bench_test.go
```

There's no lint config (no `.golangci.yml`, no Cursor/Copilot rules) — `go vet` and `gofmt` are what's enforced.

## Architecture

The query language works by peeling off one dot-separated segment at a time
and recursing, rather than tokenizing the whole query up front. Reading any
one file in isolation won't explain the flow; the call graph across files is
the important part:

- **`item.go`** — the public entry points: `GetItem`/`GetItems` (take a JSON
  string) and `GetObjectItem`/`GetObjectItems` (take an arbitrary Go value,
  `json.Marshal` it, then delegate to the string-based versions). This is
  the only place `encoding/json.Unmarshal` is called, and that matters
  elsewhere (see "The JSON type boundary" below).
- **`parser.go`** — `parse(object, queryText)` is the recursive core. On each
  call it: strips/validates a leading `.`, splits off just the *next*
  segment plus an unsplit "rest of the query" tail (`splitQuery(query, 2)`),
  decides whether that segment is an index/range, a wildcard (`*`/`*?`), or
  a field name, and either dispatches to `array.go`'s helpers or recurses
  into itself for a map lookup. Because only one segment is split off per
  call, something like `arrayLength` has to be recomputed at every level —
  there's no pre-built list of "all segments" anywhere.
- **`array.go`** — everything about indexing into or iterating over an
  array-like or map-like value: `arrayLength` (needed to resolve open-ended
  ranges), `arrayElement` (single index), `anyArrayElement` +
  `collectMatches` (wildcard — one shared loop used for both arrays and
  `map[string]any`, with a `lenient` flag for `*` vs `*?`), and
  `mapValuesByKey` (sorts object values by key for deterministic wildcard
  output, since JSON objects have no ordering of their own).
- **`range.go`** — `parseSequence`/`parseRange` turn an index/range spec
  string (`"2"`, `"1,3,5"`, `"0-3"`, `"2:"`, `":3"`) into concrete integer
  indices. `-` and `:` are interchangeable range separators.
- **`format.go`** — `format(item)` converts the `any` results `parse()`
  produced back into `[]string`, the type every public function returns.
  Maps become re-marshaled JSON text; arrays recurse per element; `nil`
  becomes the string `"null"`, not Go's `"<nil>"`.
- **`errors.go`** — the `*Error` type (`Code`/`Ctx` fields) and the
  `Err(code).Context(value)` builder chain used everywhere a query fails.
  Implements `error`, but callers must type-assert to `*Error` to get
  `Extract() (code, context string)` — **not every error returned by
  `GetItem`/`GetItems` is a `*jaxon.Error`**: a malformed JSON string
  returns the raw `encoding/json` error unchanged.

### The JSON type boundary

Every public entry point ultimately routes through `json.Unmarshal(..., &v)`
where `v any`. Go's `encoding/json` only ever produces `map[string]any`,
`[]any`, `string`, `float64`, `bool`, or `nil` from that — **never**
`[]string`, `[]int`, `[]float64`, `[]bool`, or `map[any]any`. `array.go`'s
type switches still handle those concrete typed-slice cases (for a caller
that hands a raw Go value directly to the unexported functions), but they
are unreachable through `GetItem`/`GetItems`/`GetObjectItem`/`GetObjectItems`
— `array_test.go` exercises them directly instead of through the public API.
Keep this in mind before "simplifying" those cases away, and before assuming
a new test through the public API will ever hit them.

### Bracket notation is pure sugar, resolved before dispatch

`splitQuery` converts every `[` and `]` in the *entire remaining query
string* into `.` before anything else happens. By the time `parse()` looks
at a segment, brackets never literally appear — whether a segment is
treated as an index/range vs. a field name is decided purely by its first
character (a digit, or `:` for an open-ended range with the start omitted).
`"[2]"` and `"2"` are identical by the time dispatch happens; `"[foo]"` is
just `"foo"`, a field name.

## Query language, quick reference

Full docs with worked examples are in `README.md`; this is just enough to
navigate the code without cross-referencing it constantly:

- `.` or `""` — the whole value, as-is.
- `name`, `a.b.c` — named field lookup, dot-chained for nesting.
- `name?default` — optional field: use `default` if `name` is absent (note:
  present-but-`null` still counts as present, and returns `null`, not the
  default).
- `2`, `[2]` — array index (0-based).
- `0-2`, `0:2`, `1,3,5` — index list/range (inclusive; `-` and `:` are the
  same separator).
- `2:`, `:2` — open-ended range (missing side defaults to 0 / last index,
  resolved via `arrayLength` at that recursion level).
- `*` — wildcard: every array element or every object value (sorted by
  key), strict (any non-match fails the whole query).
- `*?` — lenient wildcard: same, but a non-matching element/value is
  silently skipped instead of failing.
- `[]` — always an error (`ErrEmptyIndex`), not "no results."

## `ISSUES.md`

This is a living audit/fix-tracking document from prior work sessions —
read it before starting new work here. It records bugs found and fixed
(with the reasoning, not just the diff), performance work with actual
before/after benchmark numbers (measured by comparing against a temporary
`git worktree` checked out at the pre-fix commit — see its "Performance"
section for the exact method), spec/syntax design decisions and why they
were made that way (e.g., why negative-index syntax like `"[-1]"` isn't
supported — `-` is already claimed as a range separator), and a short list
of things deliberately left alone with the reasoning for why.

## Testing conventions in this repo

- Tests call unexported functions directly (`parse`, `format`,
  `arrayLength`, `arrayElement`, `anyArrayElement`, `parseSequence`,
  `parseRange`, `splitQuery`, `startsWithDigit`) in addition to the public
  API — this is a single-package repo, so white-box testing is the norm,
  not the exception. Reach for this instead of only testing through
  `GetItem`/`GetItems` when a lower-level function has behavior the public
  API can't directly exercise (see "The JSON type boundary" above).
- Table-driven tests are standard, and each case carries a comment
  explaining *why* it exists (what regression it guards against, or what
  non-obvious behavior it locks in) — not just a restatement of the input/
  output.
- `format()`'s four `if err != nil` guards (one per array-type case) and
  the matching one in `GetItems` are intentionally left uncovered: nothing
  `format()` can currently produce is capable of failing, so there's no
  real input that reaches them. This is explained in a comment on `format()`
  itself — don't try to force coverage there without a good reason.
