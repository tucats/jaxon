package jaxon

import (
	"encoding/json"
)

// GetObjectItem extracts a specific item from an arbitrary Go object. This can
// be a map, an array, a structure, or any combination thereof. The object
// is converted to a JSON representation, and then passed on for query processing.
// The result is always expressed as a string.
func GetObjectItem(object any, queryText string) (string, error) {
	b, err := json.Marshal(object)
	if err != nil {
		// Without this fix, a marshal failure (for example, object contains
		// a channel or a function value, neither of which JSON can
		// represent) was silently discarded here - the caller got back an
		// empty string and a nil error, with no way to tell that anything
		// had gone wrong. Returning err instead of nil reports the actual
		// problem, the same way GetItem/GetItems already do for their own
		// errors.
		return "", err
	}

	return GetItem(string(b), queryText)
}

// GetObjectItems extracts all items from an arbitrary Go object that
// correspond to the query. This can be a map, an array, a structure, or any
// combination thereof. The object is converted to a JSON representation,
// and then passed on for query processing. The results are always
// expressed as an array of strings.
func GetObjectItems(object any, queryText string) ([]string, error) {
	b, err := json.Marshal(object)
	if err != nil {
		// See the comment in GetObjectItem above: this used to return nil
		// for the error here too, silently hiding a marshal failure
		// instead of reporting it.
		return nil, err
	}

	return GetItems(string(b), queryText)
}

// GetItem extracts a specific item from the JSON payload. The item specification
// is a dot-notation string that can include integer indices and string map key
// values. The value is always returned as a string representation.
//
// Example:
//
//	v, err := GetItem(`{"name": "John Doe", "age": 30}`, "name")
//
// This returns "John Doe" as the value of the "name" field. If an error occurs
// because the JSON is invalid or the query is invalid, then the value is an
// empty string and the error.
//
// If the query returns no items, ErrNotFound is returned. If the query returns
// multiple items, ErrAmbiguous is returned. If the query is expected to return
// multiple items, use the GetItems() function instead.
func GetItem(jsonText string, queryText string) (string, error) {
	items, err := GetItems(jsonText, queryText)
	if err == nil {
		switch len(items) {
		case 0:
			return "", Err(ErrNotFound).Context(queryText)

		case 1:
			return items[0], nil

		default:
			return "", Err(ErrAmbiguous).Context(queryText)
		}
	}

	return "", err
}

// GetItem extracts all item from the JSON payload that correspond to the dot-notation
// string that can include integer indices and string map key values. The values are
// always returned as an array of string representation of the selected values.
//
// Example:
//
//	v, err := GetItem(`[{"name": "John Doe", "age": 30}]`, "[*].name")
//
// This returns an array with on element containing "John Doe". This is read by
// scanning all members of the array in the JSON and getting the "name" field
// from each element. If an error occurs because the JSON is invalid or the
// query is invalid, then the value is an nil array and the error.
//
// If the query returns no items, the array is a non-nil empty array. If the query
// is expected to return exactly one item, you can call the GetItem() function
// instead.
func GetItems(jsonText string, queryText string) ([]string, error) {
	// Convert the body text to an arbitrary interface object using JSON
	var body any

	if err := json.Unmarshal([]byte(jsonText), &body); err != nil {
		return nil, err
	}

	items, err := parse(body, queryText)
	if err != nil {
		return nil, err
	}

	var result []string

	for _, item := range items {
		text, err := format(item)
		if err != nil {
			return nil, err
		}

		result = append(result, text...)
	}

	return result, nil
}
