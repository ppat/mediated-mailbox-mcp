package mail_test

import (
	"context"
	"errors"
	"testing"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
)

// counting is a mailbox that counts the body calls it is asked and the parts they name.
type counting struct {
	*fake.Fake
	parts []coremail.BodyPart
}

func (c *counting) GetMessageBody(ctx context.Context, id string, part coremail.BodyPart) (coremail.MessageBody, error) {
	c.parts = append(c.parts, part)
	return c.Fake.GetMessageBody(ctx, id, part)
}

// apartAgain answers every body call that its part is held apart, the part named included.
type apartAgain struct{ counting }

func (a *apartAgain) GetMessageBody(_ context.Context, id string, part coremail.BodyPart) (coremail.MessageBody, error) {
	a.parts = append(a.parts, part)
	return coremail.MessageBody{}, coremail.HeldApartError{Part: coremail.BodyPart("again-" + id)}
}

// The second call for a part held apart has one home, Body, which every caller fetches a body through. A
// body the provider returns whole takes one call naming no part, one whose rendered part it holds
// apart takes a second call naming that part, and a part named that answers it is held apart again
// takes no third (ADR-0131).
func TestABodyTakesOneCallOrOneMoreForAPartHeldApart(t *testing.T) {
	f, err := fake.New(fake.Config{Account: "personal"},
		fake.Message{Metadata: coremail.MessageMetadata{ID: "inline", ThreadID: "t1"}, Body: coremail.MessageBody{HTML: "<p>inline</p>"}},
		fake.Message{Metadata: coremail.MessageMetadata{ID: "apart", ThreadID: "t2"}, Body: coremail.MessageBody{HTML: "<p>apart</p>"}, HeldApart: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id, html string
		calls    int
	}{{"inline", "<p>inline</p>", 1}, {"apart", "<p>apart</p>", 2}} {
		m := &counting{Fake: f}
		body, err := mail.Body(t.Context(), m, c.id)
		if err != nil || body.HTML != c.html {
			t.Errorf("the body of %s is %+v, %v, want its HTML part", c.id, body, err)
		}
		if len(m.parts) != c.calls || m.parts[0] != "" {
			t.Errorf("the body of %s took the calls naming %q, want %d, the first naming no part", c.id, m.parts, c.calls)
		}
	}
	again := &apartAgain{counting{Fake: f}}
	if _, err := mail.Body(t.Context(), again, "apart"); !errors.Is(err, coremail.ErrHeldApart) {
		t.Errorf("a part held apart again returned %v, want it held apart", err)
	}
	if want := []coremail.BodyPart{"", "again-apart"}; len(again.parts) != 2 || again.parts[0] != want[0] || again.parts[1] != want[1] {
		t.Errorf("a part held apart again took the calls naming %q, want %q", again.parts, want)
	}
}
