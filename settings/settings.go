// Package settings reads a deployable's configuration, layering built-in defaults, one optional
// YAML file, environment variables and command-line flags per value, in that order, and refusing
// every mistake in any layer at start (ADR-0078). settings/README.md argues why it is a shared
// library.
//
// # Names
//
// The root configuration type is a struct, and every value in it is named once, by its YAML key
// path. The environment name is Prefix plus the path upper-cased with its segments joined by a
// double underscore, and the flag is the path after two dashes, so scanner.triggers.de is
// MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE and --scanner.triggers.de=VALUE. A struct field is a
// section, a map with string keys is keyed, so each key is a segment of the path, and everything
// else is a value that a layer sets whole, a list included. A map key is runs of lower-case letters
// and digits joined by single underscores, so a language tag such as pt-BR is written pt_br and
// every environment name maps back to one path. A field tagged settings:"required" has no default
// and must be set by the file, the environment or a flag.
//
// The file is named by --config-file or MEDIATED_MAILBOX_CONFIG_FILE, the flag winning, and no file
// is read otherwise. A flag is written --path=VALUE, and an argument of any other form is refused,
// since no deployable has a mode. --help or -h returns a HelpRequested error holding the help text.
//
// # Values from the environment and flags
//
// A string value takes the text verbatim. Any other value decodes the text as one YAML document
// into the value's type, strictly, so a list is [verify, "log in"] and an empty text is refused.
//
// # Refusals
//
// Load refuses an unknown, duplicated or case-changed key, a second or empty document, an explicit
// null, an alias or a merge key, an unknown flag or one given twice, an unknown, repeated or
// mixed-case environment variable under the prefix, a map key outside its alphabet, a type error
// and a missing required value. A boolean in any layer, whether it is a whole value or sits in a
// list, a map or a section inside one, is true or false, and the other words YAML reads as
// booleans, such as yes and off, are refused. Each error names the file and line, the environment
// variable or the flag. A value outside its designed range is for each concern's own validation,
// which runs on what Load returns.
//
// # What Load returns
//
// Load returns the merged configuration, each value's path with the layer that set it, and a
// revision for each top-level section. The revision is a digest of the section's effective value,
// so a change in any layer changes it and a change to another section does not. Load holds no
// package-level state and shares no map or list with its defaults, so two loads in one process
// hold nothing of each other.
package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Prefix begins every environment variable of every deployable (ADR-0078).
const Prefix = "MEDIATED_MAILBOX_"

// Layer is where a value came from.
type Layer int

// The layers, in the order they apply.
const (
	Default Layer = iota
	File
	Environment
	Flag
)

// Source is the layer that set a value, and where in it.
type Source struct {
	Layer Layer
	// Name is the file and line, the environment variable or the flag, and empty for a default.
	Name string
}

func (s Source) String() string {
	switch s.Layer {
	case File:
		return "file " + s.Name
	case Environment:
		return "environment variable " + s.Name
	case Flag:
		return "flag " + s.Name
	case Default:
		return "default"
	}
	return fmt.Sprintf("layer %d %s", int(s.Layer), s.Name)
}

// Value is one value of the merged configuration.
type Value struct {
	Path   string
	Source Source
	Value  any
}

// Loaded is a deployable's configuration after every layer.
type Loaded[T any] struct {
	Config T
	// Values holds every value set in any layer, defaults included, sorted by path.
	Values []Value
	// Revisions holds a revision per top-level section, keyed by its name.
	Revisions map[string]string
}

// HelpRequested is the error Load returns when the arguments ask for help.
type HelpRequested struct {
	Text string
}

func (h *HelpRequested) Error() string { return "help requested" }

// assignment is one layer setting one value, or, when entry is true, naming one entry of a keyed
// map, which then exists in the result even when the layer sets nothing inside it. An entry named
// with nothing set still has its required values demanded.
type assignment struct {
	path   []string
	value  reflect.Value
	source Source
	entry  bool
}

