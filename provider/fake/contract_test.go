package fake_test

import (
	"context"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/contract"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
)

// seed returns the fake's messages for a mailbox the contract suite seeds.
func seed(mailbox []contract.Message) []fake.Message {
	out := make([]fake.Message, len(mailbox))
	for i, m := range mailbox {
		out[i] = fake.Message{Metadata: m.Metadata, Body: m.Body}
	}
	return out
}

// newFake builds a fake whose pages hold two items, so the suite pages through every listing.
func newFake(t *testing.T, account string, mailbox []contract.Message) *fake.Fake {
	t.Helper()
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, seed(mailbox)...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// implementation offers the fake behind port, delivering and removing mail through the fake's own
// test controls.
func implementation(f *fake.Fake, port mail.Port[context.Context]) contract.Implementation {
	return contract.Implementation{
		Port:    port,
		Deliver: func(m contract.Message) error { return f.Deliver(fake.Message{Metadata: m.Metadata, Body: m.Body}) },
		Remove:  f.Remove,
	}
}

func TestTheFakePassesTheContract(t *testing.T) {
	contract.Run(t, func(t *testing.T, account string, mailbox []contract.Message) contract.Implementation {
		f := newFake(t, account, mailbox)
		return implementation(f, f)
	})
}

// A fake behind a schedule that throttles nothing is the same implementation of the port.
func TestTheThrottledFakePassesTheContract(t *testing.T) {
	contract.Run(t, func(t *testing.T, account string, mailbox []contract.Message) contract.Implementation {
		f := newFake(t, account, mailbox)
		return implementation(f, fake.Throttle(f, fake.Never(), time.Now))
	})
}
