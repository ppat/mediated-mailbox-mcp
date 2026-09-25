package fake_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/contract"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// sentinelText begins the identifier, the subject and the body of every message the scoped run's
// harness adds beside the suite's own, so a search of the run's output finds any of them that
// leaked.
const sentinelText = "mmsentinelmail"

// scopedChild names the variable under which the test runs the scoped suite itself, as the child
// process the test starts.
const scopedChild = "MEDIATED_MAILBOX_SCOPED_CONTRACT_CHILD"

// The suite run as a real provider's run is, scoped and over a mailbox holding mail it did not add,
// passes, compares and changes none of that mail, and prints none of it (ADR-0043). The run goes in
// a child process in verbose mode, so everything the suite logs or fails with is in its output, and
// the output is searched for the other mail's text.
func TestTheScopedSuitePassesOverMailItDidNotAdd(t *testing.T) {
	if os.Getenv(scopedChild) == "1" {
		contract.Run(t, contract.Config{Harness: sentinelHarness, Run: "fake", Scoped: true})
		return
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestTheScopedSuitePassesOverMailItDidNotAdd$", "-test.v", "-test.count=1") //nolint:gosec // the test binary running itself
	cmd.Env = append(os.Environ(), scopedChild+"=1")
	out, err := cmd.CombinedOutput()
	if n := bytes.Count(out, []byte(sentinelText)); n > 0 {
		t.Errorf("the scoped run's output holds the text of mail the suite did not add %d times", n)
	}
	if err != nil {
		t.Errorf("the scoped run failed: %v\n%s", err, out)
	}
}

// sentinelHarness builds a fake holding mail the suite did not add. It starts with a copy of the
// shared fixtures under sentinel text, and beside every message the suite delivers it delivers two
// more. One carries the delivered message's labels, the case's own label included, so a scoped
// listing returns it beside the case's messages. The other carries only the inbox, so only the
// scope keeps it out of a scoped listing. Each shares the delivered message's sender and date, so a
// query selecting the one selects the others. When the case ends, every one of them must be as it
// was delivered.
func sentinelHarness(t *testing.T, account string) contract.Implementation {
	var pre []fake.Message
	for _, m := range contract.Mailbox() {
		pre = append(pre, sentinel(sentinelText+"-pre-"+m.Metadata.ID, m, m.Metadata.Labels))
	}
	f := newFake(t, account, nil)
	delivered := map[string]mail.MessageMetadata{}
	add := func(m fake.Message) error {
		if err := f.Deliver(m); err != nil {
			return err
		}
		got, err := f.GetMessageMetadata(t.Context(), []string{m.Metadata.ID})
		if err != nil || len(got) != 1 {
			return fmt.Errorf("reading back a sentinel: %w", err)
		}
		delivered[m.Metadata.ID] = got[0]
		return nil
	}
	for _, m := range pre {
		if err := add(m); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		ids := slices.Sorted(func(yield func(string) bool) {
			for id := range delivered {
				if !yield(id) {
					return
				}
			}
		})
		// The case's context has ended by the time its cleanups run.
		got, err := f.GetMessageMetadata(context.Background(), ids)
		if err != nil {
			t.Errorf("reading the mail the suite did not add: %v", err)
			return
		}
		if len(got) != len(ids) {
			t.Errorf("%d of the %d messages the suite did not add are gone", len(ids)-len(got), len(ids))
		}
		for _, m := range got {
			if !cmp.Equal(delivered[m.ID], m, compare.Options) {
				t.Errorf("a message the suite did not add was changed")
			}
		}
	})
	n := 0
	return contract.Implementation{
		Port: f,
		Deliver: func(m contract.Message) (string, error) {
			if err := f.Deliver(fake.Message{Metadata: m.Metadata, Body: m.Body}); err != nil {
				return "", err
			}
			n++
			for i, labels := range [][]string{m.Metadata.Labels, {mail.Inbox}} {
				if err := add(sentinel(fmt.Sprintf("%s-%d-%d", sentinelText, n, i), m, labels)); err != nil {
					return "", err
				}
			}
			return m.Metadata.ID, nil
		},
		Remove: f.Remove,
	}
}

// sentinel returns a message under the identifier id, in a thread of its own, with m's sender, date
// and flags, the labels given, and sentinel text for its subject and body.
func sentinel(id string, m contract.Message, labels []string) fake.Message {
	md := m.Metadata
	md.ID, md.ThreadID = id, id
	md.Subject = id + " subject"
	md.Labels = slices.Clone(labels)
	return fake.Message{Metadata: md, Body: mail.MessageBody{Text: id + " body"}}
}
