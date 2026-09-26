package service

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// Effect is the one declaration an operation makes about what it does to the mailbox. Its HTTP
// method and its four MCP annotations are derived from it and never declared per operation
// (ADR-0087). The zero value is no effect, and an operation declaring it fails generation.
type Effect uint8

const (
	// Read is a read whose arguments are all scalars.
	Read Effect = iota + 1
	// StructuredRead is a read that takes structured input, as search takes the canonical query.
	StructuredRead
	// Create adds a resource to a collection.
	Create
	// Reversible is a change the operator can undo, such as labelling or archiving.
	Reversible
	// Disposal is trash or spam.
	Disposal
)

// Annotations are an operation's four MCP annotations, each set explicitly.
type Annotations struct {
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	OpenWorld   bool
}

// The two HTTP methods the surface derives. Every other method is refused at generation.
const (
	methodGet  = "GET"
	methodPost = "POST"
)

// derive returns the HTTP method and the annotations ADR-0087's table gives an effect, or false for
// an effect the table has no row for.
func derive(e Effect) (string, Annotations, bool) {
	switch e {
	case Read:
		return methodGet, Annotations{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false}, true
	case StructuredRead:
		return methodPost, Annotations{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false}, true
	case Create:
		return methodPost, Annotations{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false}, true
	case Reversible:
		return methodPost, Annotations{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false}, true
	case Disposal:
		return methodPost, Annotations{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false}, true
	default:
		return "", Annotations{}, false
	}
}

// accountsPath is the accounts listing's path, and the one path that takes no account. Every other
// operation's path lies under accountPrefix (ADR-0087).
const (
	accountsPath  = "/api/accounts"
	accountPrefix = accountsPath + "/{account_id}/"
)

var (
	// literalSegment is a path segment naming a collection or a sub-resource, optionally followed by
	// a :verb on the collection.
	literalSegment = regexp.MustCompile(`^([a-z][a-z0-9-]*)(?::([a-z][a-z0-9-]*))?$`)
	// wildcardSegment is a path segment standing for one argument.
	wildcardSegment = regexp.MustCompile(`^\{([a-z][a-z0-9_]*)\}$`)
)

// lifecycleStems begin the tokens that could name a plan's lifecycle transition, refused in any
// operation name, path segment or :verb so approval cannot be expressed (ADR-0087).
var lifecycleStems = []string{"approv", "appl", "rollback"}

// namesLifecycle reports whether text names a plan's lifecycle, by a token between separators that
// begins with a lifecycle stem, or by roll_back or roll-back anywhere in it.
func namesLifecycle(text, separator string) bool {
	if strings.Contains(text, "roll_back") || strings.Contains(text, "roll-back") {
		return true
	}
	for _, token := range strings.Split(text, separator) {
		for _, stem := range lifecycleStems {
			if strings.HasPrefix(token, stem) {
				return true
			}
		}
	}
	return false
}

// pathVariables returns the arguments a path template names, in order.
func pathVariables(path string) []string {
	var vars []string
	for _, seg := range strings.Split(path, "/") {
		if m := wildcardSegment.FindStringSubmatch(seg); m != nil {
			vars = append(vars, m[1])
		}
	}
	return vars
}

// schemaFacts are what the generation checks read from an input schema.
type schemaFacts struct {
	Properties map[string]struct {
		Type any `json:"type"`
	} `json:"properties"`
	Required []string `json:"required"`
}

// scalarTypes are the JSON Schema types a query-string argument may declare.
var scalarTypes = []string{"string", "integer", "number", "boolean"}

// surfaceProblems returns why op cannot be generated onto the surface ADR-0087 describes.
func surfaceProblems(op Operation) []string {
	var problems []string
	method, _, derived := derive(op.Effect)
	switch {
	case !derived:
		problems = append(problems, "declares no effect the derivation table has a row for")
	case method != methodGet && method != methodPost:
		problems = append(problems, "derives the method "+method+", and the surface derives only GET and POST")
	}
	var facts schemaFacts
	if err := json.Unmarshal(op.Input, &facts); err != nil {
		return append(problems, "has an input schema whose properties cannot be read")
	}

	listing := op.Path == accountsPath
	switch {
	case listing && op.Effect != Read:
		problems = append(problems, "takes the accounts listing's path without being a read")
	case !listing && !strings.HasPrefix(op.Path, accountPrefix):
		problems = append(problems, "has a path not under "+accountPrefix+", so it names no account in its path")
	}
	segments := strings.Split(strings.TrimPrefix(op.Path, "/"), "/")
	seenVar := map[string]bool{}
	for i, seg := range segments {
		if m := wildcardSegment.FindStringSubmatch(seg); m != nil {
			if seenVar[m[1]] {
				problems = append(problems, "names the path variable "+m[1]+" twice")
			}
			seenVar[m[1]] = true
			continue
		}
		m := literalSegment.FindStringSubmatch(seg)
		if m == nil {
			problems = append(problems, "has the path segment "+seg+", which is neither a name, a name with a :verb nor a {variable}")
			continue
		}
		if namesLifecycle(m[1], "-") || namesLifecycle(m[2], "-") {
			problems = append(problems, "has the path segment "+seg+", which names a plan's lifecycle")
		}
		if m[2] != "" && i != len(segments)-1 {
			problems = append(problems, "has the :verb segment "+seg+" before the path's end")
		}
	}
	if namesLifecycle(op.Name, "_") {
		problems = append(problems, "has the name "+op.Name+", which names a plan's lifecycle")
	}
	if problem := shapeProblem(op.Effect, segments[len(segments)-1]); problem != "" {
		problems = append(problems, problem)
	}

	for _, v := range pathVariables(op.Path) {
		p, ok := facts.Properties[v]
		if !ok || p.Type != "string" || !slices.Contains(facts.Required, v) {
			problems = append(problems, "has the path variable "+v+", which is not a required string argument")
		}
	}
	if op.Effect == Read {
		for name, p := range facts.Properties {
			if t, ok := p.Type.(string); !ok || !slices.Contains(scalarTypes, t) {
				problems = append(problems, "is a GET whose argument "+name+" is not a scalar")
			}
		}
	}
	if op.Effect != Read && op.Effect != StructuredRead && holdsKey(op.Input, "status") {
		problems = append(problems, "changes the mailbox and declares a property named status in its input")
	}
	if holdsKey(op.Input, "x-mcp-header") || holdsKey(op.Output, "x-mcp-header") {
		problems = append(problems, "has a schema carrying x-mcp-header")
	}
	return problems
}

// shapeProblem returns why the last path segment does not take the route shape ADR-0087's table
// gives the effect, or the empty string. A structured read posts to a :search route, a reversible
// change and a disposal to another :verb route, and a read and a create to a collection or
// resource.
func shapeProblem(e Effect, last string) string {
	m := literalSegment.FindStringSubmatch(last)
	verb := ""
	if m != nil {
		verb = m[2]
	}
	switch e {
	case StructuredRead:
		if verb != "search" {
			return "is a structured read whose path does not end in a :search route"
		}
	case Reversible, Disposal:
		if verb == "" || verb == "search" {
			return "is a change whose path does not end in a :verb route other than :search"
		}
	case Read, Create:
		if verb != "" {
			return "is a read or a create whose path ends in a :verb route"
		}
	default:
	}
	return ""
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
