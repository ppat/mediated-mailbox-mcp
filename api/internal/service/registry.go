package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/core/operation"
)

// Operation is one operation of the client surface, carrying everything both roots need (ADR-0053,
// ADR-0087). Its name is the MCP tool's name and the contract's operationId, and its path is the API
// route's. Its HTTP method and its MCP annotations are derived from its effect, so no field exists
// for one root alone and a one-sided operation has nothing to be written in.
type Operation struct {
	// Name identifies the operation on both roots. It is lowercase letters, digits and underscores,
	// starting with a letter, at most 64 characters, which is a valid MCP tool name.
	Name string
	// Effect is what the operation does to the mailbox, from which its method and annotations derive.
	Effect Effect
	// Path is the API route's path template, under /api/accounts/{account_id}/ for every operation
	// but the accounts listing, which is /api/accounts. Each {variable} is a required string argument.
	Path string
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

// Descriptor is what a root is generated from, an operation without its handler, with the method
// and annotations its effect derives and the facts the registry read from its path and input schema.
// A root runs an operation only through Registry.Call, so every call passes the account check.
type Descriptor struct {
	Name        string
	Description string
	Effect      Effect
	Path        string
	Method      string
	Annotations Annotations
	Input       json.RawMessage
	Output      json.RawMessage
	// PathVariables are the arguments the path names, in order.
	PathVariables []string
	// ArgumentTypes are the declared types of the input's properties, for each property whose type
	// is one string.
	ArgumentTypes map[string]string
}

// maxArguments is the most top-level keys a call's arguments may hold. No operation takes near this
// many, and the bound keeps the work of reading a call's keys fixed whatever a client sends.
const maxArguments = 1024

// Registry is the client surface's operation set, validated. Both roots are generated from it, and a
// root serves exactly its operations. The zero value holds no operation.
type Registry struct {
	ops         []Operation
	descriptors []Descriptor
	known       *atomic.Pointer[map[string]bool]
}

// NewRegistry validates ops and returns the registry holding them, or every problem found. An
// operation with no name, no description, no handler or an input or output that is not an object
// schema could be generated onto one root and not the other, so it fails generation here.
//
// Every operation but the accounts listing names its account in its path, so it takes account_id
// as a required string argument (ADR-0087). The generation checks of ADR-0087 refuse an operation
// the surface could not carry or that could express approval, as the operation core's Problems
// states, over the facts the registry reads from each operation once. The accounts parameter holds the accounts the API server serves, and Call checks every call's account
// against them. Both roots run an operation only through Call, so the check is one check for both.
func NewRegistry(accounts []string, ops ...Operation) (Registry, error) {
	all := make([]operation.Facts, 0, len(ops))
	for _, op := range ops {
		all = append(all, facts(op))
	}
	if problems := operation.Problems(all); len(problems) > 0 {
		return Registry{}, errors.New("the operation registry does not generate: " + strings.Join(problems, "; "))
	}
	byName := make([]int, len(ops))
	for i := range byName {
		byName[i] = i
	}
	slices.SortFunc(byName, func(a, b int) int { return strings.Compare(ops[a].Name, ops[b].Name) })
	sorted := make([]Operation, 0, len(ops))
	descriptors := make([]Descriptor, 0, len(ops))
	for _, i := range byName {
		op := ops[i]
		op.Input = compact(op.Input)
		op.Output = compact(op.Output)
		sorted = append(sorted, op)
		descriptors = append(descriptors, describe(op, all[i].Input))
	}
	r := Registry{ops: sorted, descriptors: descriptors, known: &atomic.Pointer[map[string]bool]{}}
	r.Serve(accounts)
	return r, nil
}

// Serve replaces the accounts the registry serves, as each account snapshot the API server loads
// replaces the one before it (ADR-0090). A call checks its account against the set in force when it
// is made. Every copy of the registry shares the set.
func (r Registry) Serve(accounts []string) {
	known := map[string]bool{}
	for _, account := range accounts {
		known[account] = account != ""
	}
	r.known.Store(&known)
}

// serves reports whether the registry serves account. The zero Registry serves none.
func (r Registry) serves(account string) bool {
	if r.known == nil {
		return false
	}
	known := r.known.Load()
	return known != nil && (*known)[account]
}

// Call runs the operation named name on a call's JSON arguments. It first refuses arguments that are
// not one JSON object, that hold more than maxArguments keys, or that hold two top-level keys equal
// under case folding, so each argument has one value whichever way a reader matches keys. Every
// operation but the accounts listing then has its account checked. The call is refused before the operation runs when its arguments hold no
// account_id string, name the account more than once, or name an account the API server does not
// serve, and otherwise the operation gets the account and the rest of its arguments.
func (r Registry) Call(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
	op, found := r.lookup(name)
	if !found {
		return nil, ErrNoOperation
	}
	if err := distinctKeys(input); err != nil {
		return nil, err
	}
	if op.Path == accountsPath {
		return op.Handle(ctx, "", input)
	}
	account, rest, err := splitAccount(input)
	if err != nil {
		return nil, err
	}
	if !r.serves(account) {
		return nil, ErrUnknownAccount
	}
	return op.Handle(ctx, account, rest)
}

// distinctKeys refuses a JSON object holding two top-level keys equal under case folding, or more
// than maxArguments keys, and anything that is not one JSON object. Two keys naming the account are an
// ambiguous account. Each key is looked up by its folded form, so the work grows with the number of
// keys and not with its square.
func distinctKeys(input json.RawMessage) error {
	obj, ok := OpenObject(input)
	if !ok {
		return ErrNotAnObject
	}
	seen := make(map[string]bool)
	for obj.More() {
		if len(seen) == maxArguments {
			return ErrTooManyArguments
		}
		key, _, ok := obj.Next()
		if !ok {
			return ErrNotAnObject
		}
		folded := foldKey(key)
		if seen[folded] {
			if folded == foldKey("account_id") {
				return ErrAmbiguousAccount
			}
			return ErrRepeatedArgument
		}
		seen[folded] = true
	}
	return nil
}

// foldKey maps a key to one form shared by every key strings.EqualFold holds equal to it, by replacing
// each rune with the least rune of its case-folding orbit.
func foldKey(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		least := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			least = min(least, f)
		}
		b.WriteRune(least)
	}
	return b.String()
}

