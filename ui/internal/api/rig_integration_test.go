//go:build integration

package api_test

import (
	"bytes"
	"context"
	"crypto/hpke"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail/consent"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// The OAuth client the setups' tests set up, as the provider fake recognises it.
const (
	household       = "household-mail"
	householdID     = "household-id.apps.example"
	householdSecret = "household-secret"
	spare           = "spare-client"
	spareID         = "spare-id.apps.example"
	spareSecret     = "spare-secret"
)

// rig is the UI's server over the seeded database as the UI's role, with the provider fake's consent
// standing for Gmail's, a public key whose private key the test holds, a clock the test moves, and the
// superuser's connection the test reads and writes the stored rows through.
type rig struct {
	t       *testing.T
	s       *api.Server
	consent *fake.Consent
	private hpke.PrivateKey
	public  seal.PublicKey
	db      *pgx.Conn
	clock   time.Time
	bundle  fstest.MapFS
	// redirect is the configured consent redirect.
	redirect string
	// keys are the key files the client-secret package loads, the public key and its private key.
	publicFile, privateFile string
	// log is everything the server logged.
	log bytes.Buffer
	// identity is who a policy write is recorded as made by, the operator name unless a test declares
	// an identity header.
	identity api.Identity
}

// defaultRedirect is the consent redirect the rig configures unless a test names another.
const defaultRedirect = "http://127.0.0.1:47823/"

// newRig seeds the database, removes every stored client, and builds the server.
func newRig(t *testing.T) *rig {
	t.Helper()
	return newRigAt(t, defaultRedirect)
}

// newRigAt is newRig with the consent redirect configured as redirect.
func newRigAt(t *testing.T, redirect string) *rig {
	t.Helper()
	seed(t)
	r := &rig{
		t: t, consent: fake.NewConsent(redirect), clock: now, redirect: redirect, identity: api.Identity{Operator: "operator"},
		bundle: fstest.MapFS{"main.js": {Data: []byte("")}, "main.css": {Data: []byte("")}, "fonts/a.woff2": {Data: []byte("")}, ".gitkeep": {Data: []byte("")}},
	}
	r.consent.AddClient(householdID, householdSecret)
	r.consent.AddClient(spareID, spareSecret)
	private, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	r.private = private
	r.public, err = seal.ParsePublicKey(private.PublicKey().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	seed, err := private.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r.publicFile, r.privateFile = filepath.Join(dir, "public.key"), filepath.Join(dir, "private.key")
	for path, b := range map[string][]byte{r.publicFile: r.public.Bytes(), r.privateFile: seed} {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.db, err = pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	r.exec("UPDATE accounts SET oauth_client = NULL")
	r.exec("DELETE FROM oauth_clients")
	r.s = r.server(tokenKey)
	return r
}

// server builds a server over the rig's database and fake with the given request token key, as a
// second replica of the UI would be built with the key its token_key_file names. A code exchange
// obtains its client's secret from the client-secret package, as the composition root wires it.
func (r *rig) server(key []byte) *api.Server {
	r.t.Helper()
	pool := uiPool(r.t, "mediated_mailbox_ui")
	secrets, err := clientsecret.Load(r.publicFile, []string{r.privateFile}, pool)
	if err != nil {
		r.t.Fatal(err)
	}
	s, err := api.New(api.Options{
		Bundle: r.bundle, Database: pool, Datasets: registry.Datasets(lookups),
		Logger: slog.New(slog.NewJSONHandler(&r.log, nil)), Metrics: prometheus.NewRegistry(),
		Clock: func() time.Time { return r.clock }, StreamInterval: time.Second, TokenKey: key, Seal: r.public,
		Consents: map[string]mail.Consent[context.Context]{"gmail": r.consent}, ClientSecrets: secrets,
		Browser: api.Browser{DefaultTheme: "system", StreamReconnectMax: time.Second, StreamPollInterval: time.Second, ConsentRedirect: r.redirect},
		Lookups: lookups, Identity: r.identity,
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return s
}

// exec runs a statement as the superuser.
func (r *rig) exec(sql string, args ...any) {
	r.t.Helper()
	if _, err := r.db.Exec(r.t.Context(), sql, args...); err != nil {
		r.t.Fatalf("%s: %v", sql, err)
	}
}

// count counts the rows a query returns, read as the superuser.
func (r *rig) count(sql string, args ...any) int {
	r.t.Helper()
	var n int
	if err := r.db.QueryRow(r.t.Context(), "SELECT count(*) FROM ("+sql+") AS q", args...).Scan(&n); err != nil {
		r.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// stored is everything the setups and policy management write, read as the superuser, so a refused
// request is checked to have written nothing anywhere.
func (r *rig) stored() string {
	r.t.Helper()
	var out string
	err := r.db.QueryRow(r.t.Context(), `SELECT concat_ws(' | ',
		(SELECT string_agg(concat_ws(',', name, provider, client_id, encode(client_secret, 'hex'), project_id), ';' ORDER BY name) FROM oauth_clients),
		(SELECT string_agg(concat_ws(',', account_id, provider, oauth_client), ';' ORDER BY account_id) FROM accounts),
		(SELECT string_agg(concat_ws(',', account_id, encode(credential, 'hex'), mailbox, lowered_target_rate, last_auth_at, last_auth_outcome), ';' ORDER BY account_id) FROM account_state),
		(SELECT string_agg(concat_ws(',', coalesce(account_id, '(base)'), rule_id, array_to_string(domain_suffix, ' '), source, created_by), ';' ORDER BY account_id NULLS FIRST, rule_id) FROM policy_rules),
		(SELECT string_agg(concat_ws(',', id, coalesce(account_id, '(base)'), actor, action, rule_id, array_to_string(suffixes_before, ' '), array_to_string(suffixes_after, ' ')), ';' ORDER BY id) FROM policy_changes))`).Scan(&out)
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

// open opens a sealed value with the test's private key, through the construction the credential
// library documents, since the UI's list admits no code that opens one (ADR-0088).
func (r *rig) open(sealed []byte, c seal.Context) (string, error) {
	parts, err := seal.Split(sealed)
	if err != nil {
		return "", err
	}
	recipient, err := hpke.NewRecipient(parts.EncapsulatedKey, r.private, seal.KDF(), seal.AEAD(), seal.Info())
	if err != nil {
		return "", err
	}
	plain, err := recipient.Open(seal.AdditionalData(seal.Version, parts.KeyID, c), parts.Ciphertext)
	return string(plain), err
}

// sealed reads one bytea column of one row as the superuser.
func (r *rig) sealed(sql string, args ...any) []byte {
	r.t.Helper()
	var b []byte
	if err := r.db.QueryRow(r.t.Context(), sql, args...).Scan(&b); err != nil {
		r.t.Fatalf("%s: %v", sql, err)
	}
	return b
}

// browser is one browser session against a server, holding its cookies and the request token its
// entry document carried.
type browser struct {
	t       *testing.T
	h       http.Handler
	cookies map[string]string
	token   string
	// header holds headers every request carries, as an authenticating proxy in front sets them.
	header map[string]string
}

var tokenMeta = regexp.MustCompile(`<meta name="mediated-mailbox.request_token" content="([^"]*)">`)

// browser loads the entry document in a new session, keeping the session cookie and the token.
func (r *rig) browser() *browser {
	r.t.Helper()
	b := &browser{t: r.t, h: r.s.Handler(), cookies: map[string]string{}}
	res := b.do(http.MethodGet, "/setup", nil)
	m := tokenMeta.FindSubmatch(res.body)
	if m == nil || b.cookies["ui_session"] == "" {
		r.t.Fatalf("the entry document carried no token or no session:\n%s", res.body)
	}
	b.token = string(m[1])
	return b
}

// on is the same browser session against another server, as when another replica answers.
func (b *browser) on(s *api.Server) *browser {
	return &browser{t: b.t, h: s.Handler(), cookies: b.cookies, token: b.token}
}

func (b *browser) do(method, path string, body any) response {
	b.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			b.t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequestWithContext(b.t.Context(), method, path, reader)
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value}) //nolint:gosec // a request's cookie is a name and a value, its attributes the response's that set it
	}
	if method != http.MethodGet && b.token != "" {
		req.Header.Set("X-Request-Token", b.token)
	}
	for name, value := range b.header {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	b.h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	return response{status: rec.Code, header: rec.Header(), body: rec.Body.Bytes()}
}

func (b *browser) post(path string, body any) response { return b.do(http.MethodPost, path, body) }

func (b *browser) get(path string) response { return b.do(http.MethodGet, path, nil) }

// decode reads a 200 answer's body, failing the test for any other answer.
func decode[T any](t *testing.T, r response) T {
	t.Helper()
	if r.status != http.StatusOK {
		t.Fatalf("answered %d, want 200:\n%s", r.status, r.body)
	}
	var v T
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("%v:\n%s", err, r.body)
	}
	return v
}

// refused requires the answer to be the error contract's code with its status.
func refused(t *testing.T, r response, status int, code string) {
	t.Helper()
	if r.status != status {
		t.Fatalf("answered %d, want %d %s:\n%s", r.status, status, code, r.body)
	}
	if _, got := errorOf(t, r); got != code {
		t.Fatalf("answered %s, want %s:\n%s", got, code, r.body)
	}
}

type started struct {
	Attempt struct {
		Attempt        string `json:"attempt"`
		Kind           string `json:"kind"`
		Account        string `json:"account"`
		Mailbox        string `json:"mailbox"`
		Client         string `json:"client"`
		ExpiresAt      string `json:"expires_at"`
		ConsentAddress string `json:"consent_address"`
	} `json:"attempt"`
}

// addClient sets up a client through the UI, failing the test unless it is stored.
func (b *browser) addClient(name, id, secret string) {
	b.t.Helper()
	project := name
	decode[map[string]any](b.t, b.post("/api/setup/gmail/clients", map[string]any{"name": name, "client_id": id, "client_secret": secret, "project_id": project}))
}

// connect starts an attempt to connect the account through the client, failing the test unless it
// starts, and returns what the page shows of it.
func (b *browser) connect(account, client, mailbox string) started {
	b.t.Helper()
	return decode[started](b.t, b.post("/api/setup/connect", map[string]any{"account": account, "client": client, "mailbox": mailbox, "lowered_target": nil}))
}

// grant plays the person granting the attempt's consent at the provider, signed in as mailbox, and
// returns the address the browser is redirected to.
func (r *rig) grant(a started, mailbox string) string {
	r.t.Helper()
	address, err := r.consent.Grant(a.Attempt.ConsentAddress, mailbox, consent.Scope)
	if err != nil {
		r.t.Fatal(err)
	}
	return address
}

// connected connects the account through the client end to end, failing the test unless it is.
func (r *rig) connected(b *browser, account, client, mailbox string) {
	r.t.Helper()
	a := b.connect(account, client, mailbox)
	decode[map[string]any](r.t, b.post("/api/setup/connect/finish", map[string]any{"address": r.grant(a, mailbox)}))
}

// pasted is a redirect address with its query replaced, for the refusals a pasted address can meet.
func pasted(t *testing.T, address string, change func(url.Values)) string {
	t.Helper()
	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	change(q)
	u.RawQuery = q.Encode()
	return u.String()
}
