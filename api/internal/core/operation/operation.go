// Package operation holds what each operation of the client surface may be, as values (ADR-0040).
// An operation declares one effect, and its HTTP method and its four MCP annotations are derived
// from it by ADR-0087's table, never declared per operation. The generation checks refuse an
// operation the surface could not carry on both roots, or one that could express a plan's approval
// (ADR-0053, ADR-0087).
//
// The checks read facts, never a schema's JSON. The service layer reads each operation's schemas
// once into a Schema and hands the checks the Facts of every operation, so the decision of what may
// be generated sits here and the reading of JSON stays in the shell.
package operation

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Effect is the one declaration an operation makes about what it does to the mailbox. Its HTTP
// method and its four MCP annotations are derived from it and never declared per operation
// (ADR-0087). The zero value is no effect, and an operation declaring it fails generation.
type Effect uint8

const (
	// Read is a read whose arguments are all scalars.
	Read Effect = iota + 1
	// StructuredRead is a read that takes structured input, as search takes the index query.
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

// Derive returns the HTTP method and the annotations ADR-0087's table gives an effect, or false for
// an effect the table has no row for.
func Derive(e Effect) (string, Annotations, bool) {
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

// AccountsPath is the accounts listing's path, and the one path that takes no account. Every other
// operation's path lies under AccountPrefix (ADR-0087).
const (
	AccountsPath  = "/api/accounts"
	AccountPrefix = AccountsPath + "/{account_id}/"
)

// literalSegment returns the shape of a path segment naming a collection or a sub-resource,
// optionally followed by a :verb on the collection.
func literalSegment() *regexp.Regexp {
	return regexp.MustCompile(`^([a-z][a-z0-9-]*)(?::([a-z][a-z0-9-]*))?$`)
}

// wildcardSegment returns the shape of a path segment standing for one argument.
func wildcardSegment() *regexp.Regexp {
	return regexp.MustCompile(`^\{([a-z][a-z0-9_]*)\}$`)
}

// namesLifecycle reports whether text names a plan's lifecycle, by a token between separators that
// begins with a lifecycle stem, or by roll_back or roll-back anywhere in it. The stems begin the
// tokens that could name a plan's lifecycle transition, refused in any operation name, path segment
// or :verb so approval cannot be expressed (ADR-0087).
func namesLifecycle(text, separator string) bool {
	if strings.Contains(text, "roll_back") || strings.Contains(text, "roll-back") {
		return true
	}
	for _, token := range strings.Split(text, separator) {
		for _, stem := range []string{"approv", "appl", "rollback"} {
			if strings.HasPrefix(token, stem) {
				return true
			}
		}
	}
	return false
}

// PathVariables returns the arguments a path template names, in order.
func PathVariables(path string) []string {
	wildcard := wildcardSegment()
	var vars []string
	for _, seg := range strings.Split(path, "/") {
		if m := wildcard.FindStringSubmatch(seg); m != nil {
			vars = append(vars, m[1])
		}
	}
	return vars
}

// Schema is what the generation checks read of an operation's input or output schema, which the
// shell reads out of its JSON.
type Schema struct {
	// Problem says why the schema is not an object schema, and is empty for one that is.
	Problem string
	// Unreadable marks an object schema whose properties or required arguments cannot be read.
	Unreadable bool
	// Types holds each declared property's type, or the empty string for a property whose type is
	// not one string.
	Types map[string]string
	// Required are the required properties.
	Required []string
	// Status marks a schema holding a key spelled status in any case, at any depth.
	Status bool
	// MCPHeader marks a schema holding a key spelled x-mcp-header in any case, at any depth.
	MCPHeader bool
}

// Facts are what the generation checks read of one operation.
type Facts struct {
	Name        string
	Description string
	Effect      Effect
	Path        string
	Input       Schema
	Output      Schema
	// Handled marks an operation that has a handler.
	Handled bool
}

// Problems returns every reason the operations ops cannot be generated onto both roots, each naming
// its operation by position and name, or none. An operation with a name no tool can carry, a name
// another holds, no description, no handler, or an input or output that is not an object schema
// could be generated onto one root and not the other. The checks of ADR-0087 refuse an operation the
// surface could not carry or that could express approval, as surface states, and two operations on
// one route.
func Problems(ops []Facts) []string {
	var problems []string
	validName := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	seen := map[string]bool{}
	routes := map[string]bool{}
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
		if op.Input.Problem != "" {
			problems = append(problems, where+" has an input schema that "+op.Input.Problem)
		} else {
			for _, problem := range surface(op) {
				problems = append(problems, where+" "+problem)
			}
		}
		if method, _, ok := Derive(op.Effect); ok {
			route := method + " " + op.Path
			if routes[route] {
				problems = append(problems, where+" repeats the route "+route)
			}
			routes[route] = true
		}
		if op.Output.Problem != "" {
			problems = append(problems, where+" has an output schema that "+op.Output.Problem)
		}
		if !op.Handled {
			problems = append(problems, where+" has no handler")
		}
	}
	return problems
}

// surface returns why op cannot be generated onto the surface ADR-0087 describes.
func surface(op Facts) []string {
	var problems []string
	method, _, derived := Derive(op.Effect)
	switch {
	case !derived:
		problems = append(problems, "declares no effect the derivation table has a row for")
	case method != methodGet && method != methodPost:
		problems = append(problems, "derives the method "+method+", and the surface derives only GET and POST")
	}
	if op.Input.Unreadable {
		return append(problems, "has an input schema whose properties cannot be read")
	}

	listing := op.Path == AccountsPath
	switch {
	case listing && op.Effect != Read:
		problems = append(problems, "takes the accounts listing's path without being a read")
	case !listing && !strings.HasPrefix(op.Path, AccountPrefix):
		problems = append(problems, "has a path not under "+AccountPrefix+", so it names no account in its path")
	}
	literal, wildcard := literalSegment(), wildcardSegment()
	segments := strings.Split(strings.TrimPrefix(op.Path, "/"), "/")
	seenVar := map[string]bool{}
	for i, seg := range segments {
		if m := wildcard.FindStringSubmatch(seg); m != nil {
			if seenVar[m[1]] {
				problems = append(problems, "names the path variable "+m[1]+" twice")
			}
			seenVar[m[1]] = true
			continue
		}
		m := literal.FindStringSubmatch(seg)
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

	for _, v := range PathVariables(op.Path) {
		t, ok := op.Input.Types[v]
		if !ok || t != "string" || !slices.Contains(op.Input.Required, v) {
			problems = append(problems, "has the path variable "+v+", which is not a required string argument")
		}
	}
	if op.Effect == Read {
		for name, t := range op.Input.Types {
			if !scalar(t) {
				problems = append(problems, "is a GET whose argument "+name+" is not a scalar")
			}
		}
	}
	if op.Effect != Read && op.Effect != StructuredRead && op.Input.Status {
		problems = append(problems, "changes the mailbox and declares a property named status in its input")
	}
	if op.Input.MCPHeader || op.Output.MCPHeader {
		problems = append(problems, "has a schema carrying x-mcp-header")
	}
	return problems
}

// scalar reports whether t is a JSON Schema type a query-string argument may declare.
func scalar(t string) bool {
	switch t {
	case "string", "integer", "number", "boolean":
		return true
	default:
		return false
	}
}

// shapeProblem returns why the last path segment does not take the route shape ADR-0087's table
// gives the effect, or the empty string. A structured read posts to a :search route, a reversible
// change and a disposal to another :verb route, and a read and a create to a collection or
// resource.
func shapeProblem(e Effect, last string) string {
	m := literalSegment().FindStringSubmatch(last)
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
