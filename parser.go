package jaxon

import (
	"fmt"
	"strings"
)

// parse examines an arbitrary object, and applies a query string
// to locate sub-components of that object. The object is typically an
// array or map.
func parse(object any, queryText string) ([]any, error) {
	var (
		index   []int
		isIndex bool
		name    string
	)

	// If the item is just a "dot" it means the entire (remaining) body is the result
	if queryText == "." || queryText == "" {
		return []any{object}, nil
	}

	if strings.HasPrefix(queryText, "..") {
		return nil, Err(ErrJSONQuery).Context(queryText)
	}

	queryText = dotQuote(strings.TrimPrefix(queryText, "."))

	// Split out the item we seek plus whatever might be after it
	parts := splitQuery(queryText, 2)

	// splitQuery returns a completely empty slice when the query segment
	// itself is empty - which happens for an empty index specification
	// like "[]" (or "[ ]", brackets with nothing but whitespace inside).
	// That isn't a valid field name or array index of any kind, so it's
	// reported as its own dedicated error here, rather than being padded
	// into a single empty segment and left to fall through to the
	// "field not found" handling further down - which would eventually
	// produce an error too, but only as an accident of an empty string
	// not matching any field name, not because anyone decided that was
	// the right error for this case. (Without some check here at all,
	// the code below would try to read parts[0] on a zero-length slice
	// and crash the program with an "index out of range" panic.)
	if len(parts) == 0 {
		return nil, Err(ErrEmptyIndex).Context(queryText)
	}

	// If there was only one segment, there's nothing left to query after
	// it, so pad the list with a "." meaning "the rest of the value, as
	// found" (see the ". or empty string" check at the top of this
	// function).
	if len(parts) == 1 {
		parts = append(parts, ".")
	}

	for i, part := range parts {
		if i == 0 {
			parts[i] = dotUnquote(part, ".")
		} else {
			parts[i] = dotUnquote(part, "\\.")
		}
	}

	// Determine whether this segment is an array index/range specification
	// (like "3", "0-2", "1,4", or an open-ended range such as "2:" or
	// ":3") or an object field name.
	//
	// Note: by this point, any "[" and "]" bracket notation used in the
	// original query has already been converted into plain dot-separated
	// segments by splitQuery, so parts[0] never actually contains a
	// literal "[" or "]" here.
	//
	// A segment starting with a digit is unambiguously an index/range
	// (e.g. "3" or "0-2"). A segment starting with a colon is also an
	// index/range: it's an open-ended range whose starting index was
	// left out, such as ":2" meaning "from the beginning through index
	// 2".
	if startsWithDigit(parts[0]) || strings.HasPrefix(parts[0], ":") {
		// To resolve an open-ended range (one with a missing start or
		// end, like "2:" or ":3") we need to know how many elements are
		// in the array we're about to index into - that's what tells us
		// where "the end" actually is. arrayLength looks at the current
		// body value and returns its size, or an error if body isn't one
		// of the array types this package can index into at all.
		length, lenErr := arrayLength(object)
		if lenErr != nil {
			return nil, lenErr
		}

		var err error

		index, err = parseSequence(parts[0], length)
		if err != nil {
			return nil, err
		}

		isIndex = true
	} else {
		name = parts[0]
	}

	// Is the name a wildcard? A bare "*" means "every element (of an array)
	// or every value (of an object) must match the rest of the query" - an
	// error if even one doesn't. "*?" is the lenient variant: the wildcard
	// equivalent of the single-field "field?default" syntax handled below,
	// it means "collect whatever matches, and quietly skip anything that
	// doesn't" instead of failing the whole query over one mismatch.
	if name == "*" || name == "*?" {
		return anyArrayElement(object, parts[1:], queryText, name == "*?")
	}

	// If it's an index, the current item must be an array
	if isIndex {
		result := make([]any, 0, len(index))

		for _, i := range index {
			items, err := arrayElement(object, i, parts[1:], queryText)
			if err != nil {
				return nil, err
			}

			result = append(result, items...)
		}

		return result, nil
	}

	// If it's a name, the current item must be a map of some type.
	//
	// encoding/json always decodes a JSON object into a map[string]any -
	// never any other map type - so a plain type assertion is all that's
	// needed to check this and get at the map underneath. An earlier
	// version of this code used the reflect package here instead: it
	// called reflect.ValueOf(body) and then looped over every key in the
	// map (val.MapKeys()) comparing each one to name, one at a time,
	// until it found a match. That's a linear (O(n) - the time grows with
	// however many fields the object has) scan through every field using
	// the relatively slow reflect package, done again at every level of
	// every query, when a plain map index (m[name]) does the same lookup
	// in constant time (O(1) - the time doesn't depend on how many fields
	// there are) using nothing but ordinary Go map access.
	if m, ok := object.(map[string]any); ok {
		var (
			hasAlternate bool
			alternate    string
		)

		if punctuation := strings.Index(name, "?"); punctuation > 0 {
			alternate = name[punctuation+1:]
			name = name[:punctuation]
			hasAlternate = true
		}

		if v, found := m[name]; found {
			return parse(v, parts[1])
		}

		if hasAlternate {
			return []any{alternate}, nil
		}

		return nil, Err(ErrJSONElementNotFound).Context(name)
	}

	// We were asked to look up a field name, but body isn't a map at all
	// (for example, it's a plain string, number, or array). Report the
	// actual Go type of body - not item, the query string, which is
	// always type string and would make this error message useless no
	// matter what was actually queried - so the caller can see what kind
	// of value it tried to search inside.
	return nil, Err(ErrJSONInvalidContent).Context(fmt.Sprintf("%T", object))
}

