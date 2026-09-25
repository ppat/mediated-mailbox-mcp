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

// harness builds a fake for each case, with pages of two, offered behind the port wrap returns,
// delivering and removing mail through the fake's own test controls. The fake keeps the key a
// message is delivered under as its identifier.
func harness(wrap func(*fake.Fake) mail.Port[context.Context]) contract.Harness {
	return sized(2, wrap)
}

// sized is harness with pages of pageSize.
func sized(pageSize int, wrap func(*fake.Fake) mail.Port[context.Context]) contract.Harness {
	return func(t *testing.T, account string) contract.Implementation {
		f, err := fake.New(fake.Config{Account: account, PageSize: pageSize})
		if err != nil {
			t.Fatal(err)
		}
		return contract.Implementation{
			Port: wrap(f),
			Deliver: func(m contract.Message) (string, error) {
				return m.Metadata.ID, f.Deliver(fake.Message{Metadata: m.Metadata, Body: m.Body})
			},
			Remove: f.Remove,
		}
	}
}

func TestTheFakePassesTheContract(t *testing.T) {
	contract.Run(t, contract.Config{
		Harness: harness(func(f *fake.Fake) mail.Port[context.Context] { return f }),
		Run:     "fake",
	})
}

// A fake behind a schedule that throttles nothing is the same implementation of the port.
func TestTheThrottledFakePassesTheContract(t *testing.T) {
	contract.Run(t, contract.Config{
		Harness: harness(func(f *fake.Fake) mail.Port[context.Context] { return fake.Throttle(f, fake.Never(), time.Now) }),
		Run:     "fake",
	})
}

// A fake whose every listing fits one page passes too, so the suite does not fail an implementation
// that issues no page token at all.
func TestTheFakePassesTheContractOnOnePage(t *testing.T) {
	contract.Run(t, contract.Config{
		Harness: sized(100, func(f *fake.Fake) mail.Port[context.Context] { return f }),
		Run:     "fake",
	})
}
