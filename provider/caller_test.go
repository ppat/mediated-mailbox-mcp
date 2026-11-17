package provider_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	coremail "github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/provider"
)

// A caller names one of the three classes and a positive timeout, and has a pool for its leases, so
// no call is leased in no class or made with no bound on its answer.
func TestACallerIsRefusedWithoutAClassATimeoutOrAPool(t *testing.T) {
	var pool tx.Beginner = &pgxpool.Pool{}
	for name, c := range map[string]struct {
		o    provider.Options
		pool tx.Beginner
	}{
		"no class":           {provider.Options{Timeout: time.Second}, pool},
		"a class past Batch": {provider.Options{Class: provider.Batch + 1, Timeout: time.Second}, pool},
		"no timeout":         {provider.Options{Class: provider.Batch}, pool},
		"a negative timeout": {provider.Options{Class: provider.Batch, Timeout: -time.Second}, pool},
		"no pool":            {provider.Options{Class: provider.Batch, Timeout: time.Second}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := provider.NewCaller(provider.Shipped(http.DefaultClient), prometheus.NewRegistry(), c.pool, c.o); err == nil {
				t.Error("NewCaller succeeded")
			}
		})
	}
	if _, err := provider.NewCaller(provider.Shipped(http.DefaultClient), prometheus.NewRegistry(), pool, provider.Options{Class: provider.Interactive, Timeout: time.Second}); err != nil {
		t.Errorf("NewCaller with a class, a timeout and a pool: %v", err)
	}
}

// A mailbox is built only for a provider a shipped adapter serves, over a token made for that
// provider, and a token is made only for such a provider.
func TestAMailboxNeedsAShippedAdapterAndItsToken(t *testing.T) {
	adapters := provider.Shipped(http.DefaultClient)
	c, err := provider.NewCaller(adapters, prometheus.NewRegistry(), &pgxpool.Pool{}, provider.Options{Class: provider.Batch, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapters.NewToken("jmap", coremail.OAuthClient{}, []byte("credential")); err == nil {
		t.Error("NewToken made a token for a provider no adapter serves")
	}
	token, err := adapters.NewToken("gmail", coremail.OAuthClient{ID: "id", Secret: "secret"}, []byte("refresh-token"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(token.Credential()); got != "refresh-token" {
		t.Errorf("the token holds the credential %q, want the one it was made from", got)
	}
	if got := token.LastAttempt(); got != (coremail.AuthAttempt{}) {
		t.Errorf("a token that made no request reports the attempt %+v", got)
	}
	for name, conn := range map[string]provider.Connection{
		"no adapter": {Provider: "jmap", Account: "you", Token: token},
		"no token":   {Provider: "gmail", Account: "you"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Mailbox(conn); err == nil {
				t.Error("Mailbox succeeded")
			}
		})
	}
	m, err := c.Mailbox(provider.Connection{Provider: "gmail", Account: "you", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	// Gmail prices a metadata fetch at 20 units per message and a read of the label table, so three
	// fit the hard cap's 80 units, and a caller asking for two names two.
	if got := m.MetadataCallSize(50); got != 3 {
		t.Errorf("MetadataCallSize(50) = %d, want 3", got)
	}
	if got := m.MetadataCallSize(2); got != 2 {
		t.Errorf("MetadataCallSize(2) = %d, want 2", got)
	}
}

// The facts about a provider are known only for one a shipped adapter serves. Gmail's accounts
// connect through an OAuth client, and its ceiling is the budget its rate profile declares, 100 units
// a second (ADR-0023, ADR-0106).
func TestTheFactsAboutEachProvider(t *testing.T) {
	if got, ok := provider.FactsOf("gmail"); !ok || got != (provider.Facts{UsesClient: true, Ceiling: 100}) {
		t.Errorf("FactsOf(gmail) = %+v, %v", got, ok)
	}
	if got, ok := provider.FactsOf("jmap"); ok {
		t.Errorf("FactsOf(jmap) = %+v, want no facts for a provider no adapter serves", got)
	}
}
