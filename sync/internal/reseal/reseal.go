// Package reseal is delta sync's part of key replacement. It writes an OAuth client's re-sealed
// secret through the one statement that writes a client secret, which only delta sync's role is
// granted, and it emits the scan series key replacement waits on (ADR-0092, ADR-0103). The re-seal
// itself is accountload's.
package reseal

import (
	"context"
	"maps"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients/secret"
)

// DB is what the writer writes through, a pool in delta sync. oauth_clients belongs to no account, so
// the write runs outside an account's transaction (ADR-0016).
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Writer returns the ClientWriter accountload's re-seal of a client secret writes through, a
// compare-and-set on the bytes it knew (ADR-0089).
func Writer(db DB) accountload.ClientWriter {
	return func(ctx context.Context, name string, known, sealed []byte) (bool, error) {
		var n int64
		err := pgx.BeginFunc(ctx, db, func(t pgx.Tx) error {
			var err error
			n, err = secret.New(t).ReplaceSealedClient(ctx, secret.ReplaceSealedClientParams{ClientSecret: sealed, ClientName: name, Known: known})
			return err
		})
		return n == 1, err
	}
}

// The scan series key replacement waits on, named in ADR-0103. The procedure of ADR-0092 reads them,
// so they stay exactly as written.
const (
	credentialName = "mediated_mailbox_sync_credential_on_old_key"
	clientName     = "mediated_mailbox_sync_client_secret_on_old_key"
)

// Metrics are the scan series in one process, which runs until stopped, so every series stays in
// each scrape between ticks (ADR-0103).
type Metrics struct {
	credentials *prometheus.GaugeVec
	clients     *prometheus.GaugeVec
	accounts    []string
	clientNames []string
}

// NewMetrics registers the scan series on reg and returns them.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		credentials: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: credentialName,
			Help: "1 while the account's stored credential is sealed to a key other than the current one or cannot be opened, 0 once it is sealed to the current key or when the account holds none.",
		}, []string{"account"}),
		clients: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: clientName,
			Help: "1 while the OAuth client's stored secret is sealed to a key other than the current one or cannot be opened, 0 once it is sealed to the current key.",
		}, []string{"client"}),
	}
	for _, c := range []prometheus.Collector{m.credentials, m.clients} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Set sets one series for every account and every OAuth client the scan holds, the client's labelled
// by its name, and removes the series of an account or client an earlier scan held and this one does
// not, so a series is reported exactly for what the last load listed (ADR-0092, ADR-0103).
func (m *Metrics) Set(s accountload.Scan) {
	m.accounts = set(m.credentials, m.accounts, s.Accounts)
	m.clientNames = set(m.clients, m.clientNames, s.Clients)
}

func set(g *prometheus.GaugeVec, before []string, now map[string]bool) []string {
	for _, k := range before {
		if _, ok := now[k]; !ok {
			g.DeleteLabelValues(k)
		}
	}
	for k, old := range now {
		v := 0.0
		if old {
			v = 1
		}
		g.WithLabelValues(k).Set(v)
	}
	return slices.Sorted(maps.Keys(now))
}
