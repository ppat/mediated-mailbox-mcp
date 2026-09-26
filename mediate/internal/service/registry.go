package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Operation is one operation of the client surface, carrying everything both roots need (ADR-0053).
// Its name is its identity on both roots, the API operation's path segment and the MCP tool's name,
// so no field exists for one root alone and a one-sided operation has nothing to be written in.
type Operation struct {
	// Name identifies the operation on both roots. It is lowercase letters, digits and underscores,
	// starting with a letter, at most 64 characters, which is a valid MCP tool name and a path segment
	// needing no escaping.
	Name string
	// Description is the operation's contract-grade text, the MCP tool's description and the API
	// operation's description in the contract document.
	Description string
	// Input is the JSON Schema of the arguments, an object schema.
	Input json.RawMessage
	// Output is the JSON Schema of the result, an object schema.
	Output json.RawMessage
	// Handle runs the operation and returns its JSON result. account is the account the registry
	// verified, and input is the call's JSON arguments without their top-level account_id, so the
	// operation acts on the verified account and has no other top-level account to read. An
	// account_id nested inside another argument is that argument's own. The accounts listing gets an
	// empty account and its arguments as sent. Handle decodes and validates the rest of its input
	// itself.
	Handle func(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error)
}

// Descriptor is what a root is generated from, an operation without its handler. A root runs an
// operation only through Registry.Call, so every call passes the account check.
type Descriptor struct {
	Name        string
	Description string
	Input       json.RawMessage
	Output      json.RawMessage
}

// accountsListing is the name of the one operation that takes no account, the listing that makes
// every account identifier discoverable (ADR-0035, ADR-0087).
const accountsListing = "list_accounts"

// ErrMissingAccount is returned for a call whose arguments carry no account_id string.
var ErrMissingAccount = errors.New("the operation needs account_id, and the call carries none")

// ErrUnknownAccount is returned for a call whose account_id names no account the mediator serves.
var ErrUnknownAccount = errors.New("the operation's account_id names no account the mediator serves")

// ErrAmbiguousAccount is returned for a call whose arguments hold account_id more than once, or a
// key that differs from account_id only in case. A reader matching keys without regard to case, or
// keeping the last of two, would take a different account from the one checked.
var ErrAmbiguousAccount = errors.New("the call's arguments name the account more than once")

// Failure returns the structured content a failed call answers with on both roots. It says only that
// the call failed.
func Failure(error) json.RawMessage { return json.RawMessage(`{"error":"the operation failed"}`) }

// ErrNoOperation is returned for a call naming an operation the registry does not hold.
var ErrNoOperation = errors.New("no such operation")

// Registry is the client surface's operation set, validated. Both roots are generated from it, and a
// root serves exactly its operations. The zero value holds no operation.
type Registry struct {
	ops   []Operation
	known map[string]bool
}

// validName is the shape of an operation's name.
var validName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// NewRegistry validates ops and returns the registry holding them, or every problem found. An
// operation with no name, no description, no handler or an input or output that is not an object
// schema could be generated onto one root and not the other, so it fails generation here.
//
// Every operation but the accounts listing takes the account as its required argument account_id
// (ADR-0087). An operation whose input schema does not require it fails generation. accounts are
// the accounts the mediator serves, which Call checks every account against. Both roots run an
// operation only through Call, so the check is one check for both.
func NewRegistry(accounts []string, ops ...Operation) (Registry, error) {
	var problems []string
	seen := map[string]bool{}
	for i, op := range ops {
		where := "operation " + strconv.Itoa(i) + " (" + strconv.Quote(op.Name) + ")"
		if !validName.MatchString(op.Name) {
			problems = append(problems, where+" has a name that is not lowercase letters, digits and underscores starting with a letter, at most 64 characters")
		}
		if seen[op.Name] {
			problems = append(problems, where+" repeats a name")
		}
		seen[op.Name] = true
		if strings.TrimSpace(op.Description) == "" {
			problems = append(problems, where+" has no description")
		}
		if err := objectSchema(op.Input); err != nil {
			problems = append(problems, where+" has an input schema that "+err.Error())
		} else if op.Name != accountsListing && !requiresAccount(op.Input) {
			problems = append(problems, where+" has an input schema that does not require the string account_id")
		}
		if err := objectSchema(op.Output); err != nil {
			problems = append(problems, where+" has an output schema that "+err.Error())
		}
		if op.Handle == nil {
			problems = append(problems, where+" has no handler")
		}
	}
	if len(problems) > 0 {
		return Registry{}, errors.New("the operation registry does not generate: " + strings.Join(problems, "; "))
	}
	known := map[string]bool{}
	for _, account := range accounts {
		known[account] = account != ""
	}
	sorted := slices.Clone(ops)
	slices.SortFunc(sorted, func(a, b Operation) int { return strings.Compare(a.Name, b.Name) })
	for i := range sorted {
		sorted[i].Input = compact(sorted[i].Input)
		sorted[i].Output = compact(sorted[i].Output)
	}
	return Registry{ops: sorted, known: known}, nil
}

