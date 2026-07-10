package jaxon

import (
	"strconv"
	"strings"
)

// parseSequence parses an array-index specification into the concrete
// list of integer indices it describes. The specification can be:
//
//   - a single index, like "2"
//   - a comma-separated list of indices, like "1,3,5"
//   - a range, using either "-" or ":" as the separator, like "0-3" or "0:3"
//   - an open-ended range, where one side of the range is left out, like
//     "2:" (meaning "index 2 through the last element") or ":3" (meaning
//     "the first element through index 3")
//
// arrayLen is the number of elements in the array being indexed. It is
// only needed to resolve an open-ended range: since the specification
// text alone doesn't say where "the end" is, parseSequence has to be
// told how big the array actually is.
func parseSequence(s string, arrayLen int) ([]int, error) {
	var err error

	result := make([]int, 0)

	// Step 1, normalize "-" and ":" characters.
	s = strings.ReplaceAll(s, "-", ":")

	// Step 2, determine sets of values or ranges.
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if strings.Contains(part, ":") {
			// This is a range.
			start, end, err := parseRange(part, arrayLen)
			if err != nil {
				return nil, err
			}

			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// This is a single value.
			i, err := strconv.Atoi(part)
			if err != nil {
				return nil, Err(ErrInvalidInteger).Context(part)
			}

			result = append(result, i)
		}
	}

	return result, err
}

// parseRange parses one "start:end" range specification (any "-" form
// has already been normalized to ":" by the caller, parseSequence) and
// returns the start and end index as integers. Both ends are inclusive,
// so "0:2" describes the three indices 0, 1, and 2.
//
// Either side of the range may be left blank:
//   - An omitted start (e.g. ":2") defaults to 0, the first element in
//     the array. This matches the rest of the package, which is always
//     0-based.
//   - An omitted end (e.g. "2:") defaults to arrayLen-1, the last valid
//     index in the array, so the range reaches "to the end" rather than
//     some arbitrary fixed-size window.
func parseRange(s string, arrayLen int) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, Err(ErrInvalidRange).Context(s)
	}

	// Default to the first element when the start is left blank.
	start := 0

	if trimmed := strings.TrimSpace(parts[0]); trimmed != "" {
		var err error

		start, err = strconv.Atoi(trimmed)
		if err != nil {
			return 0, 0, Err(ErrInvalidInteger).Context(parts[0])
		}
	}

	// Default to the last element when the end is left blank.
	end := arrayLen - 1

	if trimmed := strings.TrimSpace(parts[1]); trimmed != "" {
		var err error

		end, err = strconv.Atoi(trimmed)
		if err != nil {
			return 0, 0, Err(ErrInvalidInteger).Context(parts[1])
		}
	}

	if start > end {
		return 0, 0, Err(ErrInvalidRange).Context(s)
	}

	return start, end, nil
}
