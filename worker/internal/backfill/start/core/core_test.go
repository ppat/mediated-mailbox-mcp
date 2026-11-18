package core_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	start "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/start/core"
)

var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

const account = "personal"

// listing returns the account's policy with one rule listing each of domains.
func listing(t *testing.T, domains ...string) classify.Policy {
	t.Helper()
	var rows []policy.Row
	for _, d := range domains {
		rows = append(rows, policy.Row{ID: "rule." + d, Class: policy.Restricted, DomainSuffixes: []string{d}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return classify.Prepare(s.For(account), lookups)
}

// skipped is a message the high-volume rule skips under the thresholds below, with a List-Id, a
// sender above the high-volume mark with no prior hit, and nothing else asking for a scan.
func skipped() index.Waiting {
	return index.Waiting{
		ID: "m1", From: "news@list.example", Domain: "list.example", ListID: true,
		SizeBytes: 50_000, SentAt: 0, SenderVolume: 10, SenderHits: 0,
	}
}

var thresholds = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 30_720, RecentAgeMillis: 86_400_000, LowVolume: 2, HighVolume: 5}

const now = mail.UnixMilli(10 * 86_400_000)

// A stored gate skip stands only while the gate, deciding it again under the thresholds given, decides
// it as the same skip. A skip the gate would now scan, every skip under thresholds that cannot decide
// and a skip whose sender the policy now restricts are overturned (ADR-0121, ADR-0093).
func TestOverturned(t *testing.T) {
	skip := func(id string, volume int64) index.Waiting {
		m := skipped()
		m.ID, m.SenderVolume = id, volume
		return m
	}
	hit, masked, bank := skip("c", 10), skip("d", 10), skip("e", 30)
	hit.SenderHits = 1
	masked.SubjectMasked = true
	bank.From, bank.Domain = "news@bank.example", "bank.example"
	skips := []index.Waiting{skip("a", 10), skip("b", 30), hit, masked, bank}
	widened := thresholds
	widened.HighVolume = 20
	cases := []struct {
		name       string
		policy     classify.Policy
		thresholds scangate.Config
		skips      []index.Waiting
		want       []string
	}{
		{"the thresholds the skips were made under", listing(t), thresholds, skips, []string{"c", "d"}},
		{"a widened high-volume mark", listing(t), widened, skips, []string{"a", "c", "d"}},
		{"thresholds that cannot decide", listing(t), scangate.Config{}, skips, []string{"a", "b", "c", "d", "e"}},
		{"a sender the policy now lists", listing(t, "bank.example"), thresholds, skips, []string{"c", "d", "e"}},
		{"no skips", listing(t), thresholds, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, start.Overturned(c.policy, c.thresholds, now, c.skips), compare.Options); diff != "" {
				t.Errorf("Overturned (-want +got):\n%s", diff)
			}
		})
	}
}
