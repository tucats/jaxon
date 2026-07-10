package jaxon

import (
	"encoding/json"
	"fmt"
	"testing"
)

// buildWideObjectJSON builds a JSON object with fieldCount top-level fields,
// named "field0", "field1", and so on. It's used below to benchmark looking
// up a field name in a large object.
func buildWideObjectJSON(fieldCount int) string {
	m := make(map[string]any, fieldCount)

	for i := 0; i < fieldCount; i++ {
		m[fmt.Sprintf("field%d", i)] = i
	}

	b, _ := json.Marshal(m)

	return string(b)
}

// BenchmarkGetItem_WideObject looks up a field in a 500-field object.
//
// This exercises the field-name lookup fixed in the performance pass: it
// used to walk every one of the object's keys one at a time with the
// reflect package looking for a match (an O(n) scan, where n is the field
// count), and now uses a single, direct map index instead (an O(1)
// lookup, regardless of field count).
func BenchmarkGetItem_WideObject(b *testing.B) {
	text := buildWideObjectJSON(500)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := GetItem(text, "field499"); err != nil {
			b.Fatal(err)
		}
	}
}

// buildDeepNestingFixture builds a JSON document nested depth levels deep,
// e.g. for depth 3: {"a0":{"a1":{"a2":"leaf"}}}, along with the matching
// query string "a0.a1.a2" that reaches the innermost value.
func buildDeepNestingFixture(depth int) (jsonText string, query string) {
	jsonText = `"leaf"`

	for i := depth - 1; i >= 0; i-- {
		jsonText = fmt.Sprintf(`{"a%d": %s}`, i, jsonText)
	}

	for i := 0; i < depth; i++ {
		if i > 0 {
			query += "."
		}

		query += fmt.Sprintf("a%d", i)
	}

	return jsonText, query
}

// BenchmarkGetItem_DeepNesting queries 20 levels deep into a nested object.
//
// Each level of nesting is one recursive call into parse(), and each of
// those calls used to re-run splitQuery's full bracket-notation handling
// (several string scans looking for "[" and "]") even though, past the
// very first level, there are never any brackets left to find - they were
// already all converted away in the first pass over the whole query.
// splitQuery now checks for a bracket character once before bothering to
// do any of that work, so every level past the first skips it entirely.
func BenchmarkGetItem_DeepNesting(b *testing.B) {
	text, query := buildDeepNestingFixture(20)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := GetItem(text, query); err != nil {
			b.Fatal(err)
		}
	}
}