// splitAccount reads a JSON object's top-level account_id string and returns it with the object
// holding every other member, in order. Keys are compared exactly, so a key differing from
// account_id only in case is refused rather than read. Call has already refused an object naming the
// account twice, through distinctKeys.
func splitAccount(input json.RawMessage) (string, json.RawMessage, error) {
	obj, ok := OpenObject(input)
	if !ok {
		return "", nil, ErrMissingAccount
	}
	var account json.RawMessage
	var rest bytes.Buffer
	rest.WriteByte('{')
	for obj.More() {
		key, value, ok := obj.Next()
		if !ok {
			return "", nil, ErrMissingAccount
		}
		if strings.EqualFold(key, "account_id") {
			if key != "account_id" {
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

// Object reads one JSON object's members in order, keys as written, which is how every reader of a
// call's arguments, on either root or in this layer, walks an object. Each reader stops where it
// needs to and says a malformed object in its own words.
type Object struct {
	input []byte
	dec   *json.Decoder
}

// OpenObject returns a reader of the JSON object input begins, or false when input does not begin
// one.
func OpenObject(input []byte) (Object, bool) {
	dec := json.NewDecoder(bytes.NewReader(input))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return Object{}, false
	}
	return Object{input: input, dec: dec}, true
}

// More reports whether another member follows.
func (o Object) More() bool { return o.dec.More() }

// Next returns the next member's key and value, or false when the object is malformed there.
func (o Object) Next() (string, json.RawMessage, bool) {
	tok, err := o.dec.Token()
	if err != nil {
		return "", nil, false
	}
	key, ok := tok.(string)
	if !ok {
		return "", nil, false
	}
	var value json.RawMessage
	if err := o.dec.Decode(&value); err != nil {
		return "", nil, false
	}
	return key, value, true
}

// End reports whether, after its last member, the object closes and nothing but JSON's white space
// follows it.
func (o Object) End() bool {
	if tok, err := o.dec.Token(); err != nil || tok != json.Delim('}') {
		return false
	}
	return len(bytes.Trim(o.input[o.dec.InputOffset():], " \t\r\n")) == 0
}

// Operations returns the registry's operations without their handlers, sorted by name.
func (r Registry) Operations() []Descriptor {
	out := make([]Descriptor, 0, len(r.descriptors))
	for _, d := range r.descriptors {
		out = append(out, cloned(d))
	}
	return out
}

// describe returns op without its handler, with the method and annotations its effect derives and
// the facts read from its path and from input, its input schema as the registry read and checked it.
func describe(op Operation, input operation.Schema) Descriptor {
	method, annotations, _ := operation.Derive(op.Effect)
	types := map[string]string{}
	for name, t := range input.Types {
		if t != "" {
			types[name] = t
		}
	}
	return Descriptor{
		Name: op.Name, Description: op.Description, Effect: op.Effect, Path: op.Path,
		Method: method, Annotations: annotations, Input: op.Input, Output: op.Output,
		PathVariables: operation.PathVariables(op.Path), ArgumentTypes: types,
	}
}

// cloned returns d with its own copies of the lists and the map, so a root changing what it was
// handed changes no other root's.
func cloned(d Descriptor) Descriptor {
	d.PathVariables = slices.Clone(d.PathVariables)
	types := make(map[string]string, len(d.ArgumentTypes))
	for name, t := range d.ArgumentTypes {
		types[name] = t
	}
	d.ArgumentTypes = types
	return d
}

// Lookup returns the description of the operation named name.
func (r Registry) Lookup(name string) (Descriptor, bool) {
	i, found := r.index(name)
	if !found {
		return Descriptor{}, false
	}
	return cloned(r.descriptors[i]), true
}

func (r Registry) lookup(name string) (Operation, bool) {
	i, found := r.index(name)
	if !found {
		return Operation{}, false
	}
	return r.ops[i], true
}

// index returns the position of the operation named name among the registry's operations.
func (r Registry) index(name string) (int, bool) {
	return slices.BinarySearchFunc(r.ops, name, func(op Operation, n string) int { return strings.Compare(op.Name, n) })
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