// Call runs the operation named name on a call's JSON arguments. Every operation but the accounts
// listing first has its account checked. The call is refused before the operation runs when its
// arguments hold no account_id string, name the account more than once, or name an account the
// mediator does not serve, and otherwise the operation gets the account and the rest of its
// arguments.
func (r Registry) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	op, found := r.lookup(name)
	if !found {
		return nil, ErrNoOperation
	}
	if op.Name == accountsListing {
		return op.Handle(ctx, "", input)
	}
	account, rest, err := splitAccount(input)
	if err != nil {
		return nil, err
	}
	if !r.known[account] {
		return nil, ErrUnknownAccount
	}
	return op.Handle(ctx, account, rest)
}

// splitAccount reads a JSON object's top-level account_id string and returns it with the object
// holding every other member, in order. Keys are compared exactly, so a key differing from
// account_id only in case is refused rather than read.
func splitAccount(input json.RawMessage) (string, json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return "", nil, ErrMissingAccount
	}
	var account json.RawMessage
	var rest bytes.Buffer
	rest.WriteByte('{')
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return "", nil, ErrMissingAccount
		}
		key, ok := tok.(string)
		if !ok {
			return "", nil, ErrMissingAccount
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return "", nil, ErrMissingAccount
		}
		if strings.EqualFold(key, "account_id") {
			if key != "account_id" || account != nil {
				return "", nil, ErrAmbiguousAccount
			}
			account = value
			continue
		}
		if rest.Len() > 1 {
			rest.WriteByte(',')
		}
		quoted, err := json.Marshal(key)
		if err != nil {
			return "", nil, ErrMissingAccount
		}
		rest.Write(quoted)
		rest.WriteByte(':')
		rest.Write(value)
	}
	rest.WriteByte('}')
	var id *string
	if account == nil || json.Unmarshal(account, &id) != nil || id == nil {
		return "", nil, ErrMissingAccount
	}
	return *id, rest.Bytes(), nil
}

// requiresAccount reports whether an input schema declares account_id as a required string.
func requiresAccount(schema json.RawMessage) bool {
	var s struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if json.Unmarshal(schema, &s) != nil {
		return false
	}
	p, ok := s.Properties["account_id"]
	return ok && p.Type == "string" && slices.Contains(s.Required, "account_id")
}

// Operations returns the registry's operations without their handlers, sorted by name.
func (r Registry) Operations() []Descriptor {
	out := make([]Descriptor, 0, len(r.ops))
	for _, op := range r.ops {
		out = append(out, describe(op))
	}
	return out
}

func describe(op Operation) Descriptor {
	return Descriptor{Name: op.Name, Description: op.Description, Input: op.Input, Output: op.Output}
}

// Lookup returns the description of the operation named name.
func (r Registry) Lookup(name string) (Descriptor, bool) {
	op, found := r.lookup(name)
	if !found {
		return Descriptor{}, false
	}
	return describe(op), true
}

func (r Registry) lookup(name string) (Operation, bool) {
	i, found := slices.BinarySearchFunc(r.ops, name, func(op Operation, n string) int { return strings.Compare(op.Name, n) })
	if !found {
		return Operation{}, false
	}
	return r.ops[i], true
}

// objectSchema returns an error unless schema is a JSON object whose type is object, which MCP
// requires of a tool's schemas and which the API's request and response bodies share.
func objectSchema(schema json.RawMessage) error {
	var s struct {
		Type any `json:"type"`
	}
	if len(schema) == 0 {
		return errors.New("is missing")
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return fmt.Errorf("is not a JSON object: %w", err)
	}
	if s.Type != "object" {
		return errors.New(`does not have the type "object"`)
	}
	return nil
}

// compact returns schema without insignificant whitespace, so both roots and the contract document
// carry the same bytes. schema is valid JSON by the time it is called.
func compact(schema json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	if err := json.Compact(&b, schema); err != nil {
		return schema
	}
	return b.Bytes()
}
