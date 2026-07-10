# Audit findings

Findings from a code/README audit on 2026-07-10. All were verified by reading the
source and, where noted, by running small probe tests against the built package
(build, vet, and the existing test suite all currently pass clean).

**Update, 2026-07-10:** All seven items in the "Bugs" section below have been
fixed, with regression tests added for each (see the `Status` note under each
item for exactly which test(s) cover it). The README conflicts and performance
items have not been touched yet. A new "Specification / syntax suggestions"
section has also been added at the bottom with ideas for improving the query
language itself, based on things learned while fixing these bugs.

## Bugs

0. **Panic (crash) on query `"[]"` (and any query that collapses to an empty
   segment list).** [parser.go:43](parser.go#L43), root cause in
   [parser.go:154-168](parser.go#L154-L168) (`splitQuery`'s leading-empty-part
   trim)

   `splitQuery("[]", 2)` strips the brackets down to the empty string `""`,
   splits it into `[""]`, and then its "remove empty leading parts" loop
   ([parser.go:166-168](parser.go#L166-L168)) strips that one empty element
   too, returning a **zero-length** slice. Back in `parse()`, the guard at
   [parser.go:29](parser.go#L29) only pads the slice when
   `len(parts) == 1`, not when it's `0`, so `parts` stays empty and the very
   next line, `strings.HasPrefix(parts[0], "[")`
   ([parser.go:43](parser.go#L43)), indexes `parts[0]` on an empty slice and
   panics. Verified:

   ```go
   GetItems(`[1,2,3]`, "[]")
   // panic: runtime error: index out of range [0] with length 0
   //   at github.com/tucats/jaxon.parse (parser.go:43)
   ```

   Since queries are ordinary string input (plausibly from user-facing
   search/filter fields in a caller's application), this is a crash any
   caller can trigger with a single malformed query — no `recover()` in the
   package, so it takes down the calling goroutine. This is more severe than
   a wrong-value bug and should be the first thing fixed (either normalize
   `parts` to `["."]` whenever it's empty, or bounds-check before indexing).
   This also directly falsifies the newly-added doc comment on `GetItems`
   ([item.go:53](item.go#L53)) claiming "if the query returns no items, the
   array is a non-nil empty array" — the actual behavior for the
   zero-match-with-no-error path (reachable via e.g. `parse` returning
   `make([]any, 0, 0)` for an index list that parses to zero indices) is a
   **nil** slice, not a non-nil empty one, since `GetItems`'s `result` is
   declared as `var result []string` and only ever grown via `append` inside
   the loop ([item.go:69-78](item.go#L69-L78)).

   **Status: Fixed.** `parse()` now pads `parts` to at least two elements
   whenever `splitQuery` returns zero *or* one element (previously only the
   one-element case was handled), so `parts[0]` is always safe to read.
   Covered by `Test_parse/empty_brackets_does_not_panic` (parser_test.go) and
   `TestGetItems/empty_brackets_is_a_clean_error,_not_a_panic` (item_test.go).
   The "non-nil empty array" doc-comment mismatch on `GetItems` is still
   open — see README conflict item below.

1. **Error `Context()` always reports the Go type `string`, never the actual
   offending value's type.**
   [parser.go:120](parser.go#L120), [array.go:56](array.go#L56), [array.go:155](array.go#L155)

   All three "wrong type" error sites do `fmt.Sprintf("%T", item)`, but `item`
   is the *query string* parameter (always type `string`), not the JSON `body`
   value that actually caused the error. So every `ErrJSONInvalidContent` and
   `ErrArrayType` error reports context `"string"` regardless of what was
   actually queried, e.g.:

   ```text
   jaxon.json.invalid.content: string
   jaxon.array.type: string
   ```

   This makes the error context useless for diagnosing the real problem. It
   should be `fmt.Sprintf("%T", body)` in all three places.

   **Status: Fixed.** All three sites now use `fmt.Sprintf("%T", body)`.
   Covered by `Test_errorContextReportsActualType` (parser_test.go), which
   checks both the `ErrJSONInvalidContent` site in parser.go and the
   `ErrArrayType` site in array.go's `arrayElement`. (The identical fix in
   `anyArrayElement`'s default case isn't separately unit-tested since it's
   the same one-line change reached via a different, harder-to-construct
   input — a wildcard query against a non-array body.)

2. **Open-ended range end defaults to `start+9`, not "rest of array."**
   [range.go:65-67](range.go#L65-L67)

   When a range's end is omitted (e.g. `"[2:]"`), `parseRange` fills in
   `start+9` as the end index — an arbitrary 10-element window. On any array
   shorter than `start+10` this produces a spurious out-of-range error instead
   of returning the remaining elements. Verified: `GetItems("[1,2,3,4,5]", "[2:]")`
   returns `jaxon.array.index: 5` instead of `[3,4,5]`.

   **Status: Fixed** — see #4 below, which covers bugs #2, #3, and #4
   together since they all had to be fixed as one change.

3. **Open-ended range start defaults to `1`, not `0`.**
   [range.go:56-58](range.go#L56-L58)

   Every other part of this package is explicitly 0-based (README says so
   directly), but an omitted range start (e.g. `"[:2]"`) defaults to `1`, not
   `0`. In practice this default is unreachable anyway, see #4.

   **Status: Fixed** — see #4 below.

4. **Open-ended range with omitted start doesn't work at all** —
   it never reaches the range-defaulting code in #3.
   [parser.go:56-67](parser.go#L56-L67)

   `parse()` decides whether a segment is an array index via
   `startsWithDigit(parts[0])`. A query like `"[:2]"` becomes the bare string
   `":2"` after bracket-stripping, which does not start with a digit, so it's
   treated as an object field *name* instead of a range. Querying an array
   with `"[:2]"` fails with `jaxon.json.invalid.content: string` (see bug #1
   for why the context is unhelpful) rather than returning the first two
   elements. Same problem affects any leading-`-` (negative-looking) index
   token, e.g. `"[-1]"` fails the same way rather than being parsed as a range
   or rejected with a clear error.

   **Status: Fixed** (bugs #2, #3, and #4 together). This turned out to need
   a small design change: `parseRange`/`parseSequence` didn't previously know
   how many elements were in the array they were indexing into, so an
   omitted end had no sensible value to default to other than an arbitrary
   guess. Both functions now take an `arrayLen` parameter. A new
   `arrayLength()` helper in array.go reports how many elements are in the
   array being queried (or an error if the value isn't an array at all), and
   `parse()` calls it right before parsing an index/range so the range
   defaults can be resolved correctly: an omitted start defaults to `0`, and
   an omitted end defaults to `arrayLen-1`, the actual last index.
   `parse()`'s dispatch logic was also changed to recognize a segment
   starting with `:` (not just a digit) as an index/range, which is what
   makes `"[:2]"`-style queries reach the range parser at all. Negative
   index tokens like `"[-1]"` are unchanged by this fix and still fall
   through to the field-name lookup path — see the new "Specification /
   syntax suggestions" section below for why, and a suggested way to
   resolve it properly.
   Covered by `Test_parseSequence` and `Test_parseRange` (range_test.go, new
   file), `Test_parse/open_ended_range_missing_end` and
   `Test_parse/open_ended_range_missing_start` (parser_test.go), and
   `TestGetItems/open_ended_range_to_end_of_array` and
   `TestGetItems/open_ended_range_from_start_of_array` (item_test.go).

5. **JSON `null` formats as the Go string `"<nil>"`.**
   [format.go:9-99](format.go#L9-L99)

   `format()` has no case for a `nil` item (which is what `encoding/json`
   produces for a JSON `null`). It falls through every type switch to the
   final `fmt.Sprintf("%v", item)`, producing the literal string `"<nil>"`.
   Verified: `GetItem(`{"name": null}`, "name")` returns `"<nil>", nil` (no
   error) — a Go-internal representation leaking into query results instead
   of e.g. `"null"` or an empty string.

   **Status: Fixed.** `format()` now checks for `item == nil` first and
   returns `"null"`. Covered by `Test_format/JSON_null_value` (format_test.go)
   and `TestGetItem/null_map_member` (item_test.go).

6. **`anyArrayElement` (wildcard `*` queries) silently swallows per-element
   errors.** [array.go:86-90, 102-107, 116-120, 129-133, 142-146](array.go#L86)

   In every `case` of the type switch, element errors are discarded:
   `if text, err := parse(element, query); err == nil { ... }`. If some array
   elements match the query and others don't (e.g. missing field, wrong
   sub-type), the mismatched elements are silently dropped instead of
   surfacing an error — only an *all-elements-failed* case returns
   `ErrArrayNotFound`. Verified with
   `GetItems([{"name":"John"},{"other":"x"},{"name":"Jane"}], "*.name")` →
   returns `["John","Jane"]` with no error, silently omitting the middle
   element. This directly contradicts the README's error-handling
   documentation (see README item #3 below).

   **Status: Fixed.** Every `case` in `anyArrayElement`'s type switch now
   returns immediately with the error from a failed element instead of
   discarding it, matching the README's stated "missing field is an error"
   behavior. Note this is a stricter, more strict-by-default semantic than
   before: a wildcard query now requires *every* element to match, not just
   at least one. See the "Specification / syntax suggestions" section below
   for a suggestion on giving callers an explicit way to opt into the old,
   best-effort behavior instead of only offering the strict one. Covered by
   `Test_parse/wildcard_reports_error_for_mismatched_element_instead_of_skipping_it`
   (parser_test.go) and `TestGetItems/wildcard_with_a_mismatched_element_is_an_error`
   plus `TestGetItems/wildcard_where_every_element_matches_still_works`
   (item_test.go, to confirm the common all-elements-match case still works).

## README / code conflicts

**Update, 2026-07-10:** All four items below are resolved. Per project
guidance, the code is treated as the source of truth here — these were
fixed by correcting the README to accurately describe existing (already
verified) code behavior, not by changing the code. (Item #3 needed no
README change at all: it was made true by the bug #6 code fix in the
previous round.)

1. **The array-query example calls the wrong function.**
   [README.md:34-37](README.md#L34-L37)

   > "To get an array of results, use:
   >
   > ```go
   > value, err := jaxon.GetItem(jsonText, "items[0:3]")
   > ```"

   `GetItem` (singular) always returns a single string and actively *errors*
   (`jaxon.ambiguous`) if the query matches more than one value — verified.
   The array-returning function is `GetItems` (plural). The example as
   written does not do what the surrounding paragraph describes; it should
   call `jaxon.GetItems`.

   **Status: Fixed.** The example now calls `jaxon.GetItems`.

2. **`Extract()` usage example doesn't match what callers actually get back.**
   [README.md:175-187](README.md#L175-L187)

   The README shows `code, context := e.Extract()` where `e` is "an error
   returned from `jaxon`," implying this works directly on the `error` value
   returned by `GetItem`/`GetItems`. Two problems:
   - `Extract()` is defined on `*Error` ([errors.go:50](errors.go#L50)), not on
     the `error` interface, so the example as written won't compile without
     an explicit type assertion (`e.(*jaxon.Error)`) that isn't shown.
   - Not all errors returned by `GetItems` are `*jaxon.Error` in the first
     place — a malformed JSON input returns the raw `encoding/json` error
     unchanged ([item.go:32-34](item.go#L32-L34)), so the documented
     `Extract()` pattern would panic/fail a type assertion for that case.
   - Relatedly, the README calls the type "a custom type `Err`" — `Err` is
     actually the constructor function; the type itself is `Error`
     ([errors.go:5-28](errors.go#L5-L28)).

   **Status: Fixed.** The README now shows the full type-assertion pattern
   (`jaxonErr, ok := err.(*jaxon.Error)`) before calling `Extract()`, calls
   the type `Error` rather than `Err`, and explicitly notes that a malformed
   JSON input returns the raw `encoding/json` error unchanged, so the type
   assertion can fail (`ok == false`) for that case.

3. **"It is an error to specify a field name in an object that does not
   exist"** [README.md:166-167](README.md#L166-L167) is not true for wildcard
   (`*`) queries — see bug #6 above. Elements missing the requested field are
   silently skipped rather than raising an error.

   **Status: Fixed** as a side effect of the bug #6 code fix in the previous
   round (wildcard queries now propagate a missing-field error instead of
   swallowing it) — verified this statement is now literally true of the
   code, so no README wording change was needed here.

4. Minor: typo "teh" → "the" at [README.md:43](README.md#L43).

   **Status: Fixed.**

## Performance

**Update, 2026-07-10:** Both items below are fixed. Benchmarks were added
(`bench_test.go`) and run against both the old and new code to confirm real,
measured improvement rather than just a theoretical one — see the numbers
under each item.

1. **Object field lookup is O(n) reflection-based scan instead of O(1) map
   index.** [parser.go:91-111](parser.go#L91-L111)

   ```go
   val := reflect.ValueOf(body)
   ...
   for _, e := range val.MapKeys() {
       if e.String() == name {
   ```

   Every named-field lookup uses `reflect.ValueOf(body)` and then linearly
   scans `val.MapKeys()` comparing each key's string form, for every level of
   every query. Since `encoding/json` unmarshals JSON objects into
   `map[string]any` exclusively (never any other map type), this can be a
   plain type assertion (`body.(map[string]any)`) followed by a direct map
   index (`m[name]`) — no `reflect` needed at all. As written, this is both
   slower per call (reflection overhead) and algorithmically worse (O(n) key
   scan vs O(1) hash lookup) for objects with many fields, and the cost is
   paid again at every nesting level for every query.

   **Status: Fixed.** `parse()` now does `body.(map[string]any)` followed by
   `m[name]` with the "comma ok" pattern, no `reflect` import left in
   parser.go at all. A regression test
   (`Test_parse/optional_field_that_exists_but_is_null_uses_the_null,_not_the_default`,
   parser_test.go) locks in a subtlety of this change: a field that exists
   but is JSON `null` must still count as "found" via `v, found := m[name]`,
   not be treated as missing.

   Measured with `BenchmarkGetItem_WideObject` (`bench_test.go`, 500-field
   object, looks up the last field) on the same machine, before vs. after:

   | metric | before | after |
   | --- | --- | --- |
   | time/op | 73,897 ns | 64,782 ns (-12%) |
   | bytes/op | 119,076 B | 98,759 B (-17%) |
   | allocs/op | 2,018 | 1,516 (-25%) |

   At 5,000 fields the per-call allocation count drops from 20,049 to
   15,047 — a difference of almost exactly 5,000, matching the removal of
   the old code's `val.MapKeys()` call, which had to allocate one
   `reflect.Value` per key just to search through them one at a time. (Total
   time doesn't drop as sharply at this size because `encoding/json`
   unmarshaling the object dominates the benchmark, and that cost is
   identical in both versions — the fix is specifically to the lookup path,
   not JSON decoding.)

2. Minor: `arrayElement`/`anyArrayElement` rebuild the remaining query by
   `strings.Join`-ing parts back into a string (e.g.
   [array.go:16](array.go#L16), [array.go:81](array.go#L81)) only for `parse()`
   to immediately re-split it via `splitQuery` on the next recursive call.
   For deeply nested queries this repeats string join/split work at every
   level rather than passing the already-split `parts` slice through
   directly.

   **Status: Fixed, but not exactly as originally diagnosed.** Investigating
   this while implementing it showed the `strings.Join` calls above aren't
   actually the waste this item describes: at both call sites, the slice
   being joined always has exactly one element (`parts[1:]` of a
   always-exactly-2-element `parts`), and Go's `strings.Join` returns a
   single-element slice's only element directly with no allocation — so
   there was nothing to save there. The real repeated work turned out to be
   inside `splitQuery` itself: its six `strings.ReplaceAll`/`TrimPrefix`/
   `TrimSuffix` calls for converting `[`/`]` bracket notation into dots run
   again at *every* level of recursion, even though brackets are only ever
   present in the original, unsplit query — by the second level down there
   are never any left to convert, so those six calls were doing nothing but
   scanning the string in vain. `splitQuery` now guards all of that behind
   a single `strings.ContainsAny(s, "[]")` check and skips straight past it
   when there's nothing to convert, which is true for every level past the
   first.

   Measured with `BenchmarkGetItem_DeepNesting` (`bench_test.go`, 20 levels
   of nested objects), before vs. after:

   | metric | before | after |
   | --- | --- | --- |
   | time/op | 7,518 ns | 4,041 ns (-46%) |
   | bytes/op | 9,454 B | 8,333 B (-12%) |
   | allocs/op | 155 | 95 (-39%) |

## Specification / syntax suggestions

Ideas for improving the query language itself, prompted by things noticed
while fixing the bugs above. These are design suggestions, not bugs — nothing
here needs to change for correctness.

1. **The `-` character is overloaded and blocks real negative-index support.**

   `parseSequence` normalizes `-` to `:` unconditionally
   ([range.go](range.go)) so that `"0-3"` and `"0:3"` mean the same range.
   That's a reasonable, README-documented shorthand, but it means `-` can
   never be used as a *sign* — there's no way to write "the last element" as
   `-1` the way many other query/slicing languages do (Python slices,
   `jq`, etc.), because `"-1"` is indistinguishable from a range shorthand
   with a blank end (it normalizes to `":1"`, i.e., "from the start through
   index 1" — not "the last element"). If negative indices are ever wanted,
   the cleanest fix is probably to *not* reuse `-` for this: keep `-` as the
   range separator (already documented, already used in tests) and
   introduce negative numbers as a distinct concept resolved the same way
   this fix resolved open-ended ranges — via the new `arrayLength` value,
   e.g. index `-1` → `arrayLen-1`. That requires detecting a *leading minus
   before any other digits* as a sign rather than a separator, which is a
   small but real grammar change, not just a parsing tweak.

2. **`*` only works on arrays; consider extending it to objects.**

   A wildcard segment currently only matches something on `[]any`,
   `[]string`, `[]float64`, `[]int`, or `[]bool` ([array.go](array.go),
   `anyArrayElement`'s default case rejects anything else, including
   `map[string]any`). A natural extension would be letting `*` on an object
   mean "every value in the object," the same way it means "every element
   of the array" today — e.g. `"config.*.enabled"` to pull an `enabled`
   field out of every entry in an object keyed by name. This isn't
   possible to bolt on purely as a bug fix since it changes what `*` means
   for a whole new body type, so it's called out here as a feature idea
   rather than folded into bug #6's fix.

   **Status: Implemented, 2026-07-10.** `anyArrayElement` (array.go) now
   handles a `map[string]any` body: it collects the map's values, sorted by
   key (via the new `mapValuesByKey` helper) so the result order is
   deterministic — a JSON object has no ordering of its own, and neither
   does a Go map. The per-element loop itself was pulled out into a new
   shared `collectMatches` helper so this didn't mean copy-pasting the loop
   a sixth time. Documented in a new "Wildcards" section in README.md.
   Covered by `Test_parse/wildcard_on_an_object_collects_every_value_in_key_order`,
   `Test_parse/wildcard_on_an_object_with_a_nested_field,_in_key_order`, and
   `Test_parse/wildcard_on_an_object_errors_on_a_value_that_does_not_match`
   (parser_test.go), plus `TestGetItems/wildcard_on_an_object_collects_every_value_in_key_order`
   and `TestGetItems/wildcard_on_an_object_errors_on_a_value_that_does_not_match`
   (item_test.go).

3. **Now that wildcard (`*`) queries are strict, an explicit "lenient
   wildcard" syntax would restore the old best-effort behavior for callers
   who want it.**

   Fixing bug #6 made `*` strict: every element must match the sub-query, or
   the whole call fails. That matches the README's documented error
   semantics, but it removes the ability to say "give me whatever matches,
   and skip anything that doesn't" for a genuinely heterogeneous array — a
   real use case, just not the one the README describes. The query language
   already has a precedent for "this might not be there, and that's OK": the
   optional-field syntax `field?default` (see README "Named Field" section
   and [parser.go](parser.go)'s `hasAlternate` handling). A parallel syntax
   for wildcards — something like `*?` instead of bare `*` — could mean
   "collect matches, silently skip elements that don't match," giving
   callers an explicit choice between strict and lenient instead of only
   ever offering one or the other.

   **Status: Implemented, 2026-07-10.** A wildcard segment that is exactly
   `*?` (as opposed to bare `*`) is now recognized in `parse()`
   (parser.go) and passed down to `anyArrayElement` as a `lenient bool`
   flag. `collectMatches` (array.go, the shared per-element loop added for
   item #2 above) skips an element that fails to match when `lenient` is
   true instead of returning its error immediately. This works for both
   arrays and objects, since both go through the same helper. An empty
   result is still an error either way (`ErrArrayNotFound`) — "collect
   what you can" only makes sense if something actually matched. Documented
   alongside the object-wildcard feature in README.md's new "Wildcards"
   section. Covered by `Test_parse/lenient_wildcard_on_an_object_skips_values_that_do_not_match`,
   `Test_parse/lenient_wildcard_on_an_array_skips_elements_that_do_not_match`,
   and `Test_parse/lenient_wildcard_still_errors_when_nothing_at_all_matches`
   (parser_test.go), plus `TestGetItems/lenient_wildcard_on_an_object_skips_values_that_do_not_match`
   and `TestGetItems/lenient_wildcard_on_an_array_skips_elements_that_do_not_match`
   (item_test.go).

4. **Open-ended ranges and the optional-field syntax aren't documented in
   the README at all.**

   This isn't a conflict (nothing in the README contradicts them), but both
   `"[2:]"`/`"[:3]"`-style open-ended ranges (now fixed, see bug #4) and the
   `field?default` optional-field syntax are real, working parts of the
   query language with zero mention in README.md. Worth adding a short
   section for each alongside the existing range documentation, now that
   open-ended ranges actually behave sensibly.

   **Status: Half already true, other half now fixed, 2026-07-10.** On a
   closer look, the `field?default` optional-field syntax was actually
   *already* documented — it's covered in the "Named Field" section (the
   ".foo?1" example) and was there before this audit started, so that part
   of this item was simply a mistake in the original write-up. The
   open-ended-range half was genuinely missing, and has now been added: the
   "Arrays" section has a new paragraph right after the existing
   range/comma/hyphen documentation explaining that either side of a range
   can be left out (`"2:"` means "index 2 through the last element,"
   `":2"` means "the first element through index 2"), with a worked example
   against the same three-element JSON array already used earlier in that
   section, verified to produce the numbers actually shown (`GetItems`
   against `[1, 15, 66]`).

5. **An empty index specification (`"[]"`) has no defined meaning.**

   The fix for bug #0 makes `"[]"` fail with a clear error instead of
   crashing, but *which* error it produces is really an accident of how an
   empty segment happens to fall through to the "look up an empty field
   name" path (see the fix note under bug #0). The language doesn't
   actually define what an empty bracket pair should mean. Two reasonable
   choices worth picking between deliberately: treat it as a syntax error
   with its own dedicated error code (e.g. `ErrEmptyIndex`, clearer to a
   caller than "field not found: "), or define it to mean "no indices,"
   i.e. always return zero results without an error. Right now it's
   whichever one the implementation happens to fall into, not a considered
   design choice.

   **Status: Implemented, 2026-07-10.** Went with the dedicated error code
   option. Added `ErrEmptyIndex` (errors.go) and changed `parse()`
   (parser.go) to return it directly as soon as `splitQuery` comes back
   empty, instead of padding the parts list to `[""]` and letting that fall
   through to whatever the generic field-lookup path happened to produce
   (`ErrJSONElementNotFound` on an object, `ErrJSONInvalidContent` on an
   array — an accident of an empty string not matching anything, not a
   deliberate choice). The error's context is now the actual offending
   query text (e.g. `"[]"`) rather than an empty string. Documented in
   README.md's "Arrays" section, right after the open-ended-range
   paragraph. Covered by
   `Test_errorContextReportsActualType/empty_brackets_on_an_array` and
   `Test_errorContextReportsActualType/empty_brackets_on_an_object`
   (parser_test.go), which check the exact error code and context, not just
   that *some* error occurred.

## Test coverage audit, 2026-07-10

Ran `go test -coverprofile` and went through every function's uncovered
lines by hand (not just chasing a percentage number) to find real gaps.
Coverage went from 75.7% to 98.2% of statements. New/expanded test files:
`errors_test.go` (new), `array_test.go` (new), plus additions to
`parser_test.go`, `range_test.go`, `format_test.go`, and `item_test.go`.
143 test cases now pass in total. Notable gaps that were closed:

- **`Error()` had 0% coverage** — every other test only ever checked
  `Extract()`'s two return values, never the actual `error`-interface
  string a caller would see from `%v`/`%s` or `err.Error()`. Added direct
  tests for the nil-receiver case (`"<nil>"`), code-with-no-context, and
  code-with-context formatting. Also added the nil-receiver case for
  `Extract()`, which was similarly untested.
- **The `[]string`/`[]float64`/`[]int`/`[]bool` cases in `arrayLength`,
  `arrayElement`, and `anyArrayElement` were almost entirely untested** —
  `encoding/json.Unmarshal` into an `any` only ever produces `[]any` for a
  JSON array, never these concrete typed slices, so nothing reachable
  through `GetItem`/`GetItems`/`GetObjectItem`/`GetObjectItems` could ever
  exercise them. They're not dead code, though (a caller within the package,
  or some future public entry point, could reach them directly) — added
  `array_test.go` calling these unexported functions directly, the same way
  `range_test.go` already does for `parseSequence`/`parseRange`.
- **The `field?default` optional-field syntax's actual "field is missing,
  default gets used" path had zero tests** — despite being the primary
  worked example in the README's "Named Field" section. The only existing
  test for that syntax covered the *other* branch (field present but
  `null`). Added the missing case.
- **Several error-propagation paths were only unit-tested at the leaf
  function, never confirmed to actually propagate up through `parse()`** —
  a malformed index (`"[2x]"`), an out-of-range index, and a leading `".."`
  query. Added cases exercising these through `parse()` directly rather
  than only through `parseSequence`/`parseRange` in isolation.
- **`GetItem`'s own two error branches — `ErrNotFound` and `ErrAmbiguous`
  — were never checked for the specific code**, only for "some error
  occurred." This also surfaced that a whole-value query (`"."`) against a
  top-level empty JSON array (`[]`) is a real, reachable way to get zero
  results with no error (via `format()` formatting an empty array's zero
  elements), which is what makes `ErrNotFound` reachable at all. Added
  `Test_GetItem_errorCodes` plus the matching `TestGetItems` case.
- **`GetItems` passing a malformed-JSON error straight through
  unchanged — as `*json.SyntaxError`, not `*jaxon.Error` — was never
  explicitly checked**, even though the README calls this out by name.
  Added `Test_GetItems_malformedJSON`, which asserts the returned error is
  *not* a `*jaxon.Error`.

**Remaining gaps, left alone on purpose:** `format()`'s four
`if err != nil { return nil, err }` guards (one per array-type case) and
the matching one in `GetItems` are structurally unreachable given
`format()`'s current implementation — every base case it can produce
(`nil`, a map, a scalar) always succeeds, so the recursive call inside each
array loop can never actually fail. Forcing a test through them would mean
contriving an artificial failure with no basis in real behavior, so a
comment was added at `format()` instead, explaining why coverage tooling
will always flag these five lines and that they're deliberately kept anyway
(as a safety net for if a future case is added that *can* fail).

## Not flagged

- `go build`, `go vet`, and the full existing test suite (`go test ./...`)
  all pass cleanly — no regressions from the current code.
- `format.go`'s `map[any]any` case was dead code — `encoding/json` never
  produces this type — and has now been removed, 2026-07-10. It was
  harmless, so this was cleanup rather than a bug fix; no test changes were
  needed since nothing exercised that branch to begin with.
- The dead bracket-stripping branch in `parse()` (previously at
  [parser.go:43-54](parser.go#L43-L54) before the bug #0/#2/#3/#4 fixes) has
  been removed as part of that fix. It was confirmed 100% unreachable:
  `splitQuery` already converts every `[`/`]` into `.` before `parse()` ever
  looks at `parts[0]`, for every query shape tried, so this was dead code
  with no behavior attached to it — not a separate bug, just cleanup that
  fell out of touching this section anyway.
