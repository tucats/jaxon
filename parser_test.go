package jaxon

import (
	"encoding/json"
	"reflect"
	"testing"
)

func Test_parse(t *testing.T) {
	type object1 struct {
		Field1 string
	}

	tests := []struct {
		name    string
		item    string
		query   string
		want    []any
		wantErr bool
	}{
		{
			name: "all array index with bracket notation",
			item: `
			{
			   "items": [
			 		{"name": "John", "age": 30},
			   		{"name": "Jane", "age": 25}
				]
			}`,
			query: ".items[*].name[1]",
			want:  []any{"Jane"},
		},
		{
			name: "all array index",
			item: `
			{
			   "items": [
			 		{"name": "John", "age": 30},
			   		{"name": "Jane", "age": 25}
				]
			}`,
			query: ".items.*.name.1",
			want:  []any{"Jane"},
		},
		{
			name:  "single integer",
			item:  `22`,
			query: ".",
			want:  []any{22.0},
		},
		{
			name:  "single float",
			item:  `3.14`,
			query: ".",
			want:  []any{3.14},
		},
		{
			name:  "single float representation of integer value",
			item:  `42.0`,
			query: ".",
			want:  []any{42.0},
		},
		{
			name:  "object field name",
			item:  `{ "name": "John Doe", "age": 30 }`,
			query: "name",
			want:  []any{"John Doe"},
		},
		{
			name:  "object field age",
			item:  `{ "name": "John Doe", "age": 30 }`,
			query: "age",
			want:  []any{30.0},
		},
		{
			name:  "nested object field",
			item:  `{ "person": { "name": "John Doe", "age": 30 } }`,
			query: "person.name",
			want:  []any{"John Doe"},
		},
		{
			name:  "array index",
			item:  `[1, 2, 3, 4, 5]`,
			query: "2",
			want:  []any{3.0},
		},
		{
			name: "array of elements",
			item: `
			[
				{"Field1": "one"},
				{"Field1": "two"},
                {"Field1": "three"}
			]`,
			query: "*.Field1",
			want:  []any{"one", "two", "three"},
		},
		{
			// Regression test for the switch from a reflect-based
			// MapKeys() scan to a plain map index with the "comma ok"
			// pattern (v, found := m[name]). A field that exists but
			// whose value is JSON null must still count as "found" - the
			// query should return the null, not silently fall through to
			// the "field?default" alternate value as if the field were
			// missing entirely.
			name:  "optional field that exists but is null uses the null, not the default",
			item:  `{ "name": null }`,
			query: "name?fallback",
			want:  []any{nil},
		},
		{
			// Regression test: an empty index specification (empty
			// brackets) used to crash the program with an "index out of
			// range" panic instead of returning an error. This should
			// now fail cleanly.
			name:    "empty brackets does not panic",
			item:    `[1, 2, 3]`,
			query:   "[]",
			wantErr: true,
		},
		{
			// Regression test: an open-ended range with no end (meaning
			// "to the last element") used to default to an arbitrary
			// 10-element window and fail with an out-of-range error on
			// an array shorter than that.
			name:  "open ended range missing end",
			item:  `[1, 2, 3, 4, 5]`,
			query: "[2:]",
			want:  []any{3.0, 4.0, 5.0},
		},
		{
			// Regression test: an open-ended range with no start (meaning
			// "from the first element") used to be misread as an object
			// field name instead of an index range, since it starts with
			// ":" rather than a digit.
			name:  "open ended range missing start",
			item:  `[10, 20, 30, 40, 50]`,
			query: "[:2]",
			want:  []any{10.0, 20.0, 30.0},
		},
		{
			// Regression test: a wildcard ("*") query used to silently
			// drop array elements that didn't match the sub-query
			// instead of reporting an error, hiding data that doesn't
			// match the expected shape.
			name: "wildcard reports error for mismatched element instead of skipping it",
			item: `
			[
				{"name": "John"},
				{"other": "x"},
				{"name": "Jane"}
			]`,
			query:   "*.name",
			wantErr: true,
		},
		{
			// A wildcard on an object collects every value in the object,
			// the same way it collects every element of an array. Since a
			// JSON object has no defined ordering of its own, the values
			// are returned sorted by key ("a", "b", "c") so the result is
			// deterministic.
			name:  "wildcard on an object collects every value in key order",
			item:  `{ "b": 2, "a": 1, "c": 3 }`,
			query: "*",
			want:  []any{1.0, 2.0, 3.0},
		},
		{
			name:  "wildcard on an object with a nested field, in key order",
			item:  `{ "config": { "b": {"enabled": false}, "a": {"enabled": true}, "c": {"enabled": true} } }`,
			query: "config.*.enabled",
			want:  []any{true, false, true},
		},
		{
			// A bare "*" on an object is just as strict as on an array:
			// every value must match the rest of the query.
			name:    "wildcard on an object errors on a value that does not match",
			item:    `{ "a": {"enabled": true}, "b": {"other": false} }`,
			query:   "*.enabled",
			wantErr: true,
		},
		{
			// The lenient wildcard "*?" is the opt-in way to get back the
			// old, pre-fix best-effort behavior: elements/values that
			// don't match the rest of the query are silently skipped
			// instead of failing the whole query.
			name:  "lenient wildcard on an object skips values that do not match",
			item:  `{ "a": {"enabled": true}, "b": {"other": false} }`,
			query: "*?.enabled",
			want:  []any{true},
		},
		{
			name: "lenient wildcard on an array skips elements that do not match",
			item: `
			[
				{"name": "John"},
				{"other": "x"},
				{"name": "Jane"}
			]`,
			query: "*?.name",
			want:  []any{"John", "Jane"},
		},
		{
			// The lenient wildcard still reports an error if literally
			// nothing matches - "collect what you can" only makes sense
			// when there's something to collect.
			name: "lenient wildcard still errors when nothing at all matches",
			item: `
			[
				{"other": "x"},
				{"other2": "y"}
			]`,
			query:   "*?.name",
			wantErr: true,
		},
		{
			// A query segment starting with ".." (two dots in a row,
			// meaning there's nothing between them) is always invalid -
			// there's no field name or index there at all.
			name:    "leading double dot is a query error",
			item:    `{ "a": 1 }`,
			query:   "..a",
			wantErr: true,
		},
		{
			// A malformed index specification (a bracketed segment that
			// starts with a digit, so it's treated as an index/range, but
			// isn't actually a valid integer) must have its error
			// propagate all the way back out of parse(), not just be
			// caught by the lower-level parseSequence unit tests in
			// range_test.go.
			name:    "malformed index propagates its error out of parse()",
			item:    `[1, 2, 3]`,
			query:   "[2x]",
			wantErr: true,
		},
		{
			// An index past the end of the array must have its
			// ErrArrayIndex error propagate all the way back out of
			// parse(), the same way the malformed-index case above does.
			name:    "out of range index propagates its error out of parse()",
			item:    `[1, 2, 3]`,
			query:   "10",
			wantErr: true,
		},
		{
			// This is the ".foo?1" example from the README's "Named
			// Field" section: if "foo" doesn't exist, the value after the
			// "?" is used instead. Despite being the primary documented
			// example of this feature, no test anywhere previously
			// exercised the case where the field is actually missing and
			// the default gets used - only the case where the field
			// exists (see "optional field that exists but is null uses
			// the null, not the default" above) was covered.
			name:  "missing optional field uses the default value",
			item:  `{ "other": 1 }`,
			query: "foo?bar",
			want:  []any{"bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b any

			err := json.Unmarshal([]byte(tt.item), &b)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
			}

			got, err := parse(b, tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_errorContextReportsActualType is a regression test for a bug where
// "wrong type" errors always reported their context as the literal Go
// type name "string", no matter what was actually queried. This happened
// because the error-construction code accidentally used the query string
// itself (which is, of course, always type string) instead of the actual
// JSON value that caused the problem. These tests check that the context
// now names the real offending type.
func Test_errorContextReportsActualType(t *testing.T) {
	tests := []struct {
		name        string
		item        string
		query       string
		wantCode    string
		wantContext string
	}{
		{
			// Looking up a field name only makes sense on an object. Doing
			// so on an array should report the array's real type.
			name:        "field name lookup on an array",
			item:        `[1, 2, 3]`,
			query:       "name",
			wantCode:    ErrJSONInvalidContent,
			wantContext: "[]interface {}",
		},
		{
			// Indexing into a value only makes sense on an array. Doing so
			// on a plain number should report the number's real type.
			name:        "index lookup on a scalar",
			item:        `{ "age": 42 }`,
			query:       "age.0",
			wantCode:    ErrArrayType,
			wantContext: "float64",
		},
		{
			// Regression test: an empty index specification ("[]") used
			// to fall through to the generic "field not found" handling
			// by accident (an empty string just happens not to match any
			// field name), rather than being reported as the invalid
			// syntax it actually is. It now gets its own dedicated error
			// code, with the offending query text as context.
			name:        "empty brackets on an array",
			item:        `[1, 2, 3]`,
			query:       "[]",
			wantCode:    ErrEmptyIndex,
			wantContext: "[]",
		},
		{
			// Same as above, but on an object instead of an array - the
			// empty index specification is invalid regardless of what
			// it's being applied to.
			name:        "empty brackets on an object",
			item:        `{ "a": 1 }`,
			query:       "[]",
			wantCode:    ErrEmptyIndex,
			wantContext: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b any

			if err := json.Unmarshal([]byte(tt.item), &b); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			_, err := parse(b, tt.query)
			if err == nil {
				t.Fatal("parse() succeeded unexpectedly")
			}

			jaxonErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("parse() returned error of type %T, want *Error", err)
			}

			code, context := jaxonErr.Extract()
			if code != tt.wantCode {
				t.Errorf("error code = %v, want %v", code, tt.wantCode)
			}

			if context != tt.wantContext {
				t.Errorf("error context = %v, want %v", context, tt.wantContext)
			}
		})
	}
}

// Test_startsWithDigit exercises this small helper directly, including its
// empty-string case. parse() only ever calls startsWithDigit(parts[0])
// after already confirming parts[0] can't be empty (splitQuery guarantees
// its first element, if any, is never an empty string - see splitQuery's
// "remove empty leading parts" step), so that branch can't actually be
// reached through parse() as the code is written today. It's tested
// directly here anyway, both to document the function's contract for its
// own sake and as a safety net in case some future change to parse()'s
// dispatch logic stops guaranteeing a non-empty parts[0].
func Test_startsWithDigit(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "empty string", s: "", want: false},
		{name: "single digit", s: "7", want: true},
		{name: "multi-digit number", s: "123", want: true},
		{name: "digit followed by other characters", s: "0-3", want: true},
		{name: "letter", s: "a", want: false},
		{name: "colon", s: ":3", want: false},
		{name: "leading space before a digit", s: " 3", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startsWithDigit(tt.s); got != tt.want {
				t.Errorf("startsWithDigit(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
