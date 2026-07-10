package jaxon

import (
	"reflect"
	"testing"
)

func Test_parseSequence(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		arrayLen int
		want     []int
		wantErr  bool
	}{
		{
			name:     "single index",
			s:        "2",
			arrayLen: 5,
			want:     []int{2},
		},
		{
			name:     "comma separated list",
			s:        "1,3,4",
			arrayLen: 5,
			want:     []int{1, 3, 4},
		},
		{
			name:     "hyphen range",
			s:        "0-2",
			arrayLen: 5,
			want:     []int{0, 1, 2},
		},
		{
			name:     "colon range",
			s:        "0:2",
			arrayLen: 5,
			want:     []int{0, 1, 2},
		},
		{
			// Regression test for the bug where an omitted range end
			// defaulted to start+9 (an arbitrary 10-element window)
			// instead of "the rest of the array." A 5 element array
			// queried with "2:" should return indices 2, 3, and 4 - not
			// error out trying to reach index 11.
			name:     "open ended range missing end",
			s:        "2:",
			arrayLen: 5,
			want:     []int{2, 3, 4},
		},
		{
			// Regression test for the bug where an omitted range start
			// defaulted to 1 instead of 0, which is inconsistent with
			// the rest of this 0-based package.
			name:     "open ended range missing start",
			s:        ":2",
			arrayLen: 5,
			want:     []int{0, 1, 2},
		},
		{
			name:     "missing start and end defaults to entire array",
			s:        ":",
			arrayLen: 3,
			want:     []int{0, 1, 2},
		},
		{
			name:     "open ended range on empty array is invalid",
			s:        "2:",
			arrayLen: 0,
			wantErr:  true,
		},
		{
			// A single value that isn't a valid integer (it doesn't
			// contain ":", so it's handled as a single index rather than
			// a range) must report ErrInvalidInteger.
			name:     "single value that is not an integer",
			s:        "abc",
			arrayLen: 5,
			wantErr:  true,
		},
		{
			// Comma-separated lists can have empty entries - for example
			// a stray trailing comma, or two commas in a row - and those
			// are silently skipped rather than treated as errors.
			name:     "empty entries in a comma separated list are skipped",
			s:        "1,,3,",
			arrayLen: 5,
			want:     []int{1, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSequence(tt.s, tt.arrayLen)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSequence() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSequence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseRange(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		arrayLen  int
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{
			name:      "explicit start and end",
			s:         "1:3",
			arrayLen:  10,
			wantStart: 1,
			wantEnd:   3,
		},
		{
			name:      "missing end resolves to last index",
			s:         "2:",
			arrayLen:  5,
			wantStart: 2,
			wantEnd:   4,
		},
		{
			name:      "missing start resolves to zero",
			s:         ":3",
			arrayLen:  10,
			wantStart: 0,
			wantEnd:   3,
		},
		{
			name:     "start after end is an error",
			s:        "5:1",
			arrayLen: 10,
			wantErr:  true,
		},
		{
			name:     "not a range at all",
			s:        "not-a-range",
			arrayLen: 10,
			wantErr:  true,
		},
		{
			// The start of the range is present but isn't a valid
			// integer, which must be reported as ErrInvalidInteger
			// rather than silently defaulting to 0 the way a genuinely
			// blank start does.
			name:     "invalid start value",
			s:        "abc:5",
			arrayLen: 10,
			wantErr:  true,
		},
		{
			// Same as above, but for the end of the range instead of the
			// start.
			name:     "invalid end value",
			s:        "5:abc",
			arrayLen: 10,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseRange(tt.s, tt.arrayLen)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRange() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("parseRange() = (%d, %d), want (%d, %d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
