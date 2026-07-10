package jaxon

import "testing"

func TestGetItem(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		item    string
		want    string
		wantErr bool
	}{
		{
			name: "escaped nested array of maps member",
			text: `[ { "user.name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "0.user\\.name",
			want: "Alice",
		},
		{
			name: "escaped nested array of maps member with bracket notation",
			text: `[ { "user.name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "[0]user\\.name",
			want: "Alice",
		},
		{
			name: "escaped nested map of maps member",
			text: `{ "first.one": { "user.name": "Alice", "age": 30 }, "second": { "name": "Bob", "age": 25 } }`,
			item: "first\\.one.user\\.name",
			want: "Alice",
		},
		{
			name: "nested array of maps member",
			text: `[ { "name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "0.name",
			want: "Alice",
		},
		{
			name: "nested array of maps member with bracket notation",
			text: `[ { "name": "Alice", "age": 30 }, { "name": "Bob", "age": 25 } ]`,
			item: "[0].name",
			want: "Alice",
		},
		{
			name: "integer array member",
			text: `[ 1, 2, 3]`,
			item: "2",
			want: "3",
		},
		{
			name: "integer array member with bracket notation",
			text: `[ 1, 2, 3]`,
			item: "[2]",
			want: "3",
		},
		{
			name: "bool array member",
			text: `[ true, false]`,
			item: "0",
			want: "true",
		},
		{
			name: "nested integer map member",
			text: `{ "person": { "age": 43}}`,
			item: "person.age",
			want: "43",
		},
		{
			name: "integer map member",
			text: `{ "age": 42}`,
			item: "age",
			want: "42",
		},
		{
			name: "string map member",
			text: `{ "color": "brown"}`,
			item: "color",
			want: "brown",
		},
		{
			name: "bool map member",
			text: `{ "open": true}`,
			item: "open",
			want: "true",
		},
		{
			name:    "integer map member not found",
			text:    `{ "age": 42}`,
			item:    "ages",
			want:    "",
			wantErr: true,
		},
		{
			name: "simple integer",
			text: `42`,
			item: ".",
			want: "42",
		},
		{
			name: "simple string",
			text: `"brown"`,
			item: ".",
			want: "brown",
		},
		{
			// Regression test: a JSON null value used to be formatted
			// using Go's internal "<nil>" spelling instead of "null".
			name: "null map member",
			text: `{ "name": null }`,
			item: "name",
			want: "null",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetItem(tt.text, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetItem() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("GetItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetItems(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		item    string
		want    []string
		wantErr bool
	}{
		{
			// A query that legitimately succeeds with zero results, and
			// no error. Querying "." (meaning "the whole value, as
			// found") against a JSON value that is itself an empty array
			// returns that empty array as parse()'s one result item.
			// format() then formats *that* array by formatting each of
			// its (zero) elements, producing zero strings - so the
			// overall result is zero strings, with no error: the query
			// wasn't wrong, there was simply nothing inside to report.
			name: "whole-value query on an empty array succeeds with zero results",
			text: `[]`,
			item: ".",
			want: nil,
		},
		{
			// Regression test: an open-ended range with the end left out
			// (meaning "through the last element") used to default to an
			// arbitrary 10-element window and error out on shorter
			// arrays instead of returning the remaining elements.
			name: "open ended range to end of array",
			text: `[1, 2, 3, 4, 5]`,
			item: "[2:]",
			want: []string{"3", "4", "5"},
		},
		{
			// Regression test: an open-ended range with the start left
			// out (meaning "from the first element") used to be
			// misinterpreted as an object field name, since it starts
			// with ":" instead of a digit, and so always failed.
			name: "open ended range from start of array",
			text: `[10, 20, 30, 40, 50]`,
			item: "[:2]",
			want: []string{"10", "20", "30"},
		},
		{
			// Regression test: this query used to panic with an "index
			// out of range" error instead of failing gracefully.
			name:    "empty brackets is a clean error, not a panic",
			text:    `[1, 2, 3]`,
			item:    "[]",
			wantErr: true,
		},
		{
			// Regression test: a wildcard ("*") query used to silently
			// drop elements that didn't match the sub-query. Now it
			// reports an error instead of returning a partial, silently
			// incomplete result.
			name:    "wildcard with a mismatched element is an error",
			text:    `[ {"name": "John"}, {"other": "x"}, {"name": "Jane"} ]`,
			item:    "*.name",
			wantErr: true,
		},
		{
			name: "wildcard where every element matches still works",
			text: `[ {"name": "John"}, {"name": "Jane"} ]`,
			item: "*.name",
			want: []string{"John", "Jane"},
		},
		{
			// A wildcard also works on an object, collecting every value
			// in it (sorted by key, since a JSON object has no ordering
			// of its own) the same way it collects every array element.
			name: "wildcard on an object collects every value in key order",
			text: `{ "config": { "b": {"enabled": false}, "a": {"enabled": true}, "c": {"enabled": true} } }`,
			item: "config.*.enabled",
			want: []string{"true", "false", "true"},
		},
		{
			name:    "wildcard on an object errors on a value that does not match",
			text:    `{ "a": {"enabled": true}, "b": {"other": false} }`,
			item:    "*.enabled",
			wantErr: true,
		},
		{
			// "*?" is the lenient wildcard: it collects whatever matches
			// and quietly skips anything that doesn't, instead of failing
			// the whole query the way a bare "*" does.
			name: "lenient wildcard on an object skips values that do not match",
			text: `{ "a": {"enabled": true}, "b": {"other": false} }`,
			item: "*?.enabled",
			want: []string{"true"},
		},
		{
			name: "lenient wildcard on an array skips elements that do not match",
			text: `[ {"name": "John"}, {"other": "x"}, {"name": "Jane"} ]`,
			item: "*?.name",
			want: []string{"John", "Jane"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetItems(tt.text, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetItems() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("GetItems() = %v, want %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetItems()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetObjectItem(t *testing.T) {
	type person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		object  any
		item    string
		want    string
		wantErr bool
	}{
		{
			name:   "struct field",
			object: person{Name: "John Doe", Age: 30},
			item:   "name",
			want:   "John Doe",
		},
		{
			// Regression test: GetObjectItem used to swallow a JSON
			// marshal failure and return a nil error along with the
			// empty string, so the caller had no way to tell the call
			// had failed at all. A channel value can't be represented in
			// JSON, so json.Marshal is guaranteed to fail here, and
			// GetObjectItem should now report that failure instead of
			// hiding it.
			name:    "value that cannot be marshaled to JSON reports an error",
			object:  make(chan int),
			item:    ".",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetObjectItem(tt.object, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectItem() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantErr {
				return
			}

			if got != tt.want {
				t.Errorf("GetObjectItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetObjectItems(t *testing.T) {
	type person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		object  any
		item    string
		want    []string
		wantErr bool
	}{
		{
			name: "wildcard over a slice of structs",
			object: []person{
				{Name: "John Doe", Age: 30},
				{Name: "Jane Doe", Age: 28},
			},
			item: "*.name",
			want: []string{"John Doe", "Jane Doe"},
		},
		{
			// Same regression as TestGetObjectItem above, but for the
			// array-returning entry point: a marshal failure must be
			// reported, not silently turned into a nil error.
			name:    "value that cannot be marshaled to JSON reports an error",
			object:  make(chan int),
			item:    ".",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetObjectItems(tt.object, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectItems() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantErr {
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("GetObjectItems() = %v, want %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetObjectItems()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Test_GetItem_errorCodes checks GetItem's own two error branches - beyond
// whatever error GetItems might already have returned - by name: querying
// something that produces zero results reports ErrNotFound, and querying
// something that produces more than one result reports ErrAmbiguous. Every
// other GetItem test in this file only checks wantErr (some error or
// none), never which specific error code came back, so neither of these
// branches had a test confirming it produces the *right* error rather than
// just *an* error.
func Test_GetItem_errorCodes(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		item     string
		wantCode string
	}{
		{
			// See the matching "whole-value query on an empty array
			// succeeds with zero results" case in TestGetItems above for
			// why this legitimately produces zero results rather than
			// failing earlier with some other error.
			name:     "zero results is ErrNotFound",
			text:     `[]`,
			item:     ".",
			wantCode: ErrNotFound,
		},
		{
			name:     "more than one result is ErrAmbiguous",
			text:     `[1, 2, 3, 4, 5]`,
			item:     "0:2",
			wantCode: ErrAmbiguous,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetItem(tt.text, tt.item)
			if err == nil {
				t.Fatal("GetItem() succeeded unexpectedly")
			}

			jaxonErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("GetItem() returned error of type %T, want *Error", err)
			}

			if code, _ := jaxonErr.Extract(); code != tt.wantCode {
				t.Errorf("error code = %v, want %v", code, tt.wantCode)
			}
		})
	}
}

// Test_GetItems_malformedJSON checks that malformed JSON text produces the
// raw error from encoding/json, completely unchanged - not a *jaxon.Error.
// This is called out explicitly in the README's "Errors" section: not
// every error GetItem/GetItems can return is a *jaxon.Error, and a
// malformed JSON payload is the one case where it isn't. Without a test
// like this, a future change that wrapped or replaced that error would
// silently break what the README promises about it.
func Test_GetItems_malformedJSON(t *testing.T) {
	_, err := GetItems(`{not valid json`, ".")
	if err == nil {
		t.Fatal("GetItems() succeeded unexpectedly")
	}

	if _, ok := err.(*Error); ok {
		t.Errorf("GetItems() returned a *jaxon.Error for malformed JSON, want the raw encoding/json error unchanged")
	}
}
