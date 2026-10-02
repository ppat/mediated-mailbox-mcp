//go:build integration

package api_test

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// finish pastes an address into the connect page of a session.
func (b *browser) finish(address string) response {
	return b.post("/api/setup/connect/finish", map[string]any{"address": address})
}

// An account connected through the UI holds its two rows, written together, its credential sealed to
// its state row, opening only with the private key to the grant the consent returned, the mailbox the
// consent confirmed and the code exchange's attempt as its latest, and the attempt ends (ADR-0080,
// ADR-0081, ADR-0091, ADR-0097, VERIFICATIONS, the stored credential row).
func TestAConnectedAccountHoldsItsSealedGrant(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	target := 0.3
	a := decode[started](t, b.post("/api/setup/connect", map[string]any{"account": "household", "client": household, "mailbox": "Jo.Smith@gmail.com", "lowered_target": target}))
	if a.Attempt.Kind != "connect" || a.Attempt.Account != "household" || !strings.HasPrefix(a.Attempt.ConsentAddress, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Fatalf("the attempt reads %+v", a.Attempt)
	}
	q, err := url.Parse(a.Attempt.ConsentAddress)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Query(); got.Get("login_hint") != "Jo.Smith@gmail.com" || got.Get("redirect_uri") != defaultRedirect || got.Get("scope") != consent.Scope {
		t.Errorf("the consent page's address asks %v", got)
	}
	r.clock = r.clock.Add(time.Minute)
	got := decode[map[string]string](t, b.finish(r.grant(a, "josmith@googlemail.com")))
	if got["account"] != "household" || got["kind"] != "connect" || got["mailbox"] != "josmith@googlemail.com" {
		t.Errorf("the finish answered %v", got)
	}
	if n := r.count(`SELECT 1 FROM accounts a JOIN account_state s USING (account_id)
		WHERE a.account_id = 'household' AND a.provider = 'gmail' AND a.oauth_client = $1 AND s.mailbox = 'josmith@googlemail.com'
		AND s.lowered_target_rate = 0.3::real AND s.last_auth_outcome = 'succeeded' AND s.last_auth_at = $2`, household, r.clock); n != 1 {
		t.Errorf("the account's rows read:\n%s", r.stored())
	}
	sealed := r.sealed("SELECT credential FROM account_state WHERE account_id = 'household'")
	credential, err := r.open(sealed, seal.AccountCredential("household"))
	if err != nil || !strings.HasPrefix(credential, "fake-refresh-token-") {
		t.Errorf("the credential opened to %q, %v", credential, err)
	}
	if strings.Contains(string(sealed), credential) {
		t.Errorf("the stored credential holds the grant in plaintext")
	}
	if _, err := r.open(sealed, seal.AccountCredential(personal)); err == nil {
		t.Errorf("the credential opened bound to another account's row")
	}
	if got := decode[started](t, b.get("/api/setup/connect")); got.Attempt.Attempt != "" {
		t.Errorf("the attempt outlived its success: %+v", got)
	}
}

// An account identifier the mediator's API root cannot address, or one of the words the UI's own
// top-level paths use, is refused when the attempt starts, before anything happens at the provider,
// and nothing is stored (ADR-0087, docs/UI.md section 8.12, VERIFICATIONS, the two identifier rows).
// The bundle here holds main.js, main.css and a fonts directory, so those three are its top level.
func TestAnIdentifierNoScreenCouldReachIsRefused(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	for _, id := range []string{".", "..", "/", "setup", "api", "main.js", "main.css", "fonts", ""} {
		res := b.post("/api/setup/connect", map[string]any{"account": id, "client": household, "mailbox": "jo@example.com", "lowered_target": nil})
		refused(t, res, http.StatusBadRequest, "identifier_refused")
	}
	if _, ok := b.cookies["ui_consent"]; ok {
		t.Errorf("a refused identifier started an attempt")
	}
	refused(t, b.post("/api/setup/connect", map[string]any{"account": personal, "client": household, "mailbox": "jo@example.com", "lowered_target": nil}),
		http.StatusConflict, "identifier_taken")
	refused(t, b.post("/api/setup/connect", map[string]any{"account": "jo", "client": "absent-client", "mailbox": "jo@example.com", "lowered_target": nil}),
		http.StatusNotFound, "unknown_client")
	for _, target := range []float64{0.05, 0.51} {
		refused(t, b.post("/api/setup/connect", map[string]any{"account": "jo", "client": household, "mailbox": "jo@example.com", "lowered_target": target}),
			http.StatusBadRequest, "target_refused")
	}
	if got := r.stored(); got != before {
		t.Errorf("a refused start stored:\n%s", got)
	}
}

