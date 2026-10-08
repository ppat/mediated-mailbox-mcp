package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// decode decodes text holding one YAML document into out, strictly. The decoder refuses a
// duplicated key and, with KnownFields, a key the type does not declare, a case-changed one
// included, since it matches names exactly. It refuses neither a second document, an empty or null
// one, an explicit null, an alias nor a merge key, so parse refuses those first. It returns the
// document's root node.
func decode(text []byte, out any) (*yaml.Node, error) {
	root, err := parse(text)
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return nil, err
	}
	return root, nil
}

// parse reads text as exactly one YAML document holding no null, alias or merge key.
func parse(text []byte) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(text))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("holds no document, and an empty document is a null")
		}
		return nil, err
	}
	var next yaml.Node
	switch err := dec.Decode(&next); {
	case err == nil:
		return nil, fmt.Errorf("line %d: a second document starts, and only one is read", next.Line)
	case !errors.Is(err, io.EOF):
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, errors.New("holds no document, and an empty document is a null")
	}
	root := doc.Content[0]
	if err := check(root); err != nil {
		return nil, err
	}
	return root, nil
}

// check refuses an explicit null, an alias and a merge key anywhere under n. A null would decode
// as the zero value and hide that a value was meant, and an alias or a merge key makes one place in
// the text set values that another place names.
func check(n *yaml.Node) error {
	switch {
	case n.Kind == yaml.AliasNode:
		return fmt.Errorf("line %d: an alias, and aliases are refused", n.Line)
	case n.ShortTag() == "!!null":
		return fmt.Errorf("line %d: an explicit null, and a value is given or left out", n.Line)
	}
	for i, child := range n.Content {
		if n.Kind == yaml.MappingNode && i%2 == 0 && child.ShortTag() == "!!merge" {
			return fmt.Errorf("line %d: a merge key, and merge keys are refused", child.Line)
		}
		if err := check(child); err != nil {
			return err
		}
	}
	return nil
}
