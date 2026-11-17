package settings

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
)

// A config.Duration is a number of nanoseconds, and the YAML decoder would read and write it as a
// bare number. So the library reads and writes it through a time.Duration, which the decoder reads
// only from text such as 5m, through time.ParseDuration, refusing a bare number, and writes back as
// such text (ADR-0078). Every type the YAML library sees is the declared type with each
// config.Duration in it a time.Duration, so the forms accepted and refused, the help, the effective
// values and the rendering a revision hashes are a time.Duration's.

// yamlType returns the type the YAML library reads and writes in place of t, t itself when t holds
// no config.Duration.
func yamlType(t reflect.Type) reflect.Type {
	return swapped(t, reflect.TypeFor[time.Duration]())
}

// textType returns the type an effective value is reported as in place of t, each config.Duration
// in it a string.
func textType(t reflect.Type) reflect.Type {
	return swapped(t, reflect.TypeFor[string]())
}

// swapped returns t with each config.Duration in it replaced by with, t itself when t holds none. A
// struct is rebuilt field by field under the same names and tags, an inline one included, which the
// YAML decoder reads by its tag alone.
func swapped(t, with reflect.Type) reflect.Type {
	if t == reflect.TypeFor[config.Duration]() {
		return with
	}
	if t.Kind() == reflect.Struct {
		fields := make([]reflect.StructField, t.NumField())
		changed := false
		for i := range t.NumField() {
			f := t.Field(i)
			ft := swapped(f.Type, with)
			changed = changed || ft != f.Type
			fields[i] = reflect.StructField{Name: f.Name, Type: ft, Tag: f.Tag}
		}
		if changed {
			return reflect.StructOf(fields)
		}
		return t
	}
	if t.Kind() == reflect.Map || t.Kind() == reflect.Slice {
		elem := swapped(t.Elem(), with)
		switch {
		case elem == t.Elem():
		case t.Kind() == reflect.Map:
			return reflect.MapOf(t.Key(), elem)
		default:
			return reflect.SliceOf(elem)
		}
	}
	return t
}

// toYAML returns v as a value of yamlType(v.Type()).
func toYAML(v reflect.Value) reflect.Value {
	return convert(v, yamlType(v.Type()))
}

// fromYAML returns y, a value of yamlType(t), as a value of t.
func fromYAML(y reflect.Value, t reflect.Type) reflect.Value {
	return convert(y, t)
}

// convert copies v into a new value of to, a type of the same shape whose durations are the other
// kind of duration or their text.
func convert(v reflect.Value, to reflect.Type) reflect.Value {
	if v.Type() == to {
		return v
	}
	out := reflect.New(to).Elem()
	k := v.Kind()
	switch {
	case k == reflect.Int64 && to.Kind() == reflect.String:
		out.SetString(time.Duration(v.Int()).String())
	case k == reflect.Int64:
		out.SetInt(v.Int())
	case k == reflect.Struct:
		for i := range v.NumField() {
			out.Field(i).Set(convert(v.Field(i), to.Field(i).Type))
		}
	case (k == reflect.Map || k == reflect.Slice) && v.IsNil():
	case k == reflect.Map:
		out.Set(reflect.MakeMapWithSize(to, v.Len()))
		for _, key := range v.MapKeys() {
			out.SetMapIndex(key, convert(v.MapIndex(key), to.Elem()))
		}
	case k == reflect.Slice:
		out.Set(reflect.MakeSlice(to, v.Len(), v.Len()))
		for i := range v.Len() {
			out.Index(i).Set(convert(v.Index(i), to.Elem()))
		}
	}
	return out
}

// decodeInto decodes text into a new value of t through yamlType(t), as decode does, and returns the
// value and the document's root node. A type error names the declared types, not the ones the YAML
// library read in their place.
func decodeInto(text []byte, t reflect.Type) (reflect.Value, *yaml.Node, error) {
	y := reflect.New(yamlType(t))
	doc, err := decode(text, y.Interface())
	if err != nil {
		return reflect.Value{}, nil, renamed(err, t)
	}
	return fromYAML(y.Elem(), t), doc, nil
}

// renamed returns err with every type the YAML library read in place of a declared type under t
// named as declared. The longest name is replaced first, since a struct's name holds the names of the
// types inside it.
func renamed(err error, t reflect.Type) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	pairs := map[string]string{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		y := yamlType(t)
		if y == t || t == reflect.TypeFor[config.Duration]() {
			return
		}
		pairs[y.String()] = t.String()
		if t.Kind() == reflect.Struct {
			for i := range t.NumField() {
				walk(t.Field(i).Type)
			}
			return
		}
		walk(t.Elem())
	}
	walk(t)
	names := slices.Collect(maps.Keys(pairs))
	slices.SortFunc(names, func(a, b string) int { return len(b) - len(a) })
	out := &yaml.TypeError{Errors: slices.Clone(typeErr.Errors)}
	for i, e := range out.Errors {
		for _, name := range names {
			e = strings.ReplaceAll(e, name, pairs[name])
		}
		out.Errors[i] = e
	}
	return out
}

// effective returns v as an effective value is reported, each config.Duration in it as its text,
// whether it is the value or sits in a list, a map or a section inside it.
func effective(v reflect.Value) any {
	return convert(v, textType(v.Type())).Interface()
}
