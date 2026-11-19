package mail_test

import (
	"errors"
	"testing"
	"time"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
)

// counted wraps f so it counts the body calls it is asked in calls, and answers each body call after
// the first that the part is held apart again when again is set. It is the provider fake alone, with
// no method of the port's written here, so a change to the port's operations is a change to the fake.
func counted(f *fake.Fake, calls *int, again bool) *fake.Throttled {
	return fake.Throttle(f, func(c fake.Call) error {
		if c.Op.Operation != coremail.OpGetMessageBody {
			return nil
		}
		*calls++
		if again && *calls > 1 {
			return coremail.HeldApartError{Part: coremail.BodyPart("again")}
		}
		return nil
	}, time.Now)
}

// The second call for a part held apart has one home, Body, which every caller fetches a body through. A
// body the provider returns whole takes one call naming no part, one whose rendered part it holds
// apart takes a second call naming that part, and a part named that answers it is held apart again
// takes no third (ADR-0131). The fake refuses a call naming a part it did not issue for the message,
// so a body that comes back whole was asked for with no part first and with the part the fake named
// second.
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
		var calls int
		body, err := mail.Body(t.Context(), counted(f, &calls, false), c.id)
		if err != nil || body.HTML != c.html {
			t.Errorf("the body of %s is %+v, %v, want its HTML part", c.id, body, err)
		}
		if calls != c.calls {
			t.Errorf("the body of %s took %d calls, want %d", c.id, calls, c.calls)
		}
	}
	var calls int
	if _, err := mail.Body(t.Context(), counted(f, &calls, true), "apart"); !errors.Is(err, coremail.ErrHeldApart) {
		t.Errorf("a part held apart again returned %v, want it held apart", err)
	}
	if calls != 2 {
		t.Errorf("a part held apart again took %d calls, want 2", calls)
	}
}
