package mail_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

const pkg = "github.com/ppat/mediated-mailbox-mcp/core/mail"

// op is what a test can observe of a built op.
type op struct {
	Verb   string
	ID     string
	Add    string
	Remove string
	Built  bool
}

func observeOp(o mail.MutationOp) op {
	return op{Verb: o.Verb().String(), ID: o.MessageID(), Add: o.AddLabel(), Remove: o.RemoveLabel(), Built: o.Built()}
}

// Each constructor builds the op its verb names, with the labels it adds and removes.
func TestConstructorsBuildTheirVerb(t *testing.T) {
	type built struct {
		mail.MutationOp
		error
	}
	b := func(o mail.MutationOp, err error) built { return built{o, err} }
	cases := []struct {
		name string
		got  built
		want op
	}{
		{"label", b(mail.LabelOp("m", "Finance")), op{Verb: "label", ID: "m", Add: "Finance", Built: true}},
		{"label into the inbox", b(mail.LabelOp("m", mail.Inbox)), op{Verb: "label", ID: "m", Add: mail.Inbox, Built: true}},
		{"unlabel", b(mail.UnlabelOp("m", "Finance")), op{Verb: "unlabel", ID: "m", Remove: "Finance", Built: true}},
		{"unlabel out of the trash", b(mail.UnlabelOp("m", mail.Trash)), op{Verb: "unlabel", ID: "m", Remove: mail.Trash, Built: true}},
		{"unlabel out of spam", b(mail.UnlabelOp("m", mail.Spam)), op{Verb: "unlabel", ID: "m", Remove: mail.Spam, Built: true}},
		{"move", b(mail.MoveOp("m", "Finance", "Finance/2024")), op{Verb: "move", ID: "m", Add: "Finance/2024", Remove: "Finance", Built: true}},
		{"label into the trash", b(mail.LabelOp("m", mail.Trash)), op{Verb: "label", ID: "m", Add: mail.Trash, Built: true}},
		{"unlabel the inbox", b(mail.UnlabelOp("m", mail.Inbox)), op{Verb: "unlabel", ID: "m", Remove: mail.Inbox, Built: true}},
		{"move into spam", b(mail.MoveOp("m", "Finance", mail.Spam)), op{Verb: "move", ID: "m", Add: mail.Spam, Remove: "Finance", Built: true}},
		{"move out of the inbox, filing the message", b(mail.MoveOp("m", mail.Inbox, "Finance")), op{Verb: "move", ID: "m", Add: "Finance", Remove: mail.Inbox, Built: true}},
		{"move out of the trash into the inbox", b(mail.MoveOp("m", mail.Trash, mail.Inbox)), op{Verb: "move", ID: "m", Add: mail.Inbox, Remove: mail.Trash, Built: true}},
		{"archive", b(mail.ArchiveOp("m")), op{Verb: "archive", ID: "m", Built: true}},
		{"mark read", b(mail.MarkReadOp("m")), op{Verb: "mark_read", ID: "m", Built: true}},
		{"star", b(mail.StarOp("m")), op{Verb: "star", ID: "m", Built: true}},
		{"trash", b(mail.TrashOp("m")), op{Verb: "trash", ID: "m", Built: true}},
		{"spam", b(mail.SpamOp("m")), op{Verb: "spam", ID: "m", Built: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got.error != nil {
				t.Fatalf("the constructor refused: %v", c.got.error)
			}
			if diff := cmp.Diff(c.want, observeOp(c.got.MutationOp), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// An op missing what it acts on is refused as malformed.
func TestAnIncompleteOpIsRefused(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"a label with no message", second(mail.LabelOp("", "Finance"))},
		{"a label with no label", second(mail.LabelOp("m", ""))},
		{"an unlabel with no label", second(mail.UnlabelOp("m", ""))},
		{"a move with no source", second(mail.MoveOp("m", "", "Finance"))},
		{"a move with no target", second(mail.MoveOp("m", "Finance", ""))},
		{"a move to where it starts", second(mail.MoveOp("m", "Finance", "Finance"))},
		{"an archive with no message", second(mail.ArchiveOp(""))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !errors.Is(c.err, mail.ErrInvalid) {
				t.Errorf("the constructor returned %v, want an error wrapping %v", c.err, mail.ErrInvalid)
			}
		})
	}
}

func second(_ mail.MutationOp, err error) error { return err }

// The zero op is no verb, which the authorizer refuses and every port refuses as not built.
func TestTheZeroOpIsNotBuilt(t *testing.T) {
	var zero mail.MutationOp
	if diff := cmp.Diff(op{Verb: "no verb"}, observeOp(zero), compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if authorize.Decide(zero.Verb(), sensitivity.NormalSender()).Authorized() {
		t.Error("the authorizer authorized the zero op's verb")
	}
}

// An op carries only its verb, its message and its labels, so nothing else can ride into a port
// through it, and no field is reachable except through a constructor.
func TestAnOpHoldsOnlyWhatItApplies(t *testing.T) {
	mustnotcompile.RequireFields(t, pkg, "MutationOp",
		"verb authorize.Verb", "messageID string", "add string", "remove string")
	mustnotcompile.RequireNoExportedFields(t, pkg, "MutationOp", "Query")
}

// query is what a test can observe of a query node.
type query struct {
	Kind     mail.QueryKind
	Instant  mail.UnixMilli
	Label    string
	Address  string
	Operands []query
	Valid    bool
}

func observeQuery(q mail.Query) query {
	out := query{Kind: q.Kind(), Instant: q.Instant(), Label: q.Label(), Address: q.Address(), Valid: q.Valid()}
	for _, o := range q.Operands() {
		out.Operands = append(out.Operands, observeQuery(o))
	}
	return out
}

func TestQueryNodes(t *testing.T) {
	cases := []struct {
		name string
		q    mail.Query
		want query
	}{
		{"all", mail.All(), query{Kind: mail.QueryAll, Valid: true}},
		{"after", mail.After(1000), query{Kind: mail.QueryAfter, Instant: 1000, Valid: true}},
		{"before", mail.Before(2000), query{Kind: mail.QueryBefore, Instant: 2000, Valid: true}},
		{"in a label", mail.InLabel("Finance"), query{Kind: mail.QueryInLabel, Label: "Finance", Valid: true}},
		{"from a sender", mail.From("a@b.example"), query{Kind: mail.QueryFrom, Address: "a@b.example", Valid: true}},
		{"and", mail.And(mail.After(1), mail.InLabel("L"), mail.From("f")), query{Kind: mail.QueryAnd, Valid: true, Operands: []query{
			{Kind: mail.QueryAfter, Instant: 1, Valid: true},
			{Kind: mail.QueryInLabel, Label: "L", Valid: true},
			{Kind: mail.QueryFrom, Address: "f", Valid: true},
		}}},
		{"the zero query", mail.Query{}, query{}},
		{"an empty label", mail.InLabel(""), query{Kind: mail.QueryInLabel}},
		{"an empty sender", mail.From(""), query{Kind: mail.QueryFrom}},
		{"an And holding an invalid operand", mail.And(mail.All(), mail.And(mail.All(), mail.Query{})), query{Kind: mail.QueryAnd, Operands: []query{
			{Kind: mail.QueryAll, Valid: true},
			{Kind: mail.QueryAnd, Operands: []query{{Kind: mail.QueryAll, Valid: true}, {}}},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observeQuery(c.q), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// Editing the operands a node returns leaves the node as it was.
func TestOperandsAreACopy(t *testing.T) {
	q := mail.And(mail.All(), mail.InLabel("L"))
	q.Operands()[0] = mail.Query{}
	if !q.Valid() {
		t.Error("editing the returned operands changed the query")
	}
}

// A throttle wraps ErrThrottled and carries its signal however deeply it is wrapped, and no other
// error is a throttle.
func TestThrottles(t *testing.T) {
	signal := mail.ThrottleSignal{RetryAfterMillis: 1500, HasRetryAfter: true, Scope: mail.ScopePerProject}
	type parsed struct {
		Signal    mail.ThrottleSignal
		Throttled bool
		Is        bool
		Text      string
	}
	observe := func(err error) parsed {
		sig, ok := mail.Throttled(err)
		p := parsed{Signal: sig, Throttled: ok, Is: errors.Is(err, mail.ErrThrottled)}
		if err != nil {
			p.Text = err.Error()
		}
		return p
	}
	cases := []struct {
		name string
		err  error
		want parsed
	}{
		{"a throttle", mail.ThrottleError{Signal: signal}, parsed{signal, true, true, "mail: the provider throttled the request, retry after 1500 ms"}},
		{"a joined throttle", errors.Join(errors.New("listing"), mail.ThrottleError{Signal: signal}), parsed{signal, true, true, "listing\nmail: the provider throttled the request, retry after 1500 ms"}},
		{"a throttle without a retry time", mail.ThrottleError{}, parsed{mail.ThrottleSignal{}, true, true, "mail: the provider throttled the request"}},
		{"the bare sentinel", mail.ErrThrottled, parsed{Is: true, Text: "mail: the provider throttled the request"}},
		{"another port error", mail.ErrProvider, parsed{Text: "mail: the provider failed the request"}},
		{"no error", nil, parsed{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(c.err), compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

// The port's errors are distinct, so a caller tells each apart with errors.Is.
func TestThePortsErrorsAreDistinct(t *testing.T) {
	all := []error{mail.ErrNotFound, mail.ErrInvalid, mail.ErrCursorGap, mail.ErrThrottled, mail.ErrProvider, mail.ErrAuthentication}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("%v is %v", a, b)
			}
		}
	}
}

// The body arrives only through the body fetch (ADR-0010). The port has exactly these operations with
// exactly these signatures, so an operation added to return the body in any form, a string
// included, fails. Of them, GetMessageBody alone returns a value through which a MessageBody can be
// reached, and it returns the body and an error and nothing else. The rate profile the port returns
// is pinned the same way.
func TestOnlyTheBodyFetchReturnsABody(t *testing.T) {
	mustnotcompile.RequireMethods(t, pkg, "Port",
		"ChangesSince func(ctx C, cursor Cursor) (ChangeSet, error)",
		"CurrentCursor func(ctx C) (Cursor, error)",
		"EnsureLabel func(ctx C, path string) (Label, error)",
		"EnumerateAll func(ctx C, page PageToken) (Page[MessageMetadata], error)",
		"GetMessageBody func(ctx C, id string) (MessageBody, error)",
		"GetMessageMetadata func(ctx C, ids []string) ([]MessageMetadata, error)",
		"GetThreadMetadata func(ctx C, threadID string) (ThreadMetadata, error)",
		"ListLabels func(ctx C) ([]Label, error)",
		"ListThreads func(ctx C, q Query, page PageToken) (Page[ThreadMetadata], error)",
		"Mutate func(ctx C, ops []MutationOp) (MutationResult, error)",
		"RateProfile func() RateLimitProfile[C]")
	mustnotcompile.RequireMethods(t, pkg, "RateLimitProfile",
		"BudgetPerSecond func() float64",
		"Cost func(op ProviderOp) OpCost",
		"ParseThrottle func(err error) (ThrottleSignal, bool)",
		"RefreshLimits func(ctx C) error")
	mustnotcompile.RequireReturning(t, pkg, "Port", "MessageBody", "GetMessageBody")
	mustnotcompile.RequireResults(t, pkg, "Port", "GetMessageBody", "MessageBody", "error")
}

// No metadata type has a field for the full body. Each type a metadata operation returns holds
// exactly these fields, so a field holding the body cannot be added unnoticed under any type, a
// string included. Two metadata fields are derived from the body, the snippet and the attachment
// names. ADR-0010 marks the snippet redactable, and the gate withholds both with the body
// (ADR-0001).
func TestMetadataTypesHaveNoBodyField(t *testing.T) {
	mustnotcompile.RequireFields(t, pkg, "MessageMetadata",
		"AccountID string", "ID string", "ThreadID string", "From Address", "To []Address", "Cc []Address",
		"Subject string", "Date UnixMilli", "Labels []string", "Flags Flags", "SizeBytes int64",
		"HasAttachments bool", "AttachmentNames []string", "Snippet string", "ListID string",
		"AuthResults AuthResults")
	mustnotcompile.RequireFields(t, pkg, "Address", "Email string", "Name string")
	mustnotcompile.RequireFields(t, pkg, "Flags", "Read bool", "Starred bool")
	mustnotcompile.RequireFields(t, pkg, "AuthResults", "SPF string", "DKIM string", "DMARC string")
	mustnotcompile.RequireFields(t, pkg, "ThreadMetadata", "AccountID string", "ID string", "Messages []MessageMetadata")
	mustnotcompile.RequireFields(t, pkg, "Page", "Items []T", "Next PageToken")
	mustnotcompile.RequireFields(t, pkg, "ChangeSet",
		"AccountID string", "Added []string", "Modified []string", "Removed []string", "Next Cursor")
	mustnotcompile.RequireFields(t, pkg, "Label", "AccountID string", "Path string")
}
