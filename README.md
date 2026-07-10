# JAXON JSON Query Processor

`jaxon` is a package that supports a query against an arbitrary JSON string. The query
can be used to extract a individual item from the JSON, or an array of items if there are
multiple possible values for the query expression.

The JSON value can be an array, an object (with named fields) or a scalar value (integer,
floating point value, boolean, or string). Arrays and objects can have nested JSON values.
The query string format allows you to specify the named field, the array index (a zero-based
value or an `*` indicating all values), or a dot indicating the entire value. Multiple dots separate each part of the query.

## About JAXON

The `jaxon` package grew out of working on creating a generalize REST API test harness, and
I needed to have rich queries against the resulting JSON payloads. This become more useful
as I also worked on other tools that might generate large JSON payloads, and it would be
nice to add the ability to query the results and/or simplify them without having to add on
additional dependancies like `jq`. Thus, `jaxon` came about.

The project was created without AI, but subseuqently Claude Code (Opus 4.6 model) was used
to audit the code and write more comprehensive unit tests.

*If `jaxon` had a logo, it would be an orange tabby cat looking at you suspiciously, wondering
what it is you want.*

## API

There are two API entry points, depending on whether the query is expected to return
a single value, or an array of values. The first parameter is a string containing the
JSON text to evaluate, and the second parameter is the query expression.

For example, to read a single value, use:

```go
value, err := jaxon.GetItem(jsonText, "user.age")
```

The result value is always a string, which contains the text representation of the
result. So if the query is expected to return an integer value, the caller would
need to convert the resulting value to an integer (using `strconv.Atoi()`, for
example).

The function returns an error if the query results in anything other than a single
value. If there are multiple possible results to the query, the error reports that
the query was ambiguous. If there are no results, the query reports an error code
of "not found".

To get an array of results, use:

```go
value, err := jaxon.GetItems(jsonText, "items[0:3]")
```

In this example, value is an array of strings, each of which represents one of
the possible items returned by the query. If the query produces no values, then
it is not an error but the array will have a length of zero. In the above example,
the array should contain four values, being the array indexes 0, 1, 2, and 3 from
the array named `items` in the JSON text.

## Querying Go Values