// Load layers defaults, the file, the environment and the flags into a T. args are the arguments
// after the program name, and environ holds KEY=VALUE entries as os.Environ returns them.
func Load[T any](defaults T, args, environ []string) (Loaded[T], error) {
	var loaded Loaded[T]
	root, err := declare(reflect.TypeFor[T]())
	if err != nil {
		return loaded, err
	}
	if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
		text, err := help(root, reflect.ValueOf(defaults))
		if err != nil {
			return loaded, err
		}
		return loaded, &HelpRequested{Text: text}
	}

	var layers []assignment
	if err := collectDefaults(root, reflect.ValueOf(defaults), nil, &layers); err != nil {
		return loaded, err
	}
	flags, fileFromFlag, err := readFlags(root, args)
	if err != nil {
		return loaded, err
	}
	env, fileFromEnv, err := readEnvironment(root, environ)
	if err != nil {
		return loaded, err
	}
	switch {
	case fileFromFlag != "":
		layers, err = readFile[T](root, fileFromFlag, layers)
	case fileFromEnv != "":
		layers, err = readFile[T](root, fileFromEnv, layers)
	}
	if err != nil {
		return loaded, err
	}
	layers = append(layers, env...)
	layers = append(layers, flags...)

	result := reflect.ValueOf(&loaded.Config).Elem()
	sources := map[string]Source{}
	for _, a := range layers {
		if a.entry {
			set(result, root, a.path, reflect.Value{})
			continue
		}
		set(result, root, a.path, copyValue(a.value))
		sources[strings.Join(a.path, ".")] = a.source
	}
	if err := checkRequired(root, result, nil, sources); err != nil {
		return Loaded[T]{}, err
	}

	for path, source := range sources {
		loaded.Values = append(loaded.Values, Value{Path: path, Source: source, Value: copyValue(get(result, root, strings.Split(path, "."))).Interface()})
	}
	slices.SortFunc(loaded.Values, func(a, b Value) int { return strings.Compare(a.Path, b.Path) })
	loaded.Revisions = map[string]string{}
	for _, f := range root.fields {
		text, err := yaml.Marshal(result.Field(f.index).Interface())
		if err != nil {
			return Loaded[T]{}, fmt.Errorf("%s: %w", f.name, err)
		}
		sum := sha256.Sum256(text)
		loaded.Revisions[f.name] = hex.EncodeToString(sum[:16])
	}
	return loaded, nil
}

// collectDefaults adds every value the defaults hold under n, which is every value of a section and
// every key of a keyed map.
func collectDefaults(n *node, v reflect.Value, path []string, out *[]assignment) error {
	switch n.kind {
	case sectionKind:
		for _, f := range n.fields {
			fv := v.Field(f.index)
			at := append(slices.Clone(path), f.name)
			if f.required {
				if !fv.IsZero() {
					return fmt.Errorf("%s is required and has a default", strings.Join(at, "."))
				}
				continue
			}
			if err := collectDefaults(f.node, fv, at, out); err != nil {
				return err
			}
		}
	case keyedKind:
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, k := range keys {
			if err := checkKey(k.String()); err != nil {
				return fmt.Errorf("the default of %s: %w", strings.Join(path, "."), err)
			}
			at := append(slices.Clone(path), k.String())
			*out = append(*out, assignment{path: at, source: Source{Layer: Default}, entry: true})
			if err := collectDefaults(n.elem, v.MapIndex(k), at, out); err != nil {
				return err
			}
		}
	case valueKind:
		*out = append(*out, assignment{path: path, value: v, source: Source{Layer: Default}})
	}
	return nil
}

// readFile decodes the file into a T and adds a value for every path the file names.
func readFile[T any](root *node, name string, out []assignment) ([]assignment, error) {
	text, err := os.ReadFile(name) //nolint:gosec // The file is the one the deployment names.
	if err != nil {
		return nil, fmt.Errorf("config file %s: %w", name, err)
	}
	decoded := reflect.New(reflect.TypeFor[T]())
	doc, err := decode(text, decoded.Interface())
	if err != nil {
		return nil, fmt.Errorf("config file %s: %w", name, err)
	}
	if err := checkBooleans(doc, reflect.TypeFor[T]()); err != nil {
		return nil, fmt.Errorf("config file %s: %w", name, err)
	}
	if err := collectFile(root, doc, decoded.Elem(), nil, name, &out); err != nil {
		return nil, fmt.Errorf("config file %s: %w", name, err)
	}
	return out, nil
}

