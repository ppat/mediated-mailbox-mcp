//go:build integration

package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// Every state-changing route of the contract document, read from the document rather than from the
// server's lists, so a route added later is covered. Each is sent from a page on another origin, which
// carries the session cookie and no token, from a session whose token another session's page holds,
// and with no session at all, and each must be refused with stale_page before it writes anything
// (ADR-0061, VERIFICATIONS, the request token row). A token the session's own page holds passes the
// check, so the refusals are the token's doing.
func TestAStateChangingRequestWithoutItsTokenIsRefused(t *testing.T) {
	r := newRig(t)
	owner := r.browser()
	owner.addClient(household, householdID, householdSecret)
	before := r.stored()
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var routes []string
	for path, item := range doc.Paths.Map() {
		if item.Post != nil {
			routes = append(routes, strings.NewReplacer("{account}", personal, "{provider}", "gmail", "{client}", household).Replace(path))
		}
	}
	slices.Sort(routes)
	if len(routes) < 8 {
		t.Fatalf("the document holds %d state-changing routes, fewer than the setups alone", len(routes))
	}
	body := map[string]any{"name": spare, "client_id": spareID, "client_secret": spareSecret, "project_id": nil}
	for _, path := range routes {
		forged := &browser{t: t, h: r.s.Handler(), cookies: owner.cookies}
		refused(t, forged.post(path, body), http.StatusForbidden, "stale_page")
		other := r.browser()
		other.token = owner.token
		refused(t, other.post(path, body), http.StatusForbidden, "stale_page")
		bare := &browser{t: t, h: r.s.Handler(), cookies: map[string]string{}, token: owner.token}
		refused(t, bare.post(path, body), http.StatusForbidden, "stale_page")
	}
	if got := r.stored(); got != before {
		t.Errorf("a refused request wrote:\nbefore %s\nafter  %s", before, got)
	}
	if res := owner.post("/api/setup/gmail/clients", body); res.status == http.StatusForbidden {
		t.Errorf("the session's own token was refused:\n%s", res.body)
	}
}

// A client Google refuses is not stored, and one it accepts is stored apart from every account, its
// secret sealed to the row its name keys, opening with the private key (ADR-0080, ADR-0081, ADR-0107,
// VERIFICATIONS, the client check row). A client Google does not answer for is not stored either.
func TestAClientIsCheckedBeforeItIsStored(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	before := r.stored()
	refused(t, b.post("/api/setup/gmail/clients", map[string]any{"name": household, "client_id": householdID, "client_secret": "wrong", "project_id": nil}),
		http.StatusBadRequest, "client_refused")
	refused(t, b.post("/api/setup/gmail/clients", map[string]any{"name": household, "client_id": "unknown.apps.example", "client_secret": householdSecret, "project_id": nil}),
		http.StatusBadRequest, "client_refused")
	r.consent.SetUnreachable(true)
	refused(t, b.post("/api/setup/gmail/clients", map[string]any{"name": household, "client_id": householdID, "client_secret": householdSecret, "project_id": nil}),
		http.StatusBadGateway, "provider_unreachable")
	r.consent.SetUnreachable(false)
	if got := r.stored(); got != before {
		t.Fatalf("a refused client was stored:\n%s", got)
	}
	b.addClient(household, householdID, householdSecret)
	if n := r.count("SELECT 1 FROM oauth_clients WHERE name = $1 AND provider = 'gmail' AND client_id = $2 AND project_id = $1", household, householdID); n != 1 {
		t.Fatalf("the client is stored %d times", n)
	}
	sealed := r.sealed("SELECT client_secret FROM oauth_clients WHERE name = $1", household)
	if strings.Contains(string(sealed), householdSecret) {
		t.Errorf("the stored secret holds the secret in plaintext")
	}
	if got, err := r.open(sealed, seal.ClientSecret(household)); err != nil || got != householdSecret {
		t.Errorf("the stored secret opened to %q, %v", got, err)
	}
	if _, err := r.open(sealed, seal.ClientSecret(spare)); err == nil {
		t.Errorf("the secret opened bound to another row")
	}
	if got := r.stored(); strings.Count(got, personal) != strings.Count(before, personal) {
		t.Errorf("setting up a client touched an account's rows:\n%s", got)
	}
}

// A new client under a name another client holds, under new, or under a name holding a dot, a slash
// or a capital is refused, and a client identifier set up a second time under another name is refused,
// each storing nothing (ADR-0106, docs/UI.md section 8.11, VERIFICATIONS, the client name row).
func TestAClientsNameReachesOneClient(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	before := r.stored()
	add := func(name, id, secret string) response {
		return b.post("/api/setup/gmail/clients", map[string]any{"name": name, "client_id": id, "client_secret": secret, "project_id": nil})
	}
	refused(t, add(household, spareID, spareSecret), http.StatusConflict, "name_taken")
	for _, name := range []string{"new", "spare.client", "spare/client", "Spare-client"} {
		refused(t, add(name, spareID, spareSecret), http.StatusBadRequest, "name_refused")
	}
	refused(t, add(spare, householdID, householdSecret), http.StatusConflict, "client_exists")
	if got := r.stored(); got != before {
		t.Errorf("a refused client was stored:\nbefore %s\nafter  %s", before, got)
	}
}

