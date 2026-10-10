//go:build integration

package session_test

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// role is backfill's role, a job kind whose sessions hold their sources across units of work, so
// row-level security and its grants apply as they do in production (ADR-0118).
const role = "mediated_mailbox_backfill"

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// provider is the provider's side of the tests' token sources. It records the OAuth client of every
// token it issues, and of every call it serves.
type provider struct {
	mu      sync.Mutex
	issued  []string
	served  []string
	rotated []byte
}

// source is a token source over provider. Its first call asks the provider for an access token with
// its client, and the provider rotates the refresh token it holds to provider.rotated when that is set.
type source struct {
	p          *provider
	client     string
	credential []byte
	token      bool
}

func (s *source) Credential() []byte            { return s.credential }
func (s *source) LastAttempt() mail.AuthAttempt { return mail.AuthAttempt{} }

// port is the Provider Port over one source. Only the cursor read is called here.
type port struct {
	mail.Port[context.Context]
	s *source
}

func (p port) CurrentCursor(context.Context) (mail.Cursor, error) {
	pr := p.s.p
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if !p.s.token {
		pr.issued = append(pr.issued, p.s.client)
		p.s.token = true
		if pr.rotated != nil {
			p.s.credential = pr.rotated
		}
	}
	pr.served = append(pr.served, p.s.client)
	return "cursor", nil
}

// unit opens the account's session over the loader's active snapshot, makes one call and ends the
// unit, handing the source's credential over, as each page of a backfill run does.
func unit(t *testing.T, h *session.Holder, loader *accountload.Loader, account string) {
	t.Helper()
	s, err := h.Open(loader.Snapshot(), account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Call(t.Context(), s, func(ctx context.Context, p mail.Port[context.Context]) (mail.Cursor, error) {
		return p.CurrentCursor(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	if handOver, record := s.End(t.Context()); handOver != nil || record != nil {
		t.Fatalf("ending the unit: %v, %v", handOver, record)
	}
}

// A holder keeps an account's source across units of work while the stored credential is the one the
// source was built from or the one it rotated to, and for the same OAuth client only. A source that
// rotated its refresh token serves the units after the rotation's write-back with the access token it
// already holds, so the provider issues none again, and an account moved to another client is served
// by a source of that client from its next unit, the same credential notwithstanding (ADR-0089,
// ADR-0106).
func TestAHeldSourceServesItsAccountUntilItsCredentialOrClientChanges(t *testing.T) {
	ctx := t.Context()
	conn, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	private, err := open.ParsePrivateKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := open.NewKeyring(private.PublicKey(), private)
	if err != nil {
		t.Fatal(err)
	}
	sealed := func(plaintext string, c seal.Context) []byte {
		b, err := private.PublicKey().Seal([]byte(plaintext), c)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', $1)",
		sealed("household-secret", seal.ClientSecret("household")))
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('employer', 'gmail', 'employer-id', $1)",
		sealed("employer-secret", seal.ClientSecret("employer")))
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('personal', 'gmail', 'household')")
	must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ('personal', $1)", sealed("first-token", seal.AccountCredential("personal")))

	db := pool(t)
	loader := accountload.New(db, keys, slog.New(slog.DiscardHandler), []string{"gmail"})
	if err := loader.Load(ctx); err != nil {
		t.Fatal(err)
	}
	p := &provider{rotated: []byte("second-token")}
	connect := session.Connect(func(c session.Credentials) *source {
		return &source{p: p, client: c.ClientID, credential: c.Credential}
	}, func(_ string, s *source) (mail.Port[context.Context], error) { return port{s: s}, nil })
	h := session.New(session.Config{
		Loader: loader, DB: db, Logger: slog.New(slog.DiscardHandler),
		Connectors: map[string]session.Connector{"gmail": connect}, Hold: true,
	})

	// The first unit's source rotates the refresh token, and the unit's end writes it back. The reload
	// after it reads the rotated token, the one the held source now holds, so the next unit keeps it.
	unit(t, h, loader, "personal")
	if err := loader.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if a, _ := loader.Snapshot().Account("personal"); string(a.Credential()) != "second-token" {
		t.Fatalf("the reload after the rotation holds %q, want the rotated token", a.Credential())
	}
	unit(t, h, loader, "personal")
	if diff := cmp.Diff([]string{"household-id"}, p.issued, compare.Options); diff != "" {
		t.Errorf("the access tokens issued over the rotation, one for the source the units shared (-want +got):\n%s", diff)
	}

	// The account moves to the employer's client keeping its token, so the next unit's source is one of
	// that client, and the provider serves it as the employer's.
	must(t, conn, "UPDATE accounts SET oauth_client = 'employer' WHERE account_id = 'personal'")
	if err := loader.Load(ctx); err != nil {
		t.Fatal(err)
	}
	unit(t, h, loader, "personal")
	if diff := cmp.Diff([]string{"household-id", "employer-id"}, p.issued, compare.Options); diff != "" {
		t.Errorf("the access tokens issued once the account moved client (-want +got):\n%s", diff)
	}
	if last := p.served[len(p.served)-1]; last != "employer-id" {
		t.Errorf("the provider served the unit after the move as %q, want the employer's client", last)
	}
	if !slices.Equal(p.served, []string{"household-id", "household-id", "employer-id"}) {
		t.Errorf("the provider served the units as %v", p.served)
	}
}
