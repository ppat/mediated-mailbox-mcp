package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/provider/consent"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/clientsecret"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// providerTimeout bounds one request to a provider, checking a client, exchanging a consent's code or
// reading which mailbox granted it.
const providerTimeout = 30 * time.Second

// New builds the read API endpoint, which the UI's spec lists at /ui/api/ (ADR-0142). It loads the
// public key the UI seals to and logs its identifier before the database is needed, so the key the UI
// would seal to is in the log whatever the database answers (ADR-0092). It then gets the process's
// pool, whose schema check refuses a schema the binary cannot run on (ADR-0140), and starts the
// readiness report the UI is ready by, the database answering a ping. It gets the checks every
// endpoint sits behind, loads the client-secret opener and Gmail's consent, and returns the read API
// behind the checks. Its event streams end once the process starts to stop.
func New(ctx context.Context, p process.Capabilities, c config.UI) (http.Handler, error) {
	sealKey, err := seal.LoadPublicKey(c.Credential.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading the public key: %w", err)
	}
	p.Logger().Info("sealing to the public key", "key_id", sealKey.KeyID().String())
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	if err := database.ReportReadiness(ctx, p, c); err != nil {
		return nil, err
	}
	checks, err := server.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	// The sender classifier's domain functions, which policy management matches senders against
	// rules with, as the deployables that classify pass them (ADR-0004).
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	secrets, err := clientsecret.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles, pool)
	if err != nil {
		return nil, fmt.Errorf("loading the keyring: %w", err)
	}
	gmailConsent, err := consent.For("gmail", &http.Client{Timeout: providerTimeout}, c.ConsentRedirect)
	if err != nil {
		return nil, err
	}
	s, err := newServer(Options{
		Database:       pool,
		Datasets:       registry.Datasets(lookups),
		Checks:         checks,
		Metrics:        p.Metrics(),
		Draining:       p.Draining(),
		Clock:          time.Now,
		Cadences:       Cadences{Sync: time.Duration(c.SyncInterval), Heuristics: time.Duration(c.HeuristicsInterval)},
		StreamInterval: time.Duration(c.StreamInterval),
		Attention: attention.Thresholds{
			BacklogShare: c.AttentionBacklogShare, MaskCount: c.AttentionMaskCount,
			ServeFactor: c.AttentionServeFactor, GapDays: c.AttentionGapDays,
		},
		Seal: sealKey,
		Consents: map[string]mail.Consent[context.Context]{
			"gmail": gmailConsent,
		},
		ClientSecrets: secrets,
		Lookups:       lookups,
		Identity:      Identity{Header: c.IdentityHeader, Operator: c.OperatorName},
	})
	if err != nil {
		return nil, err
	}
	return s.handler, nil
}
