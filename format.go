package jaxon

import (
	"encoding/json"
	"fmt"
	"math"
)

// format never actually returns a non-nil error given the current set of
// cases below: every path through this function either handles item
// directly (map, nil, the scalar fallback at the bottom) or recurses into
// format() for each element of an array, and that recursive call can only
// ever fail if some future case is added here that's capable of failing.
// The `if err != nil { return nil, err }` checks inside each array case
// below are intentionally kept anyway - they're what makes it safe to add
// such a case later without silently dropping its errors - but as things
// stand today, no input can reach them, and coverage tooling will always
// report them (and the matching check in GetItems, item.go) as untested.
func format(item any) ([]string, error) {
	// A JSON "null" value is decoded by encoding/json as a Go nil
	// interface value. Without this check, none of the type assertions
	// below would match a nil item, and it would fall all the way through
	// to fmt.Sprintf("%v", item) at the bottom of this function, which
	// prints Go's internal spelling of nil, "<nil>". Since callers only
	// think in terms of JSON, report it using JSON's own spelling of a
	// missing value, "null", instead.
	if item == nil {
		return []string{"null"}, nil
	}

	// If the item is a map, then reformat as more JSON.
	if m, ok := item.(map[string]any); ok {
		b, _ := json.MarshalIndent(m, "", "   ")

		return []string{string(b)}, nil
	}

	// If the item is an array, then reformat as more JSON.
	if a, ok := item.([]any); ok {
		var result []string

		for _, v := range a {
			r, err := format(v)
			if err != nil {
				return nil, err
			}

			result = append(result, r...)
		}

		return result, nil
	}

	// If the item is an array of int, then reformat as more JSON.
	if a, ok := item.([]int); ok {
		var result []string

		for _, v := range a {
			r, err := format(v)
			if err != nil {
				return nil, err
			}

			result = append(result, r...)
		}

		return result, nil
	}

	// If the item is an array of float64, then reformat as more JSON.
	if a, ok := item.([]float64); ok {
		var result []string

		for _, v := range a {
			r, err := format(v)
			if err != nil {
				return nil, err
			}

			result = append(result, r...)
		}

		return result, nil
	}

	// If the item is an array of strings, then reformat as more JSON.
	if a, ok := item.([]string); ok {
		var result []string

		for _, v := range a {
			r, err := format(v)
			if err != nil {
				return nil, err
			}

			result = append(result, r...)
		}

		return result, nil
	}

	// If it's a float, see if it should really be formatted
	// as an integer.
	if f, ok := item.(float64); ok {
		i := math.Floor(f)
		if i == f && math.Abs(i) < float64(math.MaxInt-1) {
			item = int(i)
		}
	}

	// Format it as the base object type.
	return []string{fmt.Sprintf("%v", item)}, nil
}