func dotQuote(s string) string {
	return strings.ReplaceAll(s, "\\.", "$$DOT$$")
}

func dotUnquote(s string, target string) string {
	return strings.ReplaceAll(s, "$$DOT$$", target)
}

func splitQuery(s string, count int) []string {
	// Strip off any trailing dot
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")

	// splitQuery is called once per level of recursion as parse() works
	// its way through a query one dotted segment at a time (see parse()'s
	// calls to splitQuery, and its recursive calls to itself and to
	// arrayElement/anyArrayElement). By the time we get past the very
	// first segment of the original query, every "[" and "]" character
	// has already been converted into a "." by an earlier call to this
	// same function - that conversion always happens on the *entire*
	// remaining query string, not just the current segment, so there's
	// nothing left for a later call to convert.
	//
	// Without this check, every one of the eight strings.ReplaceAll /
	// TrimPrefix / TrimSuffix calls below would still run and scan all
	// of s looking for brackets that are no longer there, on every
	// single level of every query, for no benefit. A single
	// strings.ContainsAny check first lets us skip straight past all of
	// that wasted work whenever there's no bracket left to convert -
	// which, for everything past the first level, is always.
	if strings.ContainsAny(s, "[]") {
		// Hide away escaped brackets
		s = strings.ReplaceAll(s, "\\[", "$$LEFT-BRACKET$$")
		s = strings.ReplaceAll(s, "\\]", "$$RIGHT-BRACKET$$")

		// Convert unescaped brackets into dots so [] act as array index separators
		s = strings.ReplaceAll(s, ".[", "[")
		s = strings.ReplaceAll(s, "].", "]")
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		s = strings.ReplaceAll(s, "[", ".")
		s = strings.ReplaceAll(s, "]", ".")

		// Restore escaped brackets back into their original positions
		s = strings.ReplaceAll(s, "$$LEFT-BRACKET$$", "\\[")
		s = strings.ReplaceAll(s, "$$RIGHT-BRACKET$$", "\\]")
	}

	// Split the resulting dot-delimited parts into individual elements
	var parts []string

	if count == 0 {
		parts = strings.Split(s, ".")
	} else {
		parts = strings.SplitN(s, ".", count)
	}

	// Remove any extra whitespace from each part and we're done.
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	// Remove empty leading parts
	for len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	return parts
}

func startsWithDigit(s string) bool {
	if len(s) == 0 {
		return false
	}

	return '0' <= s[0] && s[0] <= '9'
}
