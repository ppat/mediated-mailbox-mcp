package gmail

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// search is a canonical query compiled for Gmail's thread listing. Text is the search text and
// LabelIDs the label identifiers every selected message carries. None is a query naming a label the
// account lacks, which selects nothing, so no listing is sent.
//
// The search selects a superset of what the canonical query selects, and matches decides exactly.
type search struct {
	Text     string
	LabelIDs []string
	None     bool
}

// compileQuery compiles a valid query, resolving label paths through labels.
func compileQuery(q mail.Query, labels labelTable) (search, error) {
	var s search
	var terms []string
	if err := compileNode(q, labels, &s, &terms); err != nil {
		return search{}, err
	}
	s.Text = strings.Join(terms, " ")
	return s, nil
}

func compileNode(q mail.Query, labels labelTable, s *search, terms *[]string) error {
	switch q.Kind() {
	case mail.QueryAll:
	case mail.QueryAfter:
		*terms = append(*terms, "after:"+strconv.FormatInt(floorSeconds(q.Instant())-1, 10))
	case mail.QueryBefore:
		*terms = append(*terms, "before:"+strconv.FormatInt(ceilSeconds(q.Instant())+1, 10))
	case mail.QueryInLabel:
		id, ok := labels.idByPath[q.Label()]
		if !ok {
			s.None = true
			return nil
		}
		if !slices.Contains(s.LabelIDs, id) {
			s.LabelIDs = append(s.LabelIDs, id)
		}
	case mail.QueryFrom:
		if quotable(q.Address()) {
			*terms = append(*terms, `from:"`+q.Address()+`"`)
		}
	case mail.QueryAnd:
		for _, o := range q.Operands() {
			if err := compileNode(o, labels, s, terms); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("gmail: query node %d: %w", q.Kind(), mail.ErrInvalid)
	}
	return nil
}

// quotable reports whether an address can be written between double quotes in Gmail's search text
// without changing the query's meaning. Only the characters of an ordinary address qualify, so a
// quote, a brace, a parenthesis or a space, which could end the quoted text or start an operator,
// keep the address out of the search text.
func quotable(address string) bool {
	if address == "" {
		return false
	}
	for _, c := range address {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune(".@_+-%", c):
		default:
			return false
		}
	}
	return true
}

// floorSeconds and ceilSeconds round an instant down and up to whole seconds, negative instants
// included.
func floorSeconds(t mail.UnixMilli) int64 {
	s := int64(t) / 1000
	if int64(t)%1000 < 0 {
		s--
	}
	return s
}

func ceilSeconds(t mail.UnixMilli) int64 {
	s := int64(t) / 1000
	if int64(t)%1000 > 0 {
		s++
	}
	return s
}

// matches reports whether the query selects the message, exactly as the port defines each node.
func matches(q mail.Query, m mail.MessageMetadata) bool {
	switch q.Kind() {
	case mail.QueryAll:
		return true
	case mail.QueryAfter:
		return m.Date >= q.Instant()
	case mail.QueryBefore:
		return m.Date < q.Instant()
	case mail.QueryInLabel:
		return slices.Contains(m.Labels, q.Label())
	case mail.QueryFrom:
		return strings.EqualFold(m.From.Email, q.Address())
	case mail.QueryAnd:
		for _, o := range q.Operands() {
			if !matches(o, m) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// selected returns the threads holding at least one message the query selects, in their order.
func selected(q mail.Query, threads []mail.ThreadMetadata) []mail.ThreadMetadata {
	var out []mail.ThreadMetadata
	for _, th := range threads {
		if slices.ContainsFunc(th.Messages, func(m mail.MessageMetadata) bool { return matches(q, m) }) {
			out = append(out, th)
		}
	}
	return out
}