// collectFile walks the file's document beside the declaration and adds each value it names. The
// strict decode has already refused a key the declaration does not hold.
func collectFile(n *node, doc *yaml.Node, v reflect.Value, path []string, name string, out *[]assignment) error {
	switch n.kind {
	case sectionKind:
		for i := 0; i+1 < len(doc.Content); i += 2 {
			key := doc.Content[i].Value
			f, ok := n.field(key)
			if !ok {
				continue
			}
			if err := collectFile(f.node, doc.Content[i+1], v.Field(f.index), append(slices.Clone(path), key), name, out); err != nil {
				return err
			}
		}
	case keyedKind:
		for i := 0; i+1 < len(doc.Content); i += 2 {
			key := doc.Content[i].Value
			if err := checkKey(key); err != nil {
				return fmt.Errorf("line %d: %w", doc.Content[i].Line, err)
			}
			at := append(slices.Clone(path), key)
			*out = append(*out, assignment{path: at, source: Source{Layer: File, Name: fmt.Sprintf("%s line %d", name, doc.Content[i].Line)}, entry: true})
			if err := collectFile(n.elem, doc.Content[i+1], v.MapIndex(reflect.ValueOf(key).Convert(n.typ.Key())), at, name, out); err != nil {
				return err
			}
		}
	case valueKind:
		*out = append(*out, assignment{path: path, value: v, source: Source{Layer: File, Name: fmt.Sprintf("%s line %d", name, doc.Line)}})
	}
	return nil
}

// readEnvironment adds a value for every variable under the prefix, sorted by name, and returns the
// file the environment names.
func readEnvironment(root *node, environ []string) ([]assignment, string, error) {
	var out []assignment
	var file string
	seen := map[string]bool{}
	sorted := slices.Sorted(slices.Values(environ))
	for _, entry := range sorted {
		name, text, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if !strings.HasPrefix(upper, Prefix) {
			continue
		}
		switch {
		case name != upper:
			return nil, "", fmt.Errorf("environment variable %s: its name is not upper case, and names under %s are", name, Prefix)
		case seen[name]:
			return nil, "", fmt.Errorf("environment variable %s: it is set twice", name)
		}
		seen[name] = true
		rest := strings.TrimPrefix(name, Prefix)
		if rest == fileVariable {
			if text == "" {
				return nil, "", fmt.Errorf("environment variable %s: it names no file", name)
			}
			file = text
			continue
		}
		path := strings.Split(strings.ToLower(rest), "__")
		value, err := resolve(root, path, text)
		if err != nil {
			return nil, "", fmt.Errorf("environment variable %s: %w", name, err)
		}
		out = append(out, assignment{path: path, value: value, source: Source{Layer: Environment, Name: name}})
	}
	return out, file, nil
}

// readFlags adds a value for every flag, in order, and returns the file the flags name.
func readFlags(root *node, args []string) ([]assignment, string, error) {
	var out []assignment
	var file string
	seen := map[string]bool{}
	for _, arg := range args {
		spec, ok := strings.CutPrefix(arg, "--")
		if !ok {
			return nil, "", fmt.Errorf("argument %q: it is not a flag, and flags are written --path=VALUE", arg)
		}
		name, text, ok := strings.Cut(spec, "=")
		switch {
		case !ok:
			return nil, "", fmt.Errorf("flag --%s: it has no value, and flags are written --%s=VALUE", name, name)
		case seen[name]:
			return nil, "", fmt.Errorf("flag --%s: it is given twice", name)
		}
		seen[name] = true
		if name == fileFlag {
			if text == "" {
				return nil, "", fmt.Errorf("flag --%s: it names no file", name)
			}
			file = text
			continue
		}
		path := strings.Split(name, ".")
		value, err := resolve(root, path, text)
		if err != nil {
			return nil, "", fmt.Errorf("flag --%s: %w", name, err)
		}
		out = append(out, assignment{path: path, value: value, source: Source{Layer: Flag, Name: "--" + name}})
	}
	return out, file, nil
}

// resolve finds the value path names in the declaration and decodes text into its type.
func resolve(root *node, path []string, text string) (reflect.Value, error) {
	n := root
	for i, segment := range path {
		switch n.kind {
		case sectionKind:
			f, ok := n.field(segment)
			if !ok {
				return reflect.Value{}, fmt.Errorf("it names no configuration value, and %s declares no %q", describePath(path[:i]), segment)
			}
			n = f.node
		case keyedKind:
			if err := checkKey(segment); err != nil {
				return reflect.Value{}, err
			}
			n = n.elem
		case valueKind:
			return reflect.Value{}, fmt.Errorf("it names no configuration value, and %s is a value with no parts", strings.Join(path[:i], "."))
		}
	}
	if n.kind != valueKind {
		return reflect.Value{}, fmt.Errorf("it names %s, a section rather than a value", strings.Join(path, "."))
	}
	v := reflect.New(n.typ)
	if n.typ.Kind() == reflect.String {
		v.Elem().SetString(text)
		return v.Elem(), nil
	}
	doc, err := decode([]byte(text), v.Interface())
	if err != nil {
		return reflect.Value{}, err
	}
	if err := checkBooleans(doc, n.typ); err != nil {
		return reflect.Value{}, err
	}
	return v.Elem(), nil
}