// A grant for a mailbox other than the one named is refused and stores nothing, while one for the
// same mailbox in other capitals, with its dots moved, or under googlemail.com is accepted (docs/UI.md
// section 8.12, VERIFICATIONS, the mailbox comparison row).
func TestTheMailboxIsComparedAsGoogleIdentifiesIt(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	a := b.connect("jo", household, "jo.smith@gmail.com")
	res := b.finish(r.grant(a, "someone.else@gmail.com"))
	refused(t, res, http.StatusBadRequest, "wrong_mailbox")
	if !strings.Contains(string(res.body), "someone.else@gmail.com") {
		t.Errorf("the refusal does not name the mailbox granted:\n%s", res.body)
	}
	if got := r.stored(); got != before {
		t.Fatalf("a grant for another mailbox stored:\n%s", got)
	}
	for i, granted := range []string{"JO.SMITH@GMAIL.COM", "josmith@gmail.com", "j.o.smith@googlemail.com"} {
		account := "jo-" + string(rune('a'+i))
		a := b.connect(account, household, "jo.smith@gmail.com")
		decode[map[string]any](t, b.finish(r.grant(a, granted)))
	}
}

// A grant whose mailbox's API is not enabled for the client's project is refused with api_disabled,
// and nothing is stored (docs/UI.md section 8.12, VERIFICATIONS, the disabled API row).
func TestAGrantWhoseAPIIsDisabledIsRefused(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	r.consent.DisableAPI(householdID)
	before := r.stored()
	a := b.connect("jo", household, "jo@gmail.com")
	refused(t, b.finish(r.grant(a, "jo@gmail.com")), http.StatusBadRequest, "api_disabled")
	if got := r.stored(); got != before {
		t.Errorf("a grant with its API disabled stored:\n%s", got)
	}
}

// Only the session's latest attempt can finish. The first of two attempts in one session is refused
// with wrong_attempt, an address pasted in a session holding no attempt with no_attempt, and nothing is
// stored either time (docs/UI.md sections 8.12 and 15, VERIFICATIONS, the attempt row).
func TestOnlyTheSessionsLatestAttemptFinishes(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	first := b.connect("jo", household, "jo@gmail.com")
	firstAddress := r.grant(first, "jo@gmail.com")
	second := b.connect("jo", household, "jo@gmail.com")
	refused(t, b.finish(firstAddress), http.StatusBadRequest, "wrong_attempt")
	if got := decode[started](t, b.get("/api/setup/connect")); got.Attempt.Attempt != second.Attempt.Attempt {
		t.Errorf("the session reads the attempt %q, want the newer %q", got.Attempt.Attempt, second.Attempt.Attempt)
	}
	refused(t, r.browser().finish(r.grant(second, "jo@gmail.com")), http.StatusBadRequest, "no_attempt")
	if got := r.stored(); got != before {
		t.Errorf("a refused finish stored:\n%s", got)
	}
}

// Each refusal a pasted address can meet names its cause, stores nothing, and leaves the attempt in
// place, so opening the consent page again and pasting the new address finishes it (docs/UI.md
// section 8.12, ADR-0111). An attempt past its expiry answers as expired whatever is pasted.
func TestAPastedAddressIsRefusedForItsCause(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	a := b.connect("jo", household, "jo@gmail.com")
	address := r.grant(a, "jo@gmail.com")
	declined, err := r.consent.Decline(a.Attempt.ConsentAddress)
	if err != nil {
		t.Fatal(err)
	}
	refused(t, b.finish(declined), http.StatusBadRequest, "consent_declined")
	refused(t, b.finish(pasted(t, address, func(q url.Values) { q.Del("code") })), http.StatusBadRequest, "no_code")
	refused(t, b.finish(strings.Replace(address, "127.0.0.1", "localhost", 1)), http.StatusBadRequest, "wrong_address")
	refused(t, b.finish("https://example.com/?code=x"), http.StatusBadRequest, "wrong_address")
	refused(t, b.finish(pasted(t, address, func(q url.Values) { q.Set("code", "a-code-no-consent-issued") })), http.StatusBadRequest, "code_refused")
	scoped, err := r.consent.Grant(a.Attempt.ConsentAddress, "jo@gmail.com", "")
	if err != nil {
		t.Fatal(err)
	}
	refused(t, b.finish(scoped), http.StatusBadRequest, "scope_missing")
	scoped, err = r.consent.Grant(a.Attempt.ConsentAddress, "jo@gmail.com", consent.Scope+" https://mail.google.com/")
	if err != nil {
		t.Fatal(err)
	}
	refused(t, b.finish(scoped), http.StatusBadRequest, "scope_refused")
	r.consent.SetUnreachable(true)
	refused(t, b.finish(r.grant(a, "jo@gmail.com")), http.StatusBadGateway, "provider_unreachable")
	r.consent.SetUnreachable(false)
	if got := r.stored(); got != before {
		t.Fatalf("a refused finish stored:\n%s", got)
	}
	decode[map[string]any](t, b.finish(address))
	r.clock = r.clock.Add(16 * time.Minute)
	late := b.connect("late", household, "late@gmail.com")
	r.clock = r.clock.Add(15 * time.Minute)
	refused(t, b.finish(r.grant(late, "late@gmail.com")), http.StatusBadRequest, "attempt_expired")
	refused(t, b.finish("https://example.com/another-tab"), http.StatusBadRequest, "attempt_expired")
}

