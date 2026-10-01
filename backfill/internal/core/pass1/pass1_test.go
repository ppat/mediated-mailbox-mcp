package pass1_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
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
	s, err := policy.Load([]policy.Row{{ID: "rule.bank", Class: policy.Restricted, DomainSuffixes: []string{"bank.example"}}})
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
// sender's included (ADR-0003, ADR-0004). A listed sender carries the rule that restricted it, and
// no other sender carries a rule (ADR-0016). An address whose domain cannot be read is restricted and
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
	code := []pass1.Mask{{Rule: scan.RuleTriggerWindow, Tier: 1}}
	message := func(m mail.MessageMetadata, domain, subject string, class pass1.Class, rule string, masks []pass1.Mask) pass1.Message {
		return pass1.Message{
			ID: m.ID, ThreadID: m.ThreadID, From: m.From, Domain: domain, Subject: subject,
			SubjectMasked: len(masks) > 0, Date: m.Date, Labels: m.Labels, ListID: m.ListID,
			SizeBytes: m.SizeBytes, Class: class, ClassRule: rule, Unclassified: m.From.Email == "no-address-at-all", Masks: masks,
		}
	}
	want := pass1.Page{
		Messages: []pass1.Message{
			message(items[0], "security.example", codeSubject(), pass1.Normal, "", code),
			message(items[1], "bank.example", codeSubject(), pass1.Restricted, "rule.bank", code),
			message(items[2], "newsletter.example", items[2].Subject, pass1.Normal, "", nil),
			message(items[3], "", items[3].Subject, pass1.Restricted, "", nil),
			message(items[4], "bank.example", items[4].Subject, pass1.Restricted, "rule.bank", nil),
		},
		Domains: []string{"", "bank.example", "newsletter.example", "security.example"},
	}
	if diff := cmp.Diff(want, pass1.Decide(items, composed(t), scanner(t), lookups), compare.Options); diff != "" {
		t.Errorf("Decide (-want +got):\n%s", diff)
	}
}

// A policy that never loaded restricts every sender, and a scanner nobody built masks every subject
// whole, so neither failure lets a sender through as normal or a subject through unmasked (ADR-0042).
// No rule set the class, so the message names none.
func TestDecideFailsClosed(t *testing.T) {
	item := metadata("m1", fixture.Newsletter(), 1000)
	got := pass1.Decide([]mail.MessageMetadata{item}, policy.Snapshot{}.For(account), scan.Scanner{}, lookups)
	subject := strings.Repeat("█", len([]rune(item.Subject)))
	want := pass1.Page{
		Messages: []pass1.Message{{
			ID: "m1", ThreadID: "t-m1", From: item.From, Domain: "newsletter.example", Subject: subject,
			SubjectMasked: true, Date: 1000, ListID: item.ListID, SizeBytes: 2048, Class: pass1.Restricted,
			Masks: []pass1.Mask{{Rule: redact.RuleWholeSubject}},
		}},
		Domains: []string{"newsletter.example"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("Decide (-want +got):\n%s", diff)
	}
}

// How a run starts, from whether the pass has ended and the latest run recorded (ADR-0017).
func TestBegin(t *testing.T) {
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 7, Token: "p8"}, Counters: pass1.Counters{Pages: 7, Messages: 690}}
	cases := []struct {
		name   string
		ended  bool
		latest pass1.Latest[pass1.Progress]
		want   pass1.Start[pass1.Progress]
	}{
		{"a pass that ended", true, pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Succeeded, Progress: at}, pass1.Start[pass1.Progress]{Skip: true}},
		{"a pass that ended with a run left running", true, pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Running, Progress: at}, pass1.Start[pass1.Progress]{Skip: true}},
		{"no run yet", false, pass1.Latest[pass1.Progress]{}, pass1.Start[pass1.Progress]{}},
		{"a run that failed", false, pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Failed, Progress: at}, pass1.Start[pass1.Progress]{ResumedFrom: "r1", From: at}},
		{
			"a run that stopped without recording its end", false,
			pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Running, Progress: at},
			pass1.Start[pass1.Progress]{ResumedFrom: "r1", Abandon: true, From: at},
		},
		{"a pass asked to run again after it succeeded", false, pass1.Latest[pass1.Progress]{Found: true, RunID: "r1", State: pass1.Succeeded, Progress: at}, pass1.Start[pass1.Progress]{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, pass1.Begin(c.ended, c.latest), compare.Options); diff != "" {
				t.Errorf("Begin (-want +got):\n%s", diff)
			}
		})
	}
}

// A page made durable moves the checkpoint one page on to the provider's next token and adds what it
// added to the counters. A pass started over keeps its counters and returns to the first page.
func TestAdvanceAndRestart(t *testing.T) {
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 2, Token: "p3"}, Counters: pass1.Counters{Pages: 4, Messages: 150}}
	want := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 3, Token: "p4"}, Counters: pass1.Counters{Pages: 5, Messages: 170}}
	if diff := cmp.Diff(want, pass1.Advance(at, "p4", nil, 20), compare.Options); diff != "" {
		t.Errorf("Advance (-want +got):\n%s", diff)
	}
	last := pass1.Advance(at, "", nil, 0)
	if !last.Checkpoint.Ended() || at.Checkpoint.Ended() || (pass1.Checkpoint{}).Ended() {
		t.Errorf("Ended is %v after the last page, %v mid-way and %v before the first, want true, false, false",
			last.Checkpoint.Ended(), at.Checkpoint.Ended(), (pass1.Checkpoint{}).Ended())
	}
	if diff := cmp.Diff(pass1.Progress{Counters: at.Counters}, pass1.Restart(at), compare.Options); diff != "" {
		t.Errorf("Restart (-want +got):\n%s", diff)
	}
}

