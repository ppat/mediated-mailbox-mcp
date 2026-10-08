package settings

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// help writes each value's path, environment name, flag and default, from the declaration and the
// defaults. A keyed map's values are listed once with <key> in place of the key, followed by each
// key the defaults hold.
func help(root *node, defaults reflect.Value) (string, error) {
	var b strings.Builder
	b.WriteString("Each value is set, in rising precedence, by its default, by the YAML file named by\n")
	fmt.Fprintf(&b, "--%s=PATH or %s%s, by its environment variable and\n", fileFlag, Prefix, fileVariable)
	b.WriteString("by its flag. A value other than a string is written as YAML, so a list is [a, b].\n")
	b.WriteString("A default that is an empty string or holds a newline is shown quoted, a newline as \\n.\n")
	var collected, defaultValues []assignment
	if err := collectDefaults(root, defaults, nil, &collected); err != nil {
		return "", err
	}
	for _, a := range collected {
		if !a.entry {
			defaultValues = append(defaultValues, a)
		}
	}
	byPath := map[string]reflect.Value{}
	for _, a := range defaultValues {
		byPath[strings.Join(a.path, ".")] = a.value
	}
	var entries []string
	var walk func(n *node, path []string, required bool) error
	walk = func(n *node, path []string, required bool) error {
		switch n.kind {
		case sectionKind:
			for _, f := range n.fields {
				if err := walk(f.node, append(slices.Clone(path), f.name), f.required); err != nil {
					return err
				}
			}
			return nil
		case keyedKind:
			return walk(n.elem, append(slices.Clone(path), "<key>"), false)
		case valueKind:
		}
		at := strings.Join(path, ".")
		entry := fmt.Sprintf("\n%s\n  environment: %s\n  flag: --%s=VALUE\n", at, environmentName(path), at)
		switch value, ok := byPath[at]; {
		case required:
			entry += "  required\n"
		case ok:
			text, err := render(value)
			if err != nil {
				return fmt.Errorf("%s: %w", at, err)
			}
			entry += "  default: " + text + "\n"
		}
		entries = append(entries, entry)
		return nil
	}
	if err := walk(root, nil, false); err != nil {
		return "", err
	}
	for _, e := range entries {
		b.WriteString(e)
	}
	keyed := false
	for _, a := range defaultValues {
		if !isKeyed(root, a.path) {
			continue
		}
		if !keyed {
			b.WriteString("\nDefaults of keyed values\n")
			keyed = true
		}
		text, err := render(a.value)
		if err != nil {
			return "", fmt.Errorf("%s: %w", strings.Join(a.path, "."), err)
		}
		fmt.Fprintf(&b, "  %s: %s\n", strings.Join(a.path, "."), text)
	}
	return b.String(), nil
}

// isKeyed reports whether path passes through a keyed map.
func isKeyed(root *node, path []string) bool {
	n := root
	for _, segment := range path {
		switch n.kind {
		case sectionKind:
			f, _ := n.field(segment)
			n = f.node
		case keyedKind:
			return true
		case valueKind:
		}
	}
	return false
}

// render writes v in the form an environment variable or a flag takes, a string verbatim and
// anything else as YAML in flow style. An empty string, or one holding a newline, is quoted as Go
// quotes it, since verbatim it would read as no default or break the entry across lines.
func render(v reflect.Value) (string, error) {
	if v.Kind() == reflect.String {
		if v.String() == "" || strings.Contains(v.String(), "\n") {
			return strconv.Quote(v.String()), nil
		}
		return v.String(), nil
	}
	var n yaml.Node
	if err := n.Encode(v.Interface()); err != nil {
		return "", err
	}
	flow(&n)
	text, err := yaml.Marshal(&n)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(text)), nil
}

func flow(n *yaml.Node) {
	n.Style |= yaml.FlowStyle
	for _, c := range n.Content {
		flow(c)
	}
}