// An identifier checked free when an attempt started is checked again when it finishes, so of two
// sessions connecting under one identifier the second is refused with identifier_taken, stores
// nothing, and leaves the first account's rows as they were (ADR-0091, VERIFICATIONS, the identifier
// race row).
func TestASecondConnectionUnderOneIdentifierIsRefused(t *testing.T) {
	r := newRig(t)
	one, two := r.browser(), r.browser()
	one.addClient(household, householdID, householdSecret)
	first := one.connect("jo", household, "jo@gmail.com")
	second := two.connect("jo", household, "jo@gmail.com")
	decode[map[string]any](t, one.finish(r.grant(first, "jo@gmail.com")))
	before := r.stored()
	refused(t, two.finish(r.grant(second, "jo@gmail.com")), http.StatusConflict, "identifier_taken")
	if got := r.stored(); got != before {
		t.Errorf("the second connection changed:\nbefore %s\nafter  %s", before, got)
	}
}

// A client replaced or removed after an attempt started fails its finish with client_changed, and
// nothing is stored (docs/UI.md section 8.12).
func TestAClientReplacedMidAttemptFailsTheFinish(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	a := b.connect("jo", household, "jo@gmail.com")
	r.consent.AddClient("replacement.apps.example", householdSecret)
	decode[map[string]any](t, b.post("/api/setup/gmail/clients/"+household, map[string]any{"client_id": "replacement.apps.example", "client_secret": householdSecret, "project_id": nil}))
	before := r.stored()
	refused(t, b.finish(r.grant(a, "jo@gmail.com")), http.StatusConflict, "client_changed")
	if got := r.stored(); got != before {
		t.Errorf("a finish through a replaced client stored:\n%s", got)
	}
}

// Connecting writes the account's two rows in one transaction, so a fault between them leaves neither
// (ADR-0091, ADR-0060, VERIFICATIONS, the half-written account row).
func TestAConnectionFailedBetweenItsWritesStoresNeitherRow(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	var faults []string
	api.SetFault(r.s, func(at string) error {
		faults = append(faults, at)
		return errors.New("a fault injected between the writes")
	})
	a := b.connect("jo", household, "jo@gmail.com")
	refused(t, b.finish(r.grant(a, "jo@gmail.com")), http.StatusServiceUnavailable, "database")
	if len(faults) != 1 {
		t.Fatalf("the fault was reached %d times", len(faults))
	}
	if got := r.stored(); got != before {
		t.Errorf("a connection failed between its writes stored:\n%s", got)
	}
}

// The attempt is held in a cookie only the server opens, so another replica sharing the request
// token's key finishes it, and one holding another key finds none (ADR-0111).
func TestAnAttemptFinishesOnAnyReplicaSharingTheKey(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	a := b.connect("jo", household, "jo@gmail.com")
	address := r.grant(a, "jo@gmail.com")
	stranger := b.on(r.server([]byte("another replica's key, 32 bytes or more")))
	refused(t, stranger.finish(address), http.StatusForbidden, "stale_page")
	stranger.token = ""
	if got := decode[started](t, stranger.get("/api/setup/connect")); got.Attempt.Attempt != "" {
		t.Errorf("a replica with another key opened the attempt")
	}
	replica := b.on(r.server(tokenKey))
	if got := decode[started](t, replica.get("/api/setup/connect")); got.Attempt.Attempt != a.Attempt.Attempt {
		t.Errorf("the replica reads the attempt %+v", got)
	}
	decode[map[string]any](t, replica.finish(address))
}