// The pages an enumeration takes come from the total and page limit the page reports and its next
// token alone. A page with no total gives no estimate. Before the last page it is the items over the
// limit, rounded up, so a short last page is still a page, and never fewer than one past the current
// page while a next token says another follows. Each page's total gives a fresh figure, so a mailbox
// that grows or shrinks moves it. At the last page it is that page.
func TestAdvanceEstimatesThePagesFromTheTotal(t *testing.T) {
	at := pass1.Progress{Checkpoint: pass1.Checkpoint{Page: 2, Token: "p3", Of: 9}}
	total := func(items, limit int) *mail.Total { return &mail.Total{Items: items, PageLimit: limit} }
	cases := []struct {
		name  string
		next  mail.PageToken
		total *mail.Total
		want  int
	}{
		{"no total", "p4", nil, 0},
		{"no total at the last page", "", nil, 0},
		{"a total filling its pages", "p4", total(30, 3), 10},
		{"a total whose last page is short", "p4", total(31, 3), 11},
		{"a mailbox that shrank below the pages taken", "p4", total(4, 3), 4},
		{"a total of nothing while pages remain", "p4", total(0, 3), 4},
		{"the last page", "", total(31, 3), 3},
		{"a page limit of nothing", "p4", total(30, 0), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pass1.Advance(at, c.next, c.total, 0).Checkpoint.Of; got != c.want {
				t.Errorf("Advance to page 3 with the next token %q and the total %+v gave of %d, want %d", c.next, c.total, got, c.want)
			}
		})
	}
}

// What a run does after a failed attempt at a page. Refusing the token the run resumed from starts the
// enumeration over once. A throttled or failed page is asked for again until the last attempt, after a
// backoff when throttled. Everything else, a refused token the run got in this run included, fails the
// run.
func TestOnFailure(t *testing.T) {
	cases := []struct {
		name string
		f    pass1.Attempt
		want pass1.Next
	}{
		{"a refused resumed token", pass1.Attempt{Class: pass1.Validation, Attempts: 1, ResumedToken: true}, pass1.StartOver},
		{"a refused resumed token after the pass started over", pass1.Attempt{Class: pass1.Validation, Attempts: 1, ResumedToken: true, Restarted: true}, pass1.Abandon},
		{"a refused token the run got in this run", pass1.Attempt{Class: pass1.Validation, Attempts: 1}, pass1.Abandon},
		{"a first throttle", pass1.Attempt{Class: pass1.Throttled, Attempts: 1}, pass1.Backoff},
		{"a throttle before the last attempt", pass1.Attempt{Class: pass1.Throttled, Attempts: pass1.MaxAttempts - 1}, pass1.Backoff},
		{"a throttle on the last attempt", pass1.Attempt{Class: pass1.Throttled, Attempts: pass1.MaxAttempts}, pass1.Abandon},
		{"a provider failure", pass1.Attempt{Class: pass1.ProviderError, Attempts: 2}, pass1.Retry},
		{"a provider failure on the last attempt", pass1.Attempt{Class: pass1.ProviderError, Attempts: pass1.MaxAttempts}, pass1.Abandon},
		{"a throttle on a resumed token", pass1.Attempt{Class: pass1.Throttled, Attempts: 1, ResumedToken: true}, pass1.Backoff},
		{"a refused credential", pass1.Attempt{Class: pass1.Authentication, Attempts: 1}, pass1.Abandon},
		{"a page that is gone", pass1.Attempt{Class: pass1.Gone, Attempts: 1, ResumedToken: true}, pass1.Abandon},
		{"a failure that is not the provider's", pass1.Attempt{Class: pass1.NotProvider, Attempts: 1, ResumedToken: true}, pass1.FailRun},
		{"an attempt nobody described", pass1.Attempt{}, pass1.FailRun},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pass1.OnFailure(c.f); got != c.want {
				t.Errorf("OnFailure(%+v) = %v, want %v", c.f, got, c.want)
			}
		})
	}
	if pass1.MaxAttempts != 5 {
		t.Errorf("MaxAttempts is %d, want 5", pass1.MaxAttempts)
	}
}

// A failure's error class follows the Provider Port's error it wraps, and a failure that is not the
// provider's has none.
func TestClassOf(t *testing.T) {
	cases := []struct {
		err  error
		want pass1.ErrorClass
	}{
		{mail.ThrottleError{}, pass1.Throttled},
		{errors.Join(errors.New("wrapped"), mail.ErrProvider), pass1.ProviderError},
		{mail.ErrAuthentication, pass1.Authentication},
		{mail.ErrNotFound, pass1.Gone},
		{mail.ErrInvalid, pass1.Validation},
		{errors.New("the database went away"), pass1.NotProvider},
	}
	for _, c := range cases {
		if got := pass1.ClassOf(c.err); got != c.want {
			t.Errorf("ClassOf(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
