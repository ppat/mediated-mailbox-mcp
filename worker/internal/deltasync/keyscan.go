package deltasync

import (
	"maps"
	"slices"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/connections"
)

// The scan series key replacement waits on, named in ADR-0103 for what they measure, the stored values
// still sealed to a key other than the current one. The procedure of ADR-0092 reads them, so they stay
// exactly as written.
const (
	credentialName = "mediated_mailbox_credential_on_old_key"
	clientName     = "mediated_mailbox_client_secret_on_old_key"
)

// keyScan is the scan series on delta sync's registerer in the worker, which runs until stopped, so
// every series stays in each scrape between listings (ADR-0103).
type keyScan struct {
	credentials *prometheus.GaugeVec
	clients     *prometheus.GaugeVec
	accounts    []string
	clientNames []string
}

// newKeyScan registers the scan series on reg and returns them.
func newKeyScan(reg prometheus.Registerer) (*keyScan, error) {
	m := &keyScan{
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
func (m *keyScan) Set(s connections.Scan) {
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
