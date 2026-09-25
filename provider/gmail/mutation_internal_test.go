package gmail

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Each verb is the label change its canonical definition names, in Gmail's identifiers.
func TestOpChange(t *testing.T) {
	op := must[mail.MutationOp](t)
	cases := []struct {
		name string
		op   mail.MutationOp
		want labelChange
	}{
		{"label", op(mail.LabelOp("m1", "reading/digest")), labelChange{Add: []string{"Label_13"}}},
		{"label into the inbox", op(mail.LabelOp("m1", mail.Inbox)), labelChange{Add: []string{"INBOX"}}},
		{"unlabel", op(mail.UnlabelOp("m1", "finance")), labelChange{Remove: []string{"Label_12"}}},
		{"move", op(mail.MoveOp("m1", "finance", "reading")), labelChange{Add: []string{"Label_14"}, Remove: []string{"Label_12"}}},
		{"archive", op(mail.ArchiveOp("m1")), labelChange{Remove: []string{"INBOX"}}},
		{"mark read", op(mail.MarkReadOp("m1")), labelChange{Remove: []string{"UNREAD"}}},
		{"star", op(mail.StarOp("m1")), labelChange{Add: []string{"STARRED"}}},
		{"trash", op(mail.TrashOp("m1")), labelChange{Add: []string{"TRASH"}, Remove: []string{"INBOX"}}},
		{"spam", op(mail.SpamOp("m1")), labelChange{Add: []string{"SPAM"}, Remove: []string{"INBOX"}}},
		{"unlabel a label the account lacks", op(mail.UnlabelOp("m1", "missing")), labelChange{}},
		{"unlabel a label the model leaves out", op(mail.UnlabelOp("m1", "IMPORTANT")), labelChange{}},
		{"move from a label the account lacks", op(mail.MoveOp("m1", "missing", "finance")), labelChange{Add: []string{"Label_12"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := opChange(c.op, accountLabels(t))
			if err != nil {
				t.Fatalf("opChange: %v", err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("opChange (-want +got):\n%s", diff)
			}
		})
	}
}

// An op adding a label the account lacks, or one the model leaves out, is not found, and no request
// is built. Only a label an op adds must exist (ADR-0010).
func TestOpChangeRefusesAddingALabelTheAccountLacks(t *testing.T) {
	op := must[mail.MutationOp](t)
	for name, o := range map[string]mail.MutationOp{
		"label":           op(mail.LabelOp("m1", "missing")),
		"label left out":  op(mail.LabelOp("m1", "IMPORTANT")),
		"move to missing": op(mail.MoveOp("m1", "finance", "missing")),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := opChange(o, accountLabels(t)); !errors.Is(err, mail.ErrNotFound) {
				t.Errorf("opChange returned %v, want an error wrapping %v", err, mail.ErrNotFound)
			}
		})
	}
	if _, err := opChange(mail.MutationOp{}, accountLabels(t)); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("opChange of the zero op returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
}

// An op left with nothing to change still reads its message, so an op on a message the account
// does not have is not found rather than reported applied. The read is the one request the op
// sends, and it fails here once counted, since there is no stand-in for Google (ADR-0043).
func TestAnOpChangingNothingReadsItsMessage(t *testing.T) {
	a, reg, ctx := testAdapter(t)
	op := must[mail.MutationOp](t)(mail.UnlabelOp("18c2f0a1b2c3d4e5", "missing"))
	if err := a.begin(mail.OpMutate, 1).apply(ctx, op, accountLabels(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("apply returned %v, want the read of the message to fail on the cancelled context", err)
	}
	want := []counted{{Account: "you@example.com", Provider: "gmail", Value: unitsMessagesGet}}
	if diff := cmp.Diff(want, gatheredCost(t, reg), compare.Options); diff != "" {
		t.Errorf("request cost counted (-a read of the message +got):\n%s", diff)
	}
}

func TestStopsBatch(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"applied":     {nil, false},
		"not found":   {fmt.Errorf("x: %w", mail.ErrNotFound), false},
		"invalid":     {mail.ErrInvalid, false},
		"provider":    {mail.ErrProvider, false},
		"throttled":   {parseError(429, "", errorTime, nil), true},
		"credential":  {parseError(401, "", errorTime, nil), true},
		"cancelled":   {fmt.Errorf("x: %w", context.Canceled), true},
		"out of time": {context.DeadlineExceeded, true},
	} {
		if got := stopsBatch(c.err); got != c.want {
			t.Errorf("stopsBatch(%s) = %v, want %v", name, got, c.want)
		}
	}
}
