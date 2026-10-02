//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// The marked account the connect page's recordings name.
var (
	markedAccount = marker.Field("account")
	markedMailbox = marker.Field("mailbox") + "@gmail.com"
)

// recordedAttempt stands in a recording for an attempt's state, which the server draws at random, so
// the recordings do not move between runs. The browser's tests compare a pasted address's state with
// it as they would with the real one.
const recordedAttempt = "recorded-attempt"

// setupRecording is one answer the setup screens' tests are given, the request that produced it and
// the document's pattern it answers for.
type setupRecording struct {
	name, method, path, pattern string
	body                        any
}

// TestTheRecordedSetupFixturesMatchTheServer records the answers the installation, client setup,
// connect and account settings screens read and send, from a first run with no account to an
// installation whose accounts connect through its clients, each matching its declaration in the
// contract document (ADR-0064, ADR-0065). A browser test answers a request only with the recording
// made for that method and path.
func TestTheRecordedSetupFixturesMatchTheServer(t *testing.T) {
	r := newRig(t)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := r.browser()
	play := func(recordings ...setupRecording) {
		t.Helper()
		for _, rec := range recordings {
			res := b.do(rec.method, rec.path, rec.body)
			recordSetup(t, doc, rec, res)
		}
	}
	r.exec("TRUNCATE accounts CASCADE")
	play(
		setupRecording{"accounts-empty.json", http.MethodGet, "/api/accounts", "/api/accounts", nil},
		setupRecording{"setup-first-run.json", http.MethodGet, "/api/setup", "/api/setup", nil},
		setupRecording{
			"error-client-refused.json", http.MethodPost, "/api/setup/gmail/clients", "/api/setup/{provider}/clients",
			map[string]any{"name": household, "client_id": householdID, "client_secret": "wrong", "project_id": household},
		},
		setupRecording{
			"client-added.json", http.MethodPost, "/api/setup/gmail/clients", "/api/setup/{provider}/clients",
			map[string]any{"name": household, "client_id": householdID, "client_secret": householdSecret, "project_id": household},
		},
		setupRecording{"setup-client.json", http.MethodGet, "/api/setup", "/api/setup", nil},
		setupRecording{"connect-none.json", http.MethodGet, "/api/setup/connect", "/api/setup/connect", nil},
		setupRecording{
			"error-identifier-refused.json", http.MethodPost, "/api/setup/connect", "/api/setup/connect",
			map[string]any{"account": "setup", "client": household, "mailbox": "jo.smith@gmail.com", "lowered_target": nil},
		},
		// The account's identifier and mailbox carry markers, which no guide window may show
		// (VERIFICATIONS, the guide window row).
		setupRecording{
			"connect-started.json", http.MethodPost, "/api/setup/connect", "/api/setup/connect",
			map[string]any{"account": markedAccount, "client": household, "mailbox": markedMailbox, "lowered_target": nil},
		},
		setupRecording{"connect-attempt.json", http.MethodGet, "/api/setup/connect", "/api/setup/connect", nil},
	)
	attempt := decode[started](t, b.get("/api/setup/connect"))
	play(
		setupRecording{
			"error-wrong-mailbox.json", http.MethodPost, "/api/setup/connect/finish", "/api/setup/connect/finish",
			map[string]any{"address": r.grant(attempt, "someone.else@gmail.com")},
		},
		setupRecording{
			"connected.json", http.MethodPost, "/api/setup/connect/finish", "/api/setup/connect/finish",
			map[string]any{"address": r.grant(attempt, markedMailbox)},
		},
	)
	seed(t)
	b.addClient(spare, spareID, spareSecret)
	r.exec("UPDATE accounts SET oauth_client = $1", household)
	r.exec("UPDATE account_state SET mailbox = 'personal@gmail.com', lowered_target_rate = 0.3 WHERE account_id = $1", personal)
	play(
		setupRecording{"setup.json", http.MethodGet, "/api/setup", "/api/setup", nil},
		setupRecording{"account-personal.json", http.MethodGet, "/api/personal/account", "/api/{account}/account", nil},
		setupRecording{"account-other.json", http.MethodGet, "/api/other/account", "/api/{account}/account", nil},
		setupRecording{"reauthorize-none.json", http.MethodGet, "/api/personal/account/reauthorize", "/api/{account}/account/reauthorize", nil},
		setupRecording{
			"target-set.json", http.MethodPost, "/api/personal/account/target", "/api/{account}/account/target",
			map[string]any{"lowered_target": 0.25},
		},
	)
}

// recordSetup requires an answer to match its declaration in the contract document and compares it
// with its recorded file, which -update writes. An error's request id and an attempt's state are
// random, so a recording holds a fixed value in their place.
func recordSetup(t *testing.T, doc *openapi3.T, rec setupRecording, res response) {
	t.Helper()
	item := doc.Paths.Value(rec.pattern)
	if item == nil {
		t.Fatalf("the document has no path %s", rec.pattern)
	}
	op := item.GetOperation(rec.method)
	if op == nil {
		t.Fatalf("the document has no %s %s", rec.method, rec.pattern)
	}
	content := op.Responses.Status(res.status)
	if content == nil {
		t.Fatalf("%s %s answered %d, which the document does not declare:\n%s", rec.method, rec.path, res.status, res.body)
	}
	var value any
	if err := json.Unmarshal(res.body, &value); err != nil {
		t.Fatalf("%s: %v", rec.path, err)
	}
	if err := content.Value.Content.Get("application/json").Schema.Value.VisitJSON(value); err != nil {
		t.Errorf("%s %s does not match its declaration in the contract document: %v\n%s", rec.method, rec.path, err, res.body)
	}
	var pretty bytes.Buffer
	e := json.NewEncoder(&pretty)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	if err := e.Encode(fixRandom(t, value)); err != nil {
		t.Fatal(err)
	}
	compare.GoldenAt(t, filepath.Join("..", "..", "browser", "test", "fixtures", rec.name), pretty.Bytes())
}

// fixRandom replaces an error's request id, and an attempt's state with the state and the PKCE
// challenge its consent page's address carries, with fixed values.
func fixRandom(t *testing.T, value any) any {
	t.Helper()
	body, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if e, ok := body["error"].(map[string]any); ok {
		e["request_id"] = "recorded"
	}
	if a, ok := body["attempt"].(map[string]any); ok {
		a["attempt"] = recordedAttempt
		address, ok := a["consent_address"].(string)
		if !ok {
			t.Fatalf("the attempt carries no consent address: %v", a)
		}
		u, err := url.Parse(address)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("state", recordedAttempt)
		q.Set("code_challenge", "recorded-challenge")
		u.RawQuery = q.Encode()
		a["consent_address"] = u.String()
	}
	return body
}
