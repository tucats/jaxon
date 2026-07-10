package jaxon

import (
	"reflect"
	"testing"
)

// Test_arrayLength exercises arrayLength directly against every type it
// understands, plus an unsupported type.
//
// This matters because, in ordinary use through GetItem/GetItems,
// encoding/json.Unmarshal into an `any` only ever produces a JSON array as
// a Go []any - never a []string, []float64, []int, or []bool directly.
// Those four cases in arrayLength's (and arrayElement's and
// anyArrayElement's) type switch exist for callers who build a query
// against a raw Go value of one of those concrete types directly (for
// example, calling these unexported helpers from other code in this
// package, or a future public entry point that skips the JSON round trip).
// Without tests like this one, those four cases could be broken by a
// careless edit and nothing would notice, since nothing in the public,
// JSON-based API can ever reach them.
func Test_arrayLength(t *testing.T) {
	tests := []struct {
		name    string
		body    any
		want    int
		wantErr bool
	}{
		{name: "[]any", body: []any{1, 2, 3}, want: 3},
		{name: "[]string", body: []string{"a", "b"}, want: 2},
		{name: "[]float64", body: []float64{1.1, 2.2, 3.3, 4.4}, want: 4},
		{name: "[]int", body: []int{1, 2, 3, 4, 5}, want: 5},
		{name: "[]bool", body: []bool{true, false}, want: 2},
		{
			// arrayLength is called before resolving an index or range
			// against a value that isn't an array at all - for example,
			// querying "[0]" against a plain string. It must report an
			// error, not panic or silently return 0.
			name:    "unsupported type reports an error",
			body:    "not an array",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := arrayLength(tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("arrayLength() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got != tt.want {
				t.Errorf("arrayLength() = %d, want %d", got, tt.want)
			}
		})
	}
}

// Test_arrayElement exercises arrayElement directly: an in-range and an
// out-of-range index for every array type it understands, plus an
// unsupported type. See the comment on Test_arrayLength above for why the
// []string/[]float64/[]int/[]bool cases need their own direct tests rather
// than relying on GetItem/GetItems to reach them.
func Test_arrayElement(t *testing.T) {
	tests := []struct {
		name    string
		body    any
		index   int
		want    []any
		wantErr bool
	}{
		{name: "[]any in range", body: []any{"a", "b", "c"}, index: 1, want: []any{"b"}},
		{name: "[]any negative index is out of range", body: []any{"a", "b", "c"}, index: -1, wantErr: true},
		{name: "[]any index past the end is out of range", body: []any{"a", "b", "c"}, index: 5, wantErr: true},

		{name: "[]string in range", body: []string{"x", "y", "z"}, index: 2, want: []any{"z"}},
		{name: "[]string out of range", body: []string{"x", "y", "z"}, index: 3, wantErr: true},

		{name: "[]float64 in range", body: []float64{1.5, 2.5}, index: 0, want: []any{1.5}},
		{name: "[]float64 out of range", body: []float64{1.5, 2.5}, index: 2, wantErr: true},

		{name: "[]int in range", body: []int{10, 20, 30}, index: 2, want: []any{30}},
		{name: "[]int out of range", body: []int{10, 20, 30}, index: 3, wantErr: true},

		{name: "[]bool in range", body: []bool{true, false, true}, index: 1, want: []any{false}},
		{name: "[]bool out of range", body: []bool{true, false, true}, index: 3, wantErr: true},

		{
			// Indexing into a value that isn't an array at all - e.g.
			// querying "[0]" against a number - must report an error
			// naming the value's real type, not panic.
			name:    "unsupported type reports an error",
			body:    42,
			index:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A nil parts slice and the literal "test" as the item string
			// stand in for "there's no further query after this index,
			// and this string is only used for error-message context."
			got, err := arrayElement(tt.body, tt.index, nil, "test")
			if (err != nil) != tt.wantErr {
				t.Fatalf("arrayElement() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("arrayElement() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_anyArrayElement exercises anyArrayElement directly: a wildcard
// collecting every value from each array type it understands, from an
// object (sorted by key), and the error cases (an unsupported type, and an
// empty array). See the comment on Test_arrayLength above for why the
// []string/[]float64/[]int/[]bool cases need their own direct tests.
func Test_anyArrayElement(t *testing.T) {
	tests := []struct {
		name    string
		body    any
		want    []any
		wantErr bool
	}{
		{name: "[]any", body: []any{"a", "b"}, want: []any{"a", "b"}},
		{name: "[]string", body: []string{"a", "b", "c"}, want: []any{"a", "b", "c"}},
		{name: "[]float64", body: []float64{1.1, 2.2}, want: []any{1.1, 2.2}},
		{name: "[]int", body: []int{1, 2, 3}, want: []any{1, 2, 3}},
		{name: "[]bool", body: []bool{true, false}, want: []any{true, false}},
		{
			// A wildcard on an object collects its values sorted by key
			// ("a", "b", "c"), not in whatever random order Go happens to
			// iterate the map in.
			name: "map[string]any, sorted by key",
			body: map[string]any{"b": 2.0, "a": 1.0, "c": 3.0},
			want: []any{1.0, 2.0, 3.0},
		},
		{
			// A wildcard against something that isn't an array or an
			// object at all - e.g. a plain number - must report an
			// error, not panic.
			name:    "unsupported type reports an error",
			body:    42,
			wantErr: true,
		},
		{
			// An empty array has no elements to collect, so this is an
			// error (ErrArrayNotFound) rather than a silent, empty
			// success - "*" is supposed to find something.
			name:    "empty array reports an error rather than an empty success",
			body:    []any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A nil parts slice and lenient=false stand in for a bare
			// "*" with nothing after it: every element/value is
			// collected as-is, and (per lenient=false) any element that
			// failed to match would abort the whole call - though none
			// of these test cases have anything that could fail to
			// match, since there's no sub-query being applied.
			got, err := anyArrayElement(tt.body, nil, "test", false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("anyArrayElement() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("anyArrayElement() = %v, want %v", got, tt.want)
			}
		})
	}
}
