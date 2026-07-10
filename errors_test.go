package jaxon

import "testing"

// Test_Error_Error exercises *Error's Error() method directly. This method
// is what makes *Error satisfy the standard library's error interface, and
// it's the thing every caller sees when they print an error or format it
// with %v/%s - so its exact text matters even though nothing in this
// package's own error-construction code path happens to call it. Before
// this test, Error() had zero coverage: every other test only checked
// Extract()'s two return values, never Error()'s formatted string.
func Test_Error_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			// A nil *Error is a valid value to call Error() on (Go lets
			// you call a pointer-receiver method through a nil pointer,
			// as long as the method itself doesn't dereference it). This
			// package expects that and returns a placeholder string
			// instead of panicking.
			name: "nil receiver",
			err:  nil,
			want: "<nil>",
		},
		{
			// A code with no context (Ctx left as its zero value, "")
			// should format as just the bare code, with no trailing
			// ": " separator.
			name: "code with no context",
			err:  Err(ErrNotFound),
			want: ErrNotFound,
		},
		{
			// A code with context should format as "code: context".
			name: "code with context",
			err:  Err(ErrArrayIndex).Context(7),
			want: ErrArrayIndex + ": 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test_Error_Extract exercises *Error's Extract() method, in particular the
// nil-receiver case: every other test in this package only ever calls
// Extract() on a genuine, non-nil *Error returned from a failed query, so
// the "e == nil" branch was never actually exercised anywhere.
func Test_Error_Extract(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var err *Error

		code, ctx := err.Extract()
		if code != "" || ctx != "" {
			t.Errorf("Extract() = (%q, %q), want (\"\", \"\")", code, ctx)
		}
	})

	t.Run("code and context", func(t *testing.T) {
		err := Err(ErrInvalidRange).Context("1:2")

		code, ctx := err.Extract()
		if code != ErrInvalidRange {
			t.Errorf("Extract() code = %q, want %q", code, ErrInvalidRange)
		}

		if ctx != "1:2" {
			t.Errorf("Extract() context = %q, want %q", ctx, "1:2")
		}
	})

	t.Run("code with no context is an empty string, not omitted", func(t *testing.T) {
		err := Err(ErrNotFound)

		_, ctx := err.Extract()
		if ctx != "" {
			t.Errorf("Extract() context = %q, want empty string", ctx)
		}
	})
}
