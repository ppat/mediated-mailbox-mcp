package index_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
)

var lookups = classify.Lookups{
	ToUnicode:   idna.Lookup.ToUnicode,
	ToASCII:     idna.Lookup.ToASCII,
	Registrable: publicsuffix.EffectiveTLDPlusOne,
}

const account = "personal"

// composed returns the account's policy with one rule listing bank.example.
func composed(t *testing.T) policy.Composed {
	t.Helper()
	return listing(t, "bank.example")
}

// listing returns the account's policy with one rule listing each of domains, named rule.bank for
// bank.example and rule.<domain> for any other.
func listing(t *testing.T, domains ...string) policy.Composed {
	t.Helper()
	var rows []policy.Row
	for _, d := range domains {
		id := "rule." + d
		if d == "bank.example" {
			id = "rule.bank"
		}
		rows = append(rows, policy.Row{ID: id, Class: policy.Restricted, DomainSuffixes: []string{d}})
	}
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return s.For(account)
}

func scanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// metadata returns a fixture as the metadata a page carries.
func metadata(id string, f fixture.Message, date mail.UnixMilli, labels ...string) mail.MessageMetadata {
	return mail.MessageMetadata{
		AccountID: account,
		ID:        id,
		ThreadID:  "t-" + id,
		From:      mail.Address{Email: f.FromAddress, Name: f.FromName},
		Subject:   f.Subject,
		Date:      date,
		Labels:    labels,
		ListID:    f.ListID,
		SizeBytes: 2048,
	}
}

// codeSubject is the one-time code fixture's subject as the index stores it, the code masked.
func codeSubject() string {
	return strings.TrimSuffix(fixture.OneTimeCode().Subject, "419283") + "██████"
}

// Each sender is classified under the account's policy, and each subject is masked, a restricted
// sender's included, and stamped with the scanner version and revision it was masked under (ADR-0003,
// ADR-0004, ADR-0120). A listed sender carries the rule that restricted it, and no other sender
// carries a rule (ADR-0016). An address whose domain cannot be read is restricted and
// marked unclassified. The domains are listed once each, sorted.
func TestDecide(t *testing.T) {
	bank := fixture.Bank()
	bankCode := bank
	bankCode.Subject = fixture.OneTimeCode().Subject
	undated := fixture.Newsletter()
	undated.FromAddress = "no-address-at-all"
	items := []mail.MessageMetadata{
		metadata("m1", fixture.OneTimeCode(), 1000, "INBOX"),
		metadata("m2", bankCode, 2000),
		metadata("m3", fixture.Newsletter(), 3000, "Newsletters", "INBOX"),
		metadata("m4", undated, 4000),
		metadata("m5", bank, 5000),
	}
	code := []index.Mask{{Rule: scan.RuleTriggerWindow, Tier: 1}}
	message := func(m mail.MessageMetadata, domain, subject string, class index.Class, rule string, masks []index.Mask) index.Message {
		return index.Message{
			ID: m.ID, ThreadID: m.ThreadID, From: m.From, Domain: domain, Subject: subject,
			SubjectMasked: len(masks) > 0, Date: m.Date, Labels: m.Labels, ListID: m.ListID,
			SizeBytes: m.SizeBytes, Class: class, ClassRule: rule, Unclassified: m.From.Email == "no-address-at-all", Masks: masks,
			Stamp: index.Stamp{Version: 1, Revision: "a-revision"},
		}
	}
	want := index.Page{
		Messages: []index.Message{
			message(items[0], "security.example", codeSubject(), index.Normal, "", code),
			message(items[1], "bank.example", codeSubject(), index.Restricted, "rule.bank", code),
			message(items[2], "newsletter.example", items[2].Subject, index.Normal, "", nil),
			message(items[3], "", items[3].Subject, index.Restricted, "", nil),
			message(items[4], "bank.example", items[4].Subject, index.Restricted, "rule.bank", nil),
		},
		Domains: []string{"", "bank.example", "newsletter.example", "security.example"},
	}
	if diff := cmp.Diff(want, index.Decide(items, composed(t), scanner(t), lookups), compare.Options); diff != "" {
		t.Errorf("Decide (-want +got):\n%s", diff)
	}
}