// Replacing a client's secret keeps its identifier, and replacing its identifier is checked as a new
// client is. A client no account connects through is removed, and one an account connects through is
// not (ADR-0106, VERIFICATIONS, the client removal row).
func TestAClientIsReplacedAndRemoved(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	b.addClient(spare, spareID, spareSecret)
	r.consent.AddClient(householdID, "household-secret-2")
	decode[map[string]any](t, b.post("/api/setup/gmail/clients/"+household, map[string]any{"client_id": householdID, "client_secret": "household-secret-2", "project_id": nil}))
	if got, err := r.open(r.sealed("SELECT client_secret FROM oauth_clients WHERE name = $1", household), seal.ClientSecret(household)); err != nil || got != "household-secret-2" {
		t.Errorf("the replaced secret opened to %q, %v", got, err)
	}
	refused(t, b.post("/api/setup/gmail/clients/"+household, map[string]any{"client_id": spareID, "client_secret": spareSecret, "project_id": nil}),
		http.StatusConflict, "client_exists")
	refused(t, b.post("/api/setup/gmail/clients/absent-client", map[string]any{"client_id": "x", "client_secret": "y", "project_id": nil}),
		http.StatusNotFound, "unknown_client")
	r.connected(b, "household", household, "jo@example.com")
	before := r.stored()
	refused(t, b.post("/api/setup/gmail/clients/"+household+"/remove", nil), http.StatusConflict, "client_in_use")
	if got := r.stored(); got != before {
		t.Errorf("a refused removal changed:\nbefore %s\nafter  %s", before, got)
	}
	decode[map[string]any](t, b.post("/api/setup/gmail/clients/"+spare+"/remove", nil))
	if n := r.count("SELECT 1 FROM oauth_clients WHERE name = $1", spare); n != 0 {
		t.Errorf("the unused client is still stored")
	}
	refused(t, b.post("/api/setup/gmail/clients/"+spare+"/remove", nil), http.StatusNotFound, "unknown_client")
}

// The installation endpoint and every installation screen route answer with no account's state. Two
// accounts' state rows carry marker values in their mailbox, a last authentication time and a lowered
// target no other value holds, and none reaches any answer, which holds each client's name, provider,
// identifier and project ID and each account's identifier, provider and client (ADR-0056, VERIFICATIONS,
// the installation row). A last authentication's outcome is one of the schema's closed words, which an
// answer may hold for a reason of its own, so the time stored beside it stands for the pair.
func TestTheInstallationShowsNoAccountsState(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	b.addClient(household, householdID, householdSecret)
	r.exec("UPDATE accounts SET oauth_client = $1", household)
	r.exec(`UPDATE account_state SET mailbox = $1, last_auth_outcome = 'refused', last_auth_at = '2001-02-03T04:05:06Z', lowered_target_rate = 0.4242`,
		marker.Field("mailbox"))
	r.exec(`INSERT INTO account_state (account_id, mailbox, last_auth_outcome, last_auth_at, lowered_target_rate) VALUES ($1, $2, 'failed', '2001-02-03T04:05:07Z', 0.4242)`,
		other, marker.Field("othermailbox"))
	var answers []string
	for _, path := range []string{"/api/setup", "/api/setup/connect", "/setup", "/setup/connect", "/setup/gmail/new", "/setup/gmail/" + household} {
		res := b.get(path)
		if res.status != http.StatusOK {
			t.Fatalf("%s answered %d:\n%s", path, res.status, res.body)
		}
		answers = append(answers, string(res.body))
	}
	for _, answer := range answers {
		for _, absent := range []string{marker.Field("mailbox"), marker.Field("othermailbox"), "0.4242", "2001-02-03"} {
			if strings.Contains(answer, absent) {
				t.Errorf("an installation answer holds %q:\n%s", absent, answer)
			}
		}
	}
	got := decode[struct {
		Providers []string `json:"providers"`
		Clients   []struct {
			Name     string   `json:"name"`
			Accounts []string `json:"accounts"`
		} `json:"clients"`
		Accounts []map[string]any `json:"accounts"`
	}](t, b.get("/api/setup"))
	if !slices.Equal(got.Providers, []string{"gmail"}) || len(got.Clients) != 1 || !slices.Equal(got.Clients[0].Accounts, []string{other, personal}) || len(got.Accounts) != 2 {
		t.Errorf("the installation reads %+v", got)
	}
}

// The words the UI's own top-level paths use are the router's setup, the read API's api and the name
// of every file and directory at the top of the bundle, apart from a dot file, which is not served
// (docs/UI.md section 8.12). The bundle the binary embeds is read here, when it holds a build.
func TestTheReservedWordsAreTheBundlesTopLevel(t *testing.T) {
	dist := filepath.Join("..", "..", "browser", "dist")
	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api", "setup"}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			want = append(want, e.Name())
		}
	}
	got, err := api.Reserved(os.DirFS(dist))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the reserved words are %v, want %v", got, want)
	}
}