// checkBooleans refuses a boolean under n, the text decoded into a value of type t, written other
// than true or false, in the file, the environment and flags alike. The decoder reads yes, on, no
// and off into a boolean field too.
func checkBooleans(n *yaml.Node, t reflect.Type) error {
	switch t.Kind() {
	case reflect.Bool:
		if n.Kind == yaml.ScalarNode && (n.ShortTag() != "!!bool" || (n.Value != "true" && n.Value != "false")) {
			return fmt.Errorf("line %d: %q is not a boolean, and a boolean is true or false", n.Line, n.Value)
		}
	case reflect.Slice:
		for _, c := range n.Content {
			if err := checkBooleans(c, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		for i := 1; i < len(n.Content); i += 2 {
			if err := checkBooleans(n.Content[i], t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i+1 < len(n.Content); i += 2 {
			for j := range t.NumField() {
				if t.Field(j).Tag.Get("yaml") == n.Content[i].Value {
					if err := checkBooleans(n.Content[i+1], t.Field(j).Type); err != nil {
						return err
					}
				}
			}
		}
	case reflect.Invalid, reflect.String, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Pointer, reflect.UnsafePointer:
	}
	return nil
}

func describePath(path []string) string {
	if len(path) == 0 {
		return "the configuration"
	}
	return strings.Join(path, ".")
}

// set stores value at path under v, creating each map on the way. An invalid value creates the
// way and stores nothing.
func set(v reflect.Value, n *node, path []string, value reflect.Value) {
	if len(path) == 0 {
		if value.IsValid() {
			v.Set(value)
		}
		return
	}
	switch n.kind {
	case sectionKind:
		f, _ := n.field(path[0])
		set(v.Field(f.index), f.node, path[1:], value)
	case keyedKind:
		if v.IsNil() {
			v.Set(reflect.MakeMap(n.typ))
		}
		key := reflect.ValueOf(path[0]).Convert(n.typ.Key())
		entry := reflect.New(n.typ.Elem()).Elem()
		if existing := v.MapIndex(key); existing.IsValid() {
			entry.Set(existing)
		}
		set(entry, n.elem, path[1:], value)
		v.SetMapIndex(key, entry)
	case valueKind:
	}
}

// get returns the value at path under v.
func get(v reflect.Value, n *node, path []string) reflect.Value {
	for _, segment := range path {
		switch n.kind {
		case sectionKind:
			f, _ := n.field(segment)
			v, n = v.Field(f.index), f.node
		case keyedKind:
			v, n = v.MapIndex(reflect.ValueOf(segment).Convert(n.typ.Key())), n.elem
		case valueKind:
		}
	}
	return v
}

// checkRequired refuses a required value no layer set, in every section and every keyed entry.
func checkRequired(n *node, v reflect.Value, path []string, sources map[string]Source) error {
	switch n.kind {
	case sectionKind:
		for _, f := range n.fields {
			at := append(slices.Clone(path), f.name)
			if f.required {
				if _, ok := sources[strings.Join(at, ".")]; !ok {
					return fmt.Errorf("%s is required, and neither the file, %s nor --%s sets it", strings.Join(at, "."), environmentName(at), strings.Join(at, "."))
				}
			}
			if err := checkRequired(f.node, v.Field(f.index), at, sources); err != nil {
				return err
			}
		}
	case keyedKind:
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, k := range keys {
			if err := checkRequired(n.elem, v.MapIndex(k), append(slices.Clone(path), k.String()), sources); err != nil {
				return err
			}
		}
	case valueKind:
	}
	return nil
}

func environmentName(path []string) string {
	return Prefix + strings.ToUpper(strings.Join(path, "__"))
}

// copyValue returns a copy of v that shares no map or list with it.
func copyValue(v reflect.Value) reflect.Value {
	out := reflect.New(v.Type()).Elem()
	k := v.Kind()
	switch {
	case (k == reflect.Slice || k == reflect.Map) && v.IsNil():
	case k == reflect.Slice:
		out.Set(reflect.MakeSlice(v.Type(), v.Len(), v.Len()))
		for i := range v.Len() {
			out.Index(i).Set(copyValue(v.Index(i)))
		}
	case k == reflect.Map:
		out.Set(reflect.MakeMapWithSize(v.Type(), v.Len()))
		for _, key := range v.MapKeys() {
			out.SetMapIndex(key, copyValue(v.MapIndex(key)))
		}
	case k == reflect.Struct:
		for i := range v.NumField() {
			out.Field(i).Set(copyValue(v.Field(i)))
		}
	default:
		out.Set(v)
	}
	return out
}
