package gmail

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// 2024-09-01T09:00:00.250Z, an instant between two whole seconds.
const between = mail.UnixMilli(1725181200250)

func TestCompileQuery(t *testing.T) {
	labels := accountLabels(t)
	cases := []struct {
		name string
		q    mail.Query
		want search
	}{
		{"every message", mail.All(), search{}},
		{"after an instant between seconds", mail.After(between), search{Text: "after:1725181199"}},
		{"after a whole second", mail.After(1725181200000), search{Text: "after:1725181199"}},
		{"before an instant between seconds", mail.Before(between), search{Text: "before:1725181202"}},
		{"before a whole second", mail.Before(1725181200000), search{Text: "before:1725181201"}},
		{"before an instant before the epoch", mail.Before(-1500), search{Text: "before:0"}},
		{"after an instant before the epoch", mail.After(-1500), search{Text: "after:-3"}},
		{"a sender", mail.From("Bank@Example.com"), search{Text: `from:"Bank@Example.com"`}},
		{"a user label", mail.InLabel("reading/digest"), search{LabelIDs: []string{"Label_13"}}},
		{"a system label", mail.InLabel(mail.Trash), search{LabelIDs: []string{"TRASH"}}},
		{"a label the account lacks", mail.InLabel("missing"), search{None: true}},
		{"a label the model leaves out", mail.InLabel("IMPORTANT"), search{None: true}},
		{
			"every node together",
			mail.And(mail.InLabel("finance"), mail.From("bank@example.com"), mail.And(mail.After(between), mail.InLabel(mail.Inbox), mail.InLabel("finance"))),
			search{Text: `from:"bank@example.com" after:1725181199`, LabelIDs: []string{"Label_12", "INBOX"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := compileQuery(c.q, labels)
			if err != nil {
				t.Fatalf("compileQuery: %v", err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("compileQuery (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompileQueryRefusesANodeNoConstructorBuilt(t *testing.T) {
	if _, err := compileQuery(mail.Query{}, accountLabels(t)); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("compileQuery of the zero query returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
}

// A value in a canonical query cannot add an operator to Gmail's search. An address that could end
// its quotes stays out of the search text, and a label never enters it, so the listing is a
// superset that the exact match narrows (ADR-0010).
func TestQueryValuesCannotInjectOperators(t *testing.T) {
	labels := newLabelTable([]gmailLabel{{ID: "Label_40", Name: `finance OR in:anywhere "x" {y} -z`, Type: "user"}})
	cases := []struct {
		name string
		q    mail.Query
		want search
	}{
		{"an address closing its quotes", mail.From(`a@example.com" OR from:"b@example.com`), search{}},
		{"an address whose only unusual character is a quote", mail.From(`a"OR"@example.com`), search{}},
		{"an address with a brace", mail.From(`{a@example.com b@example.com}`), search{}},
		{"an address with a parenthesis", mail.From(`a@example.com)`), search{}},
		{"an address with a space", mail.From(`a@example.com OR b`), search{}},
		{"an address with a backslash", mail.From(`a\"@example.com`), search{}},
		{"an address with a line break", mail.From("a@example.com\nlabel:x"), search{}},
		{"a label holding operators", mail.InLabel(`finance OR in:anywhere "x" {y} -z`), search{LabelIDs: []string{"Label_40"}}},
		{
			"an injected address beside a plain one",
			mail.And(mail.From(`a@example.com" OR "`), mail.From("b@example.com")),
			search{Text: `from:"b@example.com"`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := compileQuery(c.q, labels)
			if err != nil {
				t.Fatalf("compileQuery: %v", err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("compileQuery (-want +got):\n%s", diff)
			}
		})
	}
}

// The listing Gmail returns is a superset, so each thread is kept only when one of its messages
// matches the query exactly on the mapped metadata.
func TestSelectedKeepsThreadsWithAnExactMatch(t *testing.T) {
	msg := func(thread, id string, date mail.UnixMilli, from string, labels ...string) mail.MessageMetadata {
		return mail.MessageMetadata{ID: id, ThreadID: thread, Date: date, From: mail.Address{Email: from}, Labels: labels}
	}
	all := []mail.ThreadMetadata{
		{ID: "t1", Messages: []mail.MessageMetadata{msg("t1", "m1", between-1, "bank@example.com", "finance"), msg("t1", "m2", between, "you@example.com", mail.Inbox)}},
		{ID: "t2", Messages: []mail.MessageMetadata{msg("t2", "m3", between+1, "Bank@Example.COM", mail.Inbox)}},
		{ID: "t3", Messages: []mail.MessageMetadata{msg("t3", "m4", between-1000, "news@example.com", "finance")}},
	}
	cases := []struct {
		name string
		q    mail.Query
		want []string
	}{
		{"after keeps the instant itself", mail.After(between), []string{"t1", "t2"}},
		{"before leaves the instant out", mail.Before(between), []string{"t1", "t3"}},
		{"a sender ignoring case", mail.From("bank@example.com"), []string{"t1", "t2"}},
		{"a label", mail.InLabel("finance"), []string{"t1", "t3"}},
		{"both on one message", mail.And(mail.InLabel("finance"), mail.After(between)), nil},
		{"both on one message, matched", mail.And(mail.InLabel(mail.Inbox), mail.From("bank@example.com")), []string{"t2"}},
		{"every thread", mail.All(), []string{"t1", "t2", "t3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, th := range selected(c.q, all) {
				got = append(got, th.ID)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("selected threads (-want +got):\n%s", diff)
			}
		})
	}
}