If you have a Go value instead of a JSON string - a struct, map, slice, or
any combination of these - you can query it directly with `GetObjectItem`
and `GetObjectItems`, without having to marshal it to JSON yourself first.
They work exactly like `GetItem` and `GetItems`, except the first parameter
is an arbitrary Go value instead of a JSON string; the value is converted
to JSON internally (the same way `encoding/json`'s `json.Marshal` would)
and then queried exactly as if you had passed that JSON text to `GetItem`
or `GetItems` directly.

For example, given a struct:

```go
type Person struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

value, err := jaxon.GetObjectItem(Person{Name: "John Doe", Age: 30}, "name")
```

This returns "John Doe", the same result you'd get from marshaling the
struct to JSON yourself and calling `jaxon.GetItem` with it.

`GetObjectItems` is to `GetObjectItem` what `GetItems` is to `GetItem`: it
returns an array of string results instead of a single one. For example,

```go
people := []Person{
    {Name: "John Doe", Age: 30},
    {Name: "Jane Doe", Age: 28},
}

values, err := jaxon.GetObjectItems(people, "*.name")
```

returns the two names, "John Doe" and "Jane Doe".

Because the Go value is converted to JSON using the standard `encoding/json`
rules, the usual `json:"..."` struct tags apply: the query string refers to
the JSON field names as they appear in the marshaled JSON (`"name"` in the
example above), which are not always the same as the Go struct field names
(`Name`). If the value cannot be converted to JSON at all (for example, it
contains a channel or a function value), an error is returned.

## Single items

A single item (or the last item in a query) can be specified using the "." character. For
example, if the JSON query consists of a single value (a string in this example), then
the query result is that string.

```json
"This is a test"
````

The result of the query "." (or an empty string which means the same thing) will be the
string "This is a test". Similarly, if the JSON contains just an integer value,

```json
1337
```

The result of the query "." is the integer value `1337`.

## Named Field

If the value is an object, then the query string can be a specific named field in
that object, and the result is the value of that field. For example,

```json
{
    "name": "Fred"
}
```

A query value of "name" will return the string "Fred". Similarly, if the named
field is itself a complex item, the result will be the entire item. So for the
value:

```json
{
    "person": {
        "name": "Bob",
        "age": 53
    }
}
```

A query string of "person" will return the string `{"name": "Bob", "age": 53}`
as the result. Note that you can specify cascading values in the query string
when the value is itself and object, by specifying the field names separated
by a "." character. For the above example, the query "person.age" will return
the result `53`, since that is the value of the "age" field within the "person"
object.

A query can be made for a field that may or may not exist (an optional field)
by putting a question mark after the query followed by a value to use if the
field was not found. For example a query of ".foo?1" will return the value of
the field "foo", but if `foo` does not exist, it will return a value of "1"
as the result. The optional value can only be a single value, not an array
or object item.

## Arrays

When the object is an array, the query can specify either a specific numeric
array index, with the first item in the array having index 0 (zero). Alternatively
the array index can be an asterisk `*` character, which means _all array elements_.
For the JSON value

```json
[
    1,
    15,
    66
]
```

The query string "2" will return the value `66` since that is the value at the third
(0-based) array index. You can also specify a index by placing in brackets, such as
a query string of "[2]".

You can specify a range of indexes into the array by putting
multiple index values separate by commas, or specifying a range using a hyphen. For example,
the query string "0-1" will return a list of values containing `1` and `15`
as the items at index positions 0 and 1. Note that a query of "0,1" is the same
as "0:1" since both specify only two index values. However, a query string of "0:2"
specifies three index values (`0`, `1`, and `2`).

Either side of a range can be left out to mean "the rest of the array" on that
side. A query string of "2:" (the end left out) means "index 2 through the
last element," and a query string of ":2" (the start left out) means "the
first element through index 2." For the JSON value shown above, the query
string "2:" returns `66`, and the query string ":2" returns `1` and `15` and
`66`. The same works with brackets, as "[2:]" and "[:2]".

An empty pair of brackets, "[]", is not a valid index or range - there's
nothing there to say which element(s) you mean - so it's always an error,
regardless of whether it's applied to an array or an object.

Similar to nested objects, you can reference additional information
about the array element using the "dot" notation. For example,

```json
{
    "items": [
        {
            "name": "Bob",
            "age": 56,
        },
        {
            "name": "Sue",
            "age": 52,
        }
    ]
}
```

To get the age value for the second item in the array, you can use a query
string of "items[1].age". This looks for the field "items" in the object, finds
the second (0-based) array element, and within that item, finds the field named
"age" to return the integer `52`.

Note that you can specify _all_ the elements in the array in the query string
to get a list of items from the JSON object. For the above value, a query
string of "items.*.name" will return two string values,

```text
Bob
Sue
```

As the result. The items will be separated by a newline in the output of the
query expression.

## Wildcards

As shown above, a `*` query segment matches every element of an array. The
same `*` also works on an object: it matches every value in the object,
applying the rest of the query to each one and collecting the results into
a list, the same way it does for an array. For example, for the value

```json
{
    "flags": {
        "beta": true,
        "dark_mode": false,
        "new_ui": true
    }
}
```

the query string "flags.*" will return the three values `true`, `false`,
and `true`. A JSON object doesn't define an order for its fields the way an
array does, so results from a `*` on an object are always sorted by field
name first (here, "beta", "dark_mode", "new_ui") to keep the result
predictable and repeatable from one call to the next.

By default, `*` is strict: every element (or every value) has to match the
rest of the query, and if even one of them doesn't - for example, one
object in an array of objects is missing a field the query asks for - the
whole query fails with an error. If you'd rather collect whatever matches
and quietly ignore anything that doesn't, use `*?` instead of `*`. This is
the wildcard equivalent of the optional-field `field?default` syntax
described above: both use a `?` to mean "this might not be there, and
that's OK." For example, given

```json
[
    { "name": "Alice" },
    { "id": 42 },
    { "name": "Carol" }
]
```

the query string `*.name` fails, since the second array element has no
"name" field. The query string `*?.name` instead succeeds, returning just
`Alice` and `Carol` and silently skipping the element that didn't match.

## Errors

It is an error to specify a field name in an object that does not exist. It
is also an error to specify a field name when the value is not an object.

It is an error to specify a query string for an array index that does not exist
(a value larger than the length of the array) or for a value that is not an
array.

Similarly, it is an error to use array notation for a value that is not an array.

`jaxon` errors are represented by the `Error` type, which implements the
`error` interface. Since `GetItem` and `GetItems` return the generic `error`
interface type, you need to type-assert the result to `*jaxon.Error` before
you can call `Extract()` to get the error code and the context value (if
any) for the error:

```go
value, err := jaxon.GetItem(jsonText, "user.age")
if err != nil {
    if jaxonErr, ok := err.(*jaxon.Error); ok {
        code, context := jaxonErr.Extract()
        // ...
    }
}
```

The code is a string value that uniquely identifies each error. Some errors
have additional context (such as an unrecognized name, etc.) and these
values are returned as the context. If the error does not have a context,
the value is an empty string when returned from `Extract()`.

Note that not every error returned by `GetItem`/`GetItems` is a
`*jaxon.Error`: if the JSON text itself cannot be parsed, the underlying
`encoding/json` error is returned unchanged, and the type assertion above
will fail (`ok` will be `false`) for that case.
