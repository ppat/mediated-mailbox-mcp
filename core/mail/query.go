package mail

// Query is the canonical query, a tree each adapter compiles to its provider's own search, so no
// provider's syntax reaches anything above the port (ADR-0010). It is built only through the
// constructors below. The zero Query is not a query, and a port refuses it with ErrInvalid.
//
// Every node selects messages by a property both Gmail's search and JMAP's filter express exactly.
// A node that one backend cannot express would be a contract bug rather than an adapter branch.
type Query struct {
	kind     QueryKind
	instant  UnixMilli
	text     string
	operands []Query
}

// QueryKind names what a query node selects. The zero value is no query.
type QueryKind uint8

const (
	noQuery QueryKind = iota
	// QueryAll selects every message.
	QueryAll
	// QueryAfter selects the messages dated at or after an instant.
	QueryAfter
	// QueryBefore selects the messages dated before an instant.
	QueryBefore
	// QueryInLabel selects the messages carrying a label.
	QueryInLabel
	// QueryFrom selects the messages whose sender address equals an address, ignoring case.
	QueryFrom
	// QueryAnd selects the messages every operand selects.
	QueryAnd
)

// All selects every message.
func All() Query { return Query{kind: QueryAll} }

// After selects the messages dated at or after t.
func After(t UnixMilli) Query { return Query{kind: QueryAfter, instant: t} }

// Before selects the messages dated before t.
func Before(t UnixMilli) Query { return Query{kind: QueryBefore, instant: t} }

// InLabel selects the messages carrying the label at path.
func InLabel(path string) Query { return Query{kind: QueryInLabel, text: path} }

// From selects the messages whose sender address equals address, ignoring case.
func From(address string) Query { return Query{kind: QueryFrom, text: address} }

// And selects the messages every operand selects.
func And(first, second Query, more ...Query) Query {
	operands := append([]Query{first, second}, more...)
	return Query{kind: QueryAnd, operands: operands}
}

// Kind returns what the node selects.
func (q Query) Kind() QueryKind { return q.kind }

// Instant returns the instant an After or Before node compares against.
func (q Query) Instant() UnixMilli { return q.instant }

// Label returns the label path an InLabel node selects by.
func (q Query) Label() string {
	if q.kind != QueryInLabel {
		return ""
	}
	return q.text
}

// Address returns the sender address a From node selects by.
func (q Query) Address() string {
	if q.kind != QueryFrom {
		return ""
	}
	return q.text
}

// Operands returns a copy of an And node's operands.
func (q Query) Operands() []Query { return append([]Query(nil), q.operands...) }

// Valid reports whether q is a query a port accepts. A node no constructor built is not, and neither
// is a label or an address that is empty, or an And holding an operand that is not valid.
func (q Query) Valid() bool {
	switch q.kind {
	case QueryAll, QueryAfter, QueryBefore:
		return true
	case QueryInLabel, QueryFrom:
		return q.text != ""
	case QueryAnd:
		if len(q.operands) < 2 {
			return false
		}
		for _, o := range q.operands {
			if !o.Valid() {
				return false
			}
		}
		return true
	case noQuery:
		return false
	default:
		return false
	}
}
