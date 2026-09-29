// Package schema declares the shape of every value the UI's read API sends, as data. The dataset
// registry declares each row type through it and the handler list each bespoke response, and the
// contract generator turns the declarations into the OpenAPI document (ADR-0065). Nothing is inferred
// from a Go type by reflection, because the registry already knows every field, its nullability and
// its closed value set. The Go integration tests prove each served response matches its declaration.
package schema

// Kind is the JSON kind of a value.
type Kind int

// The kinds. Timestamp is a string holding an RFC 3339 time in UTC. Map is an object whose keys are
// not declared and whose values all have the type Items names. JSON is any JSON value, for a column
// the schema stores as JSON whose shape the design leaves to the writer.
const (
	String Kind = iota + 1
	Integer
	Number
	Boolean
	Timestamp
	Array
	Object
	Map
	JSON
)

// Type is one value's shape.
type Type struct {
	Kind Kind
	// Name makes an object a named component of the document, referenced wherever it is used.
	Name string
	// Nullable admits null as well as the kind.
	Nullable bool
	// Values is the closed set of a string's values, as stored.
	Values []string
	// Items is an array's element type, or a map's value type.
	Items *Type
	// Fields are an object's fields, in the order they are sent. Every field is always present.
	Fields []Field
	// Variants makes the type exactly one of several object types. Every object declares all its
	// fields and admits no other, so a value matches one variant only.
	Variants []Type
}

// Field is one field of an object.
type Field struct {
	Name string
	Type Type
}

// Str is a string, closed to values when any are given.
func Str(values ...string) Type { return Type{Kind: String, Values: values} }

// Int is an integer.
func Int() Type { return Type{Kind: Integer} }

// Num is a number.
func Num() Type { return Type{Kind: Number} }

// Bool is a boolean.
func Bool() Type { return Type{Kind: Boolean} }

// Time is a timestamp.
func Time() Type { return Type{Kind: Timestamp} }

// Any is any JSON value.
func Any() Type { return Type{Kind: JSON} }

// Null returns t admitting null.
func Null(t Type) Type {
	t.Nullable = true
	return t
}

// ArrayOf is an array of items.
func ArrayOf(items Type) Type { return Type{Kind: Array, Items: &items} }

// MapOf is an object with undeclared keys whose values are items.
func MapOf(items Type) Type { return Type{Kind: Map, Items: &items} }

// Obj is an object with the given fields, named when name is not empty.
func Obj(name string, fields ...Field) Type { return Type{Kind: Object, Name: name, Fields: fields} }

// F is a field.
func F(name string, t Type) Field { return Field{Name: name, Type: t} }

// OneOf is exactly one of several object types.
func OneOf(variants ...Type) Type {
	return Type{Kind: Object, Variants: variants}
}
