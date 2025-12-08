package jaxon

import (
	"encoding/json"
)

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
func GetItem(text string, item string) (string, error) {
	items, err := GetItems(text, item)
	if err == nil {
		switch len(items) {
		case 0:
			return "", Err(ErrNotFound).Context(item)

		case 1:
			return items[0], nil

		default:
			return "", Err(ErrAmbiguous).Context(item)
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
func GetItems(text string, item string) ([]string, error) {
	// Convert the body text to an arbitrary interface object using JSON
	var body any

	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, err
	}

	items, err := parse(body, item)
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