// A policy that never loaded restricts every sender, and a scanner nobody built masks every subject
// whole, so neither failure lets a sender through as normal or a subject through unmasked (ADR-0042).
// No rule set the class, so the message names none.
func TestDecideFailsClosed(t *testing.T) {
	item := metadata("m1", fixture.Newsletter(), 1000)
	got := index.Decide([]mail.MessageMetadata{item}, policy.Snapshot{}.For(account), scan.Scanner{}, lookups)
	subject := strings.Repeat("█", len([]rune(item.Subject)))
	want := index.Page{
		Messages: []index.Message{{
			ID: "m1", ThreadID: "t-m1", From: item.From, Domain: "newsletter.example", Subject: subject,
			SubjectMasked: true, Date: 1000, ListID: item.ListID, SizeBytes: 2048, Class: index.Restricted,
			Masks: []index.Mask{{Rule: redact.RuleWholeSubject}},
		}},
		Domains: []string{"newsletter.example"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Decide (-want +got):\n%s", diff)
	}
}

// A failure's error class follows the Provider Port's error it wraps, and a failure that is not the
// provider's has none.
func TestClassOf(t *testing.T) {
	cases := []struct {
		err  error
		want index.ErrorClass
	}{
		{mail.ThrottleError{}, index.Throttled},
		{errors.Join(errors.New("wrapped"), mail.ErrProvider), index.ProviderError},
		{mail.ErrAuthentication, index.Authentication},
		{mail.ErrNotFound, index.Gone},
		{mail.ErrInvalid, index.Validation},
		{errors.New("the database went away"), index.NotProvider},
	}
	for _, c := range cases {
		if got := index.ClassOf(c.err); got != c.want {
			t.Errorf("ClassOf(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, its comparison. A
// domain stored as restricted or skipped as restricted that the policy no longer lists is delisted,
// one still listed or listed through a parent domain is not, a domain the classifier cannot read
// stays restricted, and a policy that never loaded delists nothing (ADR-0037).
func TestDelisted(t *testing.T) {
	stored := []string{"bank.example", "mail.gov.example", "shop.example", ""}
	cases := []struct {
		name   string
		policy policy.Composed
		want   []string
	}{
		{"every rule in place", listing(t, "bank.example", "gov.example", "shop.example"), nil},
		{"the bank's rule removed", listing(t, "gov.example", "shop.example"), []string{"bank.example"}},
		{"every rule removed", listing(t), []string{"bank.example", "mail.gov.example", "shop.example"}},
		{"a policy that never loaded", policy.Composed{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, index.Delisted(c.policy, lookups, stored), compare.Options); diff != "" {
				t.Errorf("Delisted (-want +got):\n%s", diff)
			}
		})
	}
}

// An added rule reaches the domains the index stores as normal by its effect, each with the rule that
// now restricts it, and a domain the classifier cannot read or a policy that never loaded lists nothing
// (ADR-0113).
func TestListed(t *testing.T) {
	stored := []string{"bank.example", "mail.gov.example", "shop.example", ""}
	cases := []struct {
		name   string
		policy policy.Composed
		want   []index.Listing
	}{
		{"no rule", listing(t), nil},
		{"a rule for the bank and one for a parent domain", listing(t, "bank.example", "gov.example"), []index.Listing{
			{Domain: "bank.example", Rule: "rule.bank"},
			{Domain: "mail.gov.example", Rule: "rule.gov.example"},
		}},
		{"a policy that never loaded", policy.Composed{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, index.Listed(c.policy, lookups, stored), compare.Options); diff != "" {
				t.Errorf("Listed (-want +got):\n%s", diff)
			}
		})
	}
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

// The gate decides from the message's own inputs, its sender's class under the policy in force and
// the sender's prior hits counting the hits found earlier on the same page, so a sender's first hit
// reaches its next message (ADR-0093, ADR-0094). A restricted sender is skipped as restricted whatever
// else asks for a scan (ADR-0008).
func TestGate(t *testing.T) {
	cases := []struct {
		name     string
		policy   policy.Composed
		message  func(index.Waiting) index.Waiting
		pageHits int64
		want     string
	}{
		{"the high-volume skip", listing(t), func(m index.Waiting) index.Waiting { return m }, 0, "high_volume_no_hits"},
		{"a hit earlier on the page", listing(t), func(m index.Waiting) index.Waiting { return m }, 1, "prior_hit"},
		{"a stored prior hit", listing(t), func(m index.Waiting) index.Waiting { m.SenderHits = 1; return m }, 0, "prior_hit"},
		{"no List-Id", listing(t), func(m index.Waiting) index.Waiting { m.ListID = false; return m }, 0, "default"},
		{"a masked subject", listing(t), func(m index.Waiting) index.Waiting { m.SubjectMasked = true; return m }, 0, "subject_signal"},
		{
			"a listed local part without a List-Id", listing(t),
			func(m index.Waiting) index.Waiting { m.ListID, m.From = false, "NoReply@list.example"; return m }, 0, "noreply_local_part",
		},
		{"a sender the policy lists", listing(t, "list.example"), func(m index.Waiting) index.Waiting { m.SubjectMasked = true; return m }, 1, "restricted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := index.Gate(c.policy, lookups, thresholds, now, c.message(skipped()), c.pageHits)
			if got.Reason().String() != c.want {
				t.Errorf("Gate decided %q, want %q", got.Reason().String(), c.want)
			}
		})
	}
}

// A scanned body records a flag found in either part, the content rules that fired in either, each
// once and sorted, and the scanner's version and revision. A scanner nobody built flags the body
// (ADR-0009, ADR-0017).
func TestScan(t *testing.T) {
	const code, link = "Your verification code is 419283.", "Sign in: https://x.example/login/h3J9dK2mQ8xR5tY1vB7nW4sZ6"
	type got struct {
		MFA, Link bool
		Rules     []string
		Version   int
		Revision  string
	}
	observe := func(s index.Scanned) got {
		return got{MFA: s.Flags.MFACode(), Link: s.Flags.LoginLink(), Rules: s.Rules, Version: s.Version, Revision: s.Revision}
	}
	s := scanner(t)
	cases := []struct {
		name           string
		scanner        scan.Scanner
		markdown, text string
		want           got
	}{
		{"a clean body", s, "Hello there.", "Hello there.", got{Rules: []string{}, Version: 1, Revision: "a-revision"}},
		{"a code in the HTML part", s, code, "Hello there.", got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a code in the text part", s, "Hello there.", code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a code in both parts", s, code, code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{
			"a code in one part and a link in the other", s, link, code,
			got{MFA: true, Link: true, Rules: []string{scan.RuleLinkPath, scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"},
		},
		{"a body with no HTML part", s, "", code, got{MFA: true, Rules: []string{scan.RuleTriggerWindow}, Version: 1, Revision: "a-revision"}},
		{"a scanner nobody built", scan.Scanner{}, "Hello there.", "Hello there.", got{MFA: true, Link: true, Rules: []string{}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(index.Scan(c.scanner, c.markdown, c.text)), compare.Options); diff != "" {
				t.Errorf("Scan (-want +got):\n%s", diff)
			}
		})
	}
}
