package settings

import (
	"fmt"
	"reflect"
	"strings"
)

// kind is what a declared type is to the layers. A section is a struct whose fields are named
// values, a keyed type is a map whose keys are path segments, and a value is everything else,
// which a layer sets whole.
type kind int

const (
	valueKind kind = iota
	sectionKind
	keyedKind
)

// node is one declared type.
type node struct {
	kind   kind
	typ    reflect.Type
	fields []field // a section's fields, in declaration order
	elem   *node   // a keyed type's element
}

// field is one field of a section, named by its YAML tag. index is its path through the struct, one
// step for a field the section declares itself and more for one an inline section declares.
type field struct {
	name     string
	index    []int
	required bool
	node     *node
}

func (n *node) field(name string) (field, bool) {
	for _, f := range n.fields {
		if f.name == name {
			return f, true
		}
	}
	return field{}, false
}

// fileFlag and fileVariable name the file, as a flag and after the prefix in the environment. The
// root section may not declare the key whose environment name would be the same. versionFlag asks
// for the release version, and takes no value.
const (
	fileFlag     = "config-file"
	fileVariable = "CONFIG_FILE"
	versionFlag  = "version"
)

// declare walks the root type. Every refusal here is a mistake in the code, not in a deployment.
func declare(t reflect.Type) (*node, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("the root configuration type %s is not a struct", t)
	}
	root, err := declareType(t, "", false)
	if err != nil {
		return nil, err
	}
	if _, ok := root.field(strings.ToLower(fileVariable)); ok {
		return nil, fmt.Errorf("the root configuration type declares %s, whose environment name is the file's", strings.ToLower(fileVariable))
	}
	return root, nil
}

// declareType walks one type at path. Inside a value that a layer sets whole, a section is decoded
// as a struct and a map as a map, and neither is a path segment, so neither may hold a required
// field.
func declareType(t reflect.Type, path string, inValue bool) (*node, error) {
	switch t.Kind() {
	case reflect.Struct:
		n := &node{kind: sectionKind, typ: t}
		if inValue {
			n.kind = valueKind
		}
		for i := range t.NumField() {
			f := t.Field(i)
			name := f.Tag.Get("yaml")
			if f.Anonymous && name == inlineTag {
				if err := declareInline(n, f, i, path, inValue); err != nil {
					return nil, err
				}
				continue
			}
			at := join(path, name)
			switch {
			case !f.IsExported():
				return nil, fmt.Errorf("%s: field %s is unexported, so no layer can set it", describe(path, t), f.Name)
			case !isName(name):
				return nil, fmt.Errorf("%s: field %s has the yaml tag %q, and a name is lower-case letters and digits in words joined by single underscores, with no options", describe(path, t), f.Name, name)
			}
			if _, ok := n.field(name); ok {
				return nil, fmt.Errorf("%s is declared twice", at)
			}
			required := false
			switch tag := f.Tag.Get("settings"); tag {
			case "":
			case "required":
				required = true
			default:
				return nil, fmt.Errorf("%s has the settings tag %q, and the only one is required", at, tag)
			}
			child, err := declareType(f.Type, at, inValue)
			if err != nil {
				return nil, err
			}
			if required && (inValue || child.kind != valueKind) {
				return nil, fmt.Errorf("%s is required, and only a value outside a list can be", at)
			}
			n.fields = append(n.fields, field{name: name, index: []int{i}, required: required, node: child})
		}
		return n, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("%s: a map's keys must be strings", describe(path, t))
		}
		elem, err := declareType(t.Elem(), join(path, "<key>"), inValue)
		if err != nil {
			return nil, err
		}
		if inValue {
			return &node{kind: valueKind, typ: t}, nil
		}
		return &node{kind: keyedKind, typ: t, elem: elem}, nil
	case reflect.Slice:
		if _, err := declareType(t.Elem(), path, true); err != nil {
			return nil, err
		}
		return &node{kind: valueKind, typ: t}, nil
	case reflect.Bool, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &node{kind: valueKind, typ: t}, nil
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Pointer, reflect.UnsafePointer:
	}
	return nil, fmt.Errorf("%s: type %s cannot be configured", describe(path, t), t)
}

// inlineTag is the one yaml tag that is not a name. An embedded struct carrying it adds its fields to
// the section that embeds it, at the same path, as the YAML decoder reads it, so a type composed from
// sections declares each value once and keeps its names (ADR-0078).
const inlineTag = ",inline"

// declareInline adds the fields of f, an embedded struct tagged inline at index i of n's type, to n.
// The embedded type is a section, so its fields are named, checked and refused as n's own are, and a
// name it shares with another field of n is declared twice.
func declareInline(n *node, f reflect.StructField, i int, path string, inValue bool) error {
	switch {
	case !f.IsExported():
		return fmt.Errorf("%s: field %s is unexported, so no layer can set it", describe(path, n.typ), f.Name)
	case f.Type.Kind() != reflect.Struct:
		return fmt.Errorf("%s: field %s is inline and of type %s, and only a struct can be inline", describe(path, n.typ), f.Name, f.Type)
	case f.Tag.Get("settings") != "":
		return fmt.Errorf("%s: field %s is inline and has a settings tag, and only a value can be required", describe(path, n.typ), f.Name)
	}
	inner, err := declareType(f.Type, path, inValue)
	if err != nil {
		return err
	}
	for _, g := range inner.fields {
		if _, ok := n.field(g.name); ok {
			return fmt.Errorf("%s is declared twice", join(path, g.name))
		}
		g.index = append([]int{i}, g.index...)
		n.fields = append(n.fields, g)
	}
	return nil
}

// yamlField returns the type of the field of struct type t that name names, a field of an inline
// section included.
func yamlField(t reflect.Type, name string) (reflect.Type, bool) {
	for i := range t.NumField() {
		f := t.Field(i)
		switch tag := f.Tag.Get("yaml"); {
		case tag == name:
			return f.Type, true
		case f.Anonymous && tag == inlineTag && f.Type.Kind() == reflect.Struct:
			if inner, ok := yamlField(f.Type, name); ok {
				return inner, true
			}
		}
	}
	return nil, false
}

func describe(path string, t reflect.Type) string {
	if path == "" {
		return t.String()
	}
	return path
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// isName reports whether s is runs of lower-case letters and digits joined by single underscores,
// the form of a field name and of a map key. No segment starts or ends with an underscore or holds
// two in a row, so the environment name of a path splits back into exactly one path at each double
// underscore.
func isName(s string) bool {
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' || strings.Contains(s, "__") {
		return false
	}
	return isKeyAlphabet(s)
}

// checkKey refuses a map key that could not be a segment of an environment name (ADR-0078).
func checkKey(key string) error {
	if !isName(key) {
		return fmt.Errorf("the key %q is not runs of lower-case letters and digits joined by single underscores", key)
	}
	return nil
}

func isKeyAlphabet(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
