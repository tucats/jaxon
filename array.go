package jaxon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// arrayLength reports how many elements are in body, provided body is one
// of the array/slice types this package knows how to index into (the
// same set of types handled by arrayElement and anyArrayElement below).
//
// This is used before parsing an index or range specification like "2:"
// or "0-3", so that an "open ended" range - one with a missing start or
// end - can be resolved against the real size of the array. For example,
// "2:" means "from index 2 to the last element," and answering that
// requires knowing how many elements the array actually has.
//
// If body is not one of the recognized array types, an ErrArrayType
// error is returned. The error's context records body's actual Go type
// (e.g. "string" or "map[string]any"), so a caller can tell what kind of
// value was queried instead of an array.
func arrayLength(body any) (int, error) {
	switch actual := body.(type) {
	case []any:
		return len(actual), nil
	case []string:
		return len(actual), nil
	case []float64:
		return len(actual), nil
	case []int:
		return len(actual), nil
	case []bool:
		return len(actual), nil
	default:
		return 0, Err(ErrArrayType).Context(fmt.Sprintf("%T", body))
	}
}

func arrayElement(body any, index int, parts []string, item string) ([]any, error) {
	var (
		result   []any
		subQuery string
	)

	if len(parts) > 0 {
		subQuery = strings.Join(parts, ".")
	}

	switch actual := body.(type) {
	case []any:
		if index < 0 || index >= len(actual) {
			return result, Err(ErrArrayIndex).Context(index)
		}

		return parse(actual[index], subQuery)

	case []string:
		if index < 0 || index >= len(actual) {
			return result, Err(ErrArrayIndex).Context(index)
		}

		return []any{actual[index]}, nil

	case []float64:
		if index < 0 || index >= len(actual) {
			return result, Err(ErrArrayIndex).Context(index)
		}

		return []any{actual[index]}, nil

	case []int:
		if index < 0 || index >= len(actual) {
			return result, Err(ErrArrayIndex).Context(index)
		}

		return []any{actual[index]}, nil

	case []bool:
		if index < 0 || index >= len(actual) {
			return result, Err(ErrArrayIndex).Context(index)
		}

		return []any{actual[index]}, nil

	default:
		// body isn't a type we can index into at all (for example, it's a
		// plain string or number). Report the actual Go type of body -
		// not item, the query string, which is always type string and
		// would make this error message say "string" no matter what was
		// actually queried - so the caller can see what went wrong.
		return result, Err(ErrArrayType).Context(fmt.Sprintf("%T", body))
	}
}

// collectMatches applies query to each value in elements in turn, and
// collects every successful result into a single list. This is the shared
// work behind every case of anyArrayElement's type switch below: once
// we're down to "here is a list of values, check each one against the
// query," it no longer matters whether those values came from a JSON array
// or (for a wildcard on an object) the values of a JSON object.
//
// The lenient flag controls what happens when an element does not match
// the query - for example, it's an object missing a field the query asked
// for:
//   - lenient == false (an ordinary wildcard, "*"): every element is
//     required to match. The first one that fails stops everything, and
//     its error is returned immediately.
//   - lenient == true (the "*?" lenient wildcard - the wildcard version of
//     the single-field "field?default" syntax elsewhere in this package):
//     an element that doesn't match is simply skipped, so the caller gets
//     back whatever did match, without an error.
func collectMatches(elements []any, query string, lenient bool) ([]any, error) {
	var result []any

	for _, element := range elements {
		text, err := parse(element, query)
		if err != nil {
			if lenient {
				continue
			}

			return nil, err
		}

		result = append(result, text...)
	}

	return result, nil
}

// mapValuesByKey returns the values of m as a slice, ordered by sorting the
// map's keys alphabetically first. A JSON object has no defined ordering of
// its own, and neither does a Go map - iterating over m directly would
// return its values in a different, randomized order every time the
// program runs. Sorting by key first gives a wildcard query against an
// object (see the map[string]any case in anyArrayElement below) a
// deterministic, repeatable result order that a caller can actually write
// a test against.
func mapValuesByKey(m map[string]any) []any {
	keys := make([]string, 0, len(m))

	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	values := make([]any, len(keys))
	for i, k := range keys {
		values[i] = m[k]
	}

	return values
}

func anyArrayElement(body any, parts []string, item string, lenient bool) ([]any, error) {
	var (
		query    string
		subQuery string
	)

	if len(parts) > 0 {
		query = parts[0]
		// See if the query string ends with a "." followed by an integer value.
		queryParts := splitQuery(query, 0)
		if len(queryParts) > 1 {
			lastItem := queryParts[len(queryParts)-1]
			if _, err := strconv.Atoi(lastItem); err == nil {
				query = strings.Join(queryParts[:len(queryParts)-1], ".")
				parts = append([]string{query, lastItem}, parts[1:]...)
			}
		}
	}

	if len(parts) > 1 {
		subQuery = strings.Join(parts[1:], ".")
	}

	// Gather up the values to apply the query to, as a plain []any,
	// regardless of which concrete type body actually is. Once we have
	// that list, collectMatches (above) does the actual work the same
	// way no matter where the values came from.
	var elements []any

	switch actual := body.(type) {
	case []any:
		elements = actual

	case []string:
		elements = make([]any, len(actual))
		for i, v := range actual {
			elements[i] = v
		}

	case []float64:
		elements = make([]any, len(actual))
		for i, v := range actual {
			elements[i] = v
		}

	case []int:
		elements = make([]any, len(actual))
		for i, v := range actual {
			elements[i] = v
		}

	case []bool:
		elements = make([]any, len(actual))
		for i, v := range actual {
			elements[i] = v
		}

	case map[string]any:
		// A wildcard on an object means "every value in the object," the
		// same way it means "every element of the array" for the cases
		// above - e.g. "config.*.enabled" pulls an "enabled" field out of
		// every entry in an object keyed by name.
		elements = mapValuesByKey(actual)

	default:
		return nil, Err(ErrArrayType).Context(fmt.Sprintf("%T", body))
	}

	result, err := collectMatches(elements, query, lenient)
	if err != nil {
		return nil, err
	}

	if len(result) > 0 {
		if subQuery != "" {
			return parse(result, subQuery)
		}

		return result, nil
	}

	return result, Err(ErrArrayNotFound).Context(item)
}
