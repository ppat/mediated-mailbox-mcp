package service

import (
	"encoding/json"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/operation"
)

// Effect is the one declaration an operation makes about what it does to the mailbox, from which its
// HTTP method and its MCP annotations derive (ADR-0087). It is the operation core's, named here so the
// roots, which reach no lower than this layer, can read it.
type Effect = operation.Effect

// The effects an operation declares.
const (
	Read           = operation.Read
	StructuredRead = operation.StructuredRead
	Create         = operation.Create
	Reversible     = operation.Reversible
	Disposal       = operation.Disposal
)

// Annotations are an operation's four MCP annotations, derived from its effect.
type Annotations = operation.Annotations

// accountsPath is the accounts listing's path, and accountPrefix the prefix of every other
// operation's path (ADR-0087).
const (
	accountsPath  = operation.AccountsPath
	accountPrefix = operation.AccountPrefix
)

// schemaFacts are what the generation checks read from an input schema.
type schemaFacts struct {
	Properties map[string]struct {
		Type any `json:"type"`
	} `json:"properties"`
	Required []string `json:"required"`
}

// readSchema reads what the generation checks and the roots read of a schema out of its JSON.
// NewRegistry reads each schema through it once, and the checks and the descriptors share the result.
func readSchema(schema json.RawMessage) operation.Schema {
	s := operation.Schema{Status: holdsKey(schema, "status"), MCPHeader: holdsKey(schema, "x-mcp-header")}
	if err := objectSchema(schema); err != nil {
		s.Problem = err.Error()
		return s
	}
	var facts schemaFacts
	if err := json.Unmarshal(schema, &facts); err != nil {
		s.Unreadable = true
		return s
	}
	s.Types = map[string]string{}
	for name, p := range facts.Properties {
		if t, ok := p.Type.(string); ok {
			s.Types[name] = t
		} else {
			s.Types[name] = ""
		}
	}
	s.Required = facts.Required
	return s
}

// facts returns what the generation checks read of op.
func facts(op Operation) operation.Facts {
	return operation.Facts{
		Name: op.Name, Description: op.Description, Effect: op.Effect, Path: op.Path,
		Input: readSchema(op.Input), Output: readSchema(op.Output), Handled: op.Handle != nil,
	}
}

// holdsKey reports whether a JSON document holds key as an object key at any depth.
func holdsKey(doc json.RawMessage, key string) bool {
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return false
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if strings.EqualFold(k, key) || walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range x {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}