// reauthorize starts the re-authorization of an account in a session.
func (b *browser) reauthorize(account string, body map[string]any) response {
	return b.post("/api/"+account+"/account/reauthorize", body)
}

// Re-authorizing checks the grant against the mailbox the account remembers, and refuses a request
// naming a mailbox of its own while one is remembered. Each refusal leaves the stored credential and
// the mailbox unchanged (ADR-0080, ADR-0107, VERIFICATIONS, the re-authorization mailbox row).
func TestReauthorizingCannotChangeTheMailbox(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	r.connected(b, "jo", household, "jo@gmail.com")
	before := r.stored()
	refused(t, b.reauthorize("jo", map[string]any{"mailbox": "other@gmail.com", "client": nil}), http.StatusBadRequest, "mailbox_remembered")
	a := decode[started](t, b.reauthorize("jo", map[string]any{"mailbox": nil, "client": nil}))
	if a.Attempt.Kind != "reauthorize" || a.Attempt.Mailbox != "jo@gmail.com" {
		t.Fatalf("the attempt reads %+v", a.Attempt)
	}
	refused(t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": r.grant(a, "other@gmail.com")}), http.StatusBadRequest, "wrong_mailbox")
	if got := r.stored(); got != before {
		t.Errorf("a refused re-authorization changed:\nbefore %s\nafter  %s", before, got)
	}
}

// A re-authorization of an account whose state reads refused stores the new credential and records
// the code exchange's attempt, succeeded at the exchange's start, in the same transaction, even over a
// credential a deployable has since rewritten. One whose code the provider refuses leaves the row and
// the credential unchanged (ADR-0097, ADR-0089, VERIFICATIONS, the recorded attempt and the operator's
// write rows).
func TestAReauthorizationRecordsItsAttemptAndAlwaysLands(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	r.connected(b, "jo", household, "jo@gmail.com")
	r.exec("UPDATE account_state SET credential = 'a deployable wrote this', last_auth_outcome = 'refused', last_auth_at = $1 WHERE account_id = 'jo'", r.clock)
	before := r.stored()
	a := decode[started](t, b.reauthorize("jo", map[string]any{"mailbox": nil, "client": nil}))
	refused(t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": pasted(t, r.grant(a, "jo@gmail.com"), func(q url.Values) { q.Set("code", "refused") })}),
		http.StatusBadRequest, "code_refused")
	if got := r.stored(); got != before {
		t.Fatalf("a refused code changed:\nbefore %s\nafter  %s", before, got)
	}
	r.clock = r.clock.Add(time.Minute)
	got := decode[map[string]string](t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": r.grant(a, "J.O@gmail.com")}))
	if got["kind"] != "reauthorize" || got["mailbox"] != "jo@gmail.com" {
		t.Errorf("the finish answered %v", got)
	}
	if n := r.count("SELECT 1 FROM account_state WHERE account_id = 'jo' AND mailbox = 'jo@gmail.com' AND last_auth_outcome = 'succeeded' AND last_auth_at = $1", r.clock); n != 1 {
		t.Errorf("the attempt is not recorded:\n%s", r.stored())
	}
	if credential, err := r.open(r.sealed("SELECT credential FROM account_state WHERE account_id = 'jo'"), seal.AccountCredential("jo")); err != nil ||
		!strings.HasPrefix(credential, "fake-refresh-token-") {
		t.Errorf("the stored credential opened to %q, %v", credential, err)
	}
}

// A move writes the new client and the credential issued to it together or neither. A fault between
// the two writes, and a move whose code the provider refuses, each leave the account on its old client
// with its old credential, and a move that lands names the new client (ADR-0106, VERIFICATIONS, the
// move row).
func TestAMoveWritesTheClientAndTheCredentialTogether(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	b.addClient(spare, spareID, spareSecret)
	r.connected(b, "jo", household, "jo@gmail.com")
	refused(t, b.reauthorize("jo", map[string]any{"mailbox": nil, "client": "absent-client"}), http.StatusNotFound, "unknown_client")
	before := r.stored()
	a := decode[started](t, b.reauthorize("jo", map[string]any{"mailbox": nil, "client": spare}))
	if a.Attempt.Kind != "move" || a.Attempt.Client != spare {
		t.Fatalf("the attempt reads %+v", a.Attempt)
	}
	refused(t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": pasted(t, r.grant(a, "jo@gmail.com"), func(q url.Values) { q.Set("code", "refused") })}),
		http.StatusBadRequest, "code_refused")
	if got := r.stored(); got != before {
		t.Fatalf("a refused move changed:\nbefore %s\nafter  %s", before, got)
	}
	api.SetFault(r.s, func(string) error { return errors.New("a fault injected between the writes") })
	refused(t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": r.grant(a, "jo@gmail.com")}), http.StatusServiceUnavailable, "database")
	if got := r.stored(); got != before {
		t.Fatalf("a move failed between its writes changed:\nbefore %s\nafter  %s", before, got)
	}
	api.SetFault(r.s, nil)
	decode[map[string]any](t, b.post("/api/jo/account/reauthorize/finish", map[string]any{"address": r.grant(a, "jo@gmail.com")}))
	if n := r.count("SELECT 1 FROM accounts WHERE account_id = 'jo' AND oauth_client = $1", spare); n != 1 {
		t.Errorf("the moved account reads:\n%s", r.stored())
	}
}

// An account with no state row is connected through its re-authorization, which asks for the mailbox
// once and remembers the one the provider confirmed (ADR-0080, docs/UI.md section 8.12).
func TestAnAccountWithNoStateIsConnectedByReauthorizing(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	r.exec("UPDATE accounts SET oauth_client = $1 WHERE account_id = $2", household, other)
	refused(t, b.reauthorize(other, map[string]any{"mailbox": nil, "client": nil}), http.StatusBadRequest, "mailbox_required")
	a := decode[started](t, b.reauthorize(other, map[string]any{"mailbox": "Other.Person@gmail.com", "client": nil}))
	got := decode[map[string]any](t, b.post("/api/"+other+"/account/reauthorize/finish", map[string]any{"address": r.grant(a, "otherperson@gmail.com")}))
	if got["mailbox"] != "otherperson@gmail.com" {
		t.Errorf("the finish answered %v", got)
	}
	if n := r.count("SELECT 1 FROM account_state WHERE account_id = $1 AND mailbox = 'otherperson@gmail.com' AND credential IS NOT NULL", other); n != 1 {
		t.Errorf("the account's state reads:\n%s", r.stored())
	}
}

// Account settings read what the UI reads of the account's two rows and never its credential, and
// its target is stored within ADR-0024's range or cleared, and refused for an account with no state row
// (docs/UI.md section 8.13).
func TestAccountSettingsReadAndSetTheTarget(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	b.addClient(spare, spareID, spareSecret)
	r.connected(b, "jo", household, "jo@gmail.com")
	res := b.get("/api/jo/account")
	got := decode[map[string]any](t, res)
	if got["oauth_client"] != household || got["mailbox"] != "jo@gmail.com" || got["connected"] != true || got["lowered_target"] != nil {
		t.Errorf("account settings read %v", got)
	}
	if others, ok := got["other_clients"].([]any); !ok || len(others) != 1 || others[0] != spare {
		t.Errorf("the other clients are %v", got["other_clients"])
	}
	if strings.Contains(string(res.body), "fake-refresh-token") || strings.Contains(string(res.body), "credential") {
		t.Errorf("account settings carry the credential:\n%s", res.body)
	}
	decode[map[string]any](t, b.post("/api/jo/account/target", map[string]any{"lowered_target": 0.25}))
	if n := r.count("SELECT 1 FROM account_state WHERE account_id = 'jo' AND lowered_target_rate = 0.25::real"); n != 1 {
		t.Errorf("the target is not stored")
	}
	refused(t, b.post("/api/jo/account/target", map[string]any{"lowered_target": 0.05}), http.StatusBadRequest, "target_refused")
	decode[map[string]any](t, b.post("/api/jo/account/target", map[string]any{"lowered_target": nil}))
	if n := r.count("SELECT 1 FROM account_state WHERE account_id = 'jo' AND lowered_target_rate IS NULL"); n != 1 {
		t.Errorf("the target is not cleared")
	}
	refused(t, b.post("/api/"+other+"/account/target", map[string]any{"lowered_target": 0.25}), http.StatusConflict, "not_connected")
}

// A client's secret in plaintext lives only for the client check and the code exchange. It reaches no
// answer of any request a setup sends, no cookie, and no log line, while the exchange obtains it from the
// client's sealed row, so the connection lands only when the secret opened (ADR-0081).
func TestAClientsSecretReachesNoAnswerCookieOrLog(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	var seen []string
	keep := func(res response) response {
		seen = append(seen, string(res.body))
		seen = append(seen, res.header.Values("Set-Cookie")...)
		return res
	}
	keep(b.post("/api/setup/gmail/clients", map[string]any{"name": household, "client_id": householdID, "client_secret": householdSecret, "project_id": nil}))
	a := decode[started](t, keep(b.post("/api/setup/connect", map[string]any{"account": "jo", "client": household, "mailbox": "jo@gmail.com", "lowered_target": nil})))
	keep(b.get("/api/setup/connect"))
	keep(b.get("/api/setup"))
	decode[map[string]any](t, keep(b.finish(r.grant(a, "jo@gmail.com"))))
	keep(b.get("/api/jo/account"))
	seen = append(seen, r.log.String())
	for _, s := range seen {
		if strings.Contains(s, householdSecret) {
			t.Errorf("the secret reached:\n%s", s)
		}
	}
}

// An attempt is sealed to the session that started it, so its cookie carried by another session opens
// to no attempt, and that session can neither read it nor finish it (ADR-0111).
func TestAnAttemptOpensOnlyInItsSession(t *testing.T) {
	r := newRig(t)
	owner := r.browser()
	owner.addClient(household, householdID, householdSecret)
	a := owner.connect("jo", household, "jo@gmail.com")
	other := r.browser()
	other.cookies["ui_consent"] = owner.cookies["ui_consent"]
	if got := decode[started](t, other.get("/api/setup/connect")); got.Attempt.Attempt != "" {
		t.Errorf("another session read the attempt %+v", got)
	}
	before := r.stored()
	refused(t, other.finish(r.grant(a, "jo@gmail.com")), http.StatusBadRequest, "no_attempt")
	if got := r.stored(); got != before {
		t.Errorf("another session's finish stored:\n%s", got)
	}
}

// The account routes setup adds are scoped like every other, so all and an unknown account are refused
// before anything is read or written (ADR-0056, VERIFICATIONS, the per-account row).
func TestTheAccountSetupRoutesAreScoped(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	before := r.stored()
	for _, account := range []string{"all", "nobody"} {
		code := map[string]string{"all": "all_accounts", "nobody": "unknown_account"}[account]
		refused(t, b.get("/api/"+account+"/account"), http.StatusBadRequest, code)
		refused(t, b.get("/api/"+account+"/account/reauthorize"), http.StatusBadRequest, code)
		refused(t, b.post("/api/"+account+"/account/reauthorize", map[string]any{"mailbox": nil, "client": nil}), http.StatusBadRequest, code)
		refused(t, b.post("/api/"+account+"/account/reauthorize/finish", map[string]any{"address": "x"}), http.StatusBadRequest, code)
		refused(t, b.post("/api/"+account+"/account/target", map[string]any{"lowered_target": nil}), http.StatusBadRequest, code)
	}
	if got := r.stored(); got != before {
		t.Errorf("a refused request stored:\n%s", got)
	}
}

// Another configured redirect address is the one every use names. The consent request and the code
// exchange name it, the entry document hands it to the browser, a pasted address on it finishes the
// attempt, and one on the default address is refused as not the redirect (docs/UI.md sections 8.12
// and 18.1, VERIFICATIONS, the consent redirect row).
func TestAnotherConfiguredRedirectIsUsedEverywhere(t *testing.T) {
	const redirect = "http://[::1]:5000/"
	r := newRigAt(t, redirect)
	b := r.browser()
	if res := b.get("/setup"); !strings.Contains(string(res.body), `<meta name="mediated-mailbox.consent_redirect" content="http://[::1]:5000/">`) {
		t.Errorf("the entry document does not carry the configured redirect:\n%s", res.body)
	}
	b.addClient(household, householdID, householdSecret)
	a := b.connect("jo", household, "jo@gmail.com")
	q, err := url.Parse(a.Attempt.ConsentAddress)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Query().Get("redirect_uri"); got != redirect {
		t.Errorf("the consent request redirects to %q, want %q", got, redirect)
	}
	address := r.grant(a, "jo@gmail.com")
	if !strings.HasPrefix(address, redirect) {
		t.Fatalf("the provider redirected to %q", address)
	}
	refused(t, b.finish(strings.Replace(address, redirect, defaultRedirect, 1)), http.StatusBadRequest, "wrong_address")
	decode[map[string]any](t, b.finish(address))
}
