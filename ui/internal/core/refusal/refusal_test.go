package refusal_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
)

// Every code is pinned to the string the error contract sends, since the browser branches on codes
// and its recorded fixtures carry them, so a constant whose string drifts fails here before any call
// site sends it. The list is written out rather than read from the package, so a code added to
// Codes() and not here fails too. A constant declared and left out of Codes() is not caught, and is
// held by review.
func TestEveryCodeIsItsWireString(t *testing.T) {
	want := []struct {
		code refusal.Code
		wire string
	}{
		{refusal.AllAccounts, "all_accounts"},
		{refusal.APIDisabled, "api_disabled"},
		{refusal.AttemptExpired, "attempt_expired"},
		{refusal.ClientChanged, "client_changed"},
		{refusal.ClientExists, "client_exists"},
		{refusal.ClientInUse, "client_in_use"},
		{refusal.ClientRefused, "client_refused"},
		{refusal.ClientWrongProvider, "client_wrong_provider"},
		{refusal.CodeRefused, "code_refused"},
		{refusal.ConfirmationRequired, "confirmation_required"},
		{refusal.ConsentDeclined, "consent_declined"},
		{refusal.FileRefused, "file_refused"},
		{refusal.IdentifierRefused, "identifier_refused"},
		{refusal.IdentifierTaken, "identifier_taken"},
		{refusal.IdentityMissing, "identity_missing"},
		{refusal.InvalidFilter, "invalid_filter"},
		{refusal.InvalidLevel, "invalid_level"},
		{refusal.InvalidPage, "invalid_page"},
		{refusal.InvalidRange, "invalid_range"},
		{refusal.InvalidRow, "invalid_row"},
		{refusal.InvalidSort, "invalid_sort"},
		{refusal.MailboxRefused, "mailbox_refused"},
		{refusal.MailboxRemembered, "mailbox_remembered"},
		{refusal.MailboxRequired, "mailbox_required"},
		{refusal.MalformedBody, "malformed_body"},
		{refusal.MissingDataset, "missing_dataset"},
		{refusal.MissingFilter, "missing_filter"},
		{refusal.MissingGroup, "missing_group"},
		{refusal.MissingParent, "missing_parent"},
		{refusal.NameRefused, "name_refused"},
		{refusal.NameTaken, "name_taken"},
		{refusal.NoAttempt, "no_attempt"},
		{refusal.NoCode, "no_code"},
		{refusal.NotConnected, "not_connected"},
		{refusal.NotFound, "not_found"},
		{refusal.ProjectRefused, "project_refused"},
		{refusal.RepeatedParameter, "repeated_parameter"},
		{refusal.RuleRefused, "rule_refused"},
		{refusal.ScopeMissing, "scope_missing"},
		{refusal.ScopeRefused, "scope_refused"},
		{refusal.StalePage, "stale_page"},
		{refusal.StalePreview, "stale_preview"},
		{refusal.StaleRule, "stale_rule"},
		{refusal.TargetRefused, "target_refused"},
		{refusal.UnknownAccount, "unknown_account"},
		{refusal.UnknownClient, "unknown_client"},
		{refusal.UnknownDataset, "unknown_dataset"},
		{refusal.UnknownDimension, "unknown_dimension"},
		{refusal.UnknownGroup, "unknown_group"},
		{refusal.UnknownParameter, "unknown_parameter"},
		{refusal.UnknownProvider, "unknown_provider"},
		{refusal.UnknownRow, "unknown_row"},
		{refusal.UnknownRule, "unknown_rule"},
		{refusal.UnknownRun, "unknown_run"},
		{refusal.UnknownSort, "unknown_sort"},
		{refusal.WrongAddress, "wrong_address"},
		{refusal.WrongAttempt, "wrong_attempt"},
		{refusal.WrongMailbox, "wrong_mailbox"},
	}
	got := refusal.Codes()
	if len(got) != len(want) {
		t.Fatalf("the package declares %d codes, the table pins %d", len(got), len(want))
	}
	for i, w := range want {
		if string(w.code) != w.wire {
			t.Errorf("%q is sent as %q", w.wire, string(w.code))
		}
		if got[i] != w.code {
			t.Errorf("code %d is %q, want %q", i, got[i], w.code)
		}
	}
}

// Every problem kind is pinned to its wire string, as the codes are.
func TestEveryKindIsItsWireString(t *testing.T) {
	want := []string{"blank_identifier", "repeated_identifier", "no_suffix", "invalid_suffix", "other_class", "reserved_identifier", "not_the_form", "too_large"}
	var got []string
	for _, k := range refusal.Kinds() {
		got = append(got, string(k))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("kinds (-want +got):\n%s", diff)
	}
}
