package jaxon

import (
	"reflect"
	"testing"
)

func Test_format(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		item    any
		want    []string
		wantErr bool
	}{
		{
			name:    "simple integer value",
			item:    42,
			want:    []string{"42"},
			wantErr: false,
		},
		{
			name:    "simple float value",
			item:    42.5,
			want:    []string{"42.5"},
			wantErr: false,
		},
		{
			name:    "simple float value that could be integer",
			item:    42.0,
			want:    []string{"42"},
			wantErr: false,
		},
		{
			name:    "simple array value",
			item:    []any{1, 2, 3},
			want:    []string{"1", "2", "3"},
			wantErr: false,
		},
		{
			// Regression test: a JSON "null" decodes to a Go nil interface
			// value. format() must report this as the string "null" (JSON's
			// own spelling), not Go's internal "<nil>" representation.
			name:    "JSON null value",
			item:    nil,
			want:    []string{"null"},
			wantErr: false,
		},
		{
			name:    "simple array of ints value",
			item:    []int{1, 2, 3},
			want:    []string{"1", "2", "3"},
			wantErr: false,
		},
		{
			// format() has its own dedicated case for []float64, distinct
			// from the []int case above and the []any case earlier in
			// this table - this exercises it directly, since nothing
			// else in this package's tests passes a raw []float64 to
			// format().
			name:    "simple array of float64 values",
			item:    []float64{1.5, 2.0, 3.25},
			want:    []string{"1.5", "2", "3.25"},
			wantErr: false,
		},
		{
			// Likewise for format()'s dedicated []string case.
			name:    "simple array of string values",
			item:    []string{"a", "b", "c"},
			want:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "simple map",
			item: map[string]any{
				"name": "John Doe",
				"age":  30,
			},
			want: []string{
				`{
   "age": 30,
   "name": "John Doe"
}`},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := format(tt.item)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("format() failed: %v", gotErr)
				}

				return
			}

			if tt.wantErr {
				t.Fatal("format() succeeded unexpectedly")
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("format() = %v, want %v", got, tt.want)
			}
		})
	}
}
