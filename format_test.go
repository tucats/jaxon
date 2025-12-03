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
			name:    "simple array of ints value",
			item:    []int{1, 2, 3},
			want:    []string{"1", "2", "3"},
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
