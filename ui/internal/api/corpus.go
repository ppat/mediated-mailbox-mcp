package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/maskingevents"
	messageclassification "github.com/ppat/mediated-mailbox-mcp/db/messages/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// auditActions are the audit actions as stored (ADR-0016), which the corpus endpoint counts, each
// present with zero when no row has it.
func auditActions() []string { return []string{"READ_BODY", "DENY_BODY", "MUTATE", "DENY_MUTATE"} }

// corpusResponse is Home's at-a-glance corpus figures (docs/UI.md sections 8.1 and 17.4), with the
// time the read transaction began, since Home reads them apart from the system endpoint and shows the
// older of the two times. Audit24h counts the audit rows of the last 24 hours by action as stored. It counts every message of the
// account, so it is a read apart from the system endpoint, which the partial-index banner reads again
// on every backfill run event, and only Home reads it (docs/UI.md section 9).
type corpusResponse struct {
	Account         string           `json:"account"`
	AsOf            string           `json:"as_of"`
	Messages        int64            `json:"messages"`
	Threads         int64            `json:"threads"`
	Unfiled         int64            `json:"unfiled"`
	Restricted      int64            `json:"restricted"`
	Audit24h        map[string]int64 `json:"audit_24h"`
	MaskingEvents7d int64            `json:"masking_events_7d"`
	GateSkips7d     int64            `json:"gate_skips_7d"`
}

func corpusType() schema.Type {
	return schema.Obj("Corpus",
		schema.F("account", schema.Str()),
		schema.F("as_of", schema.Time()),
		schema.F("messages", schema.Int()),
		schema.F("threads", schema.Int()),
		schema.F("unfiled", schema.Int()),
		schema.F("restricted", schema.Int()),
		schema.F("audit_24h", schema.MapOf(schema.Int())),
		schema.F("masking_events_7d", schema.Int()),
		schema.F("gate_skips_7d", schema.Int()),
	)
}

// getCorpus answers the corpus endpoint in one read transaction for the account.
func (s *Server) getCorpus(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	out := corpusResponse{Account: account, Audit24h: map[string]int64{}}
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		ctx := r.Context()
		// as_of is the time the read transaction began.
		now := s.opts.Clock()
		out.AsOf = registry.Stamp(now)
		day := pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true}
		week := pgtype.Timestamptz{Time: now.Add(-7 * 24 * time.Hour), Valid: true}
		corpus, err := messageclassification.New(t).CorpusFigures(ctx)
		if err != nil {
			return err
		}
		out.Messages, out.Threads, out.Unfiled, out.Restricted = corpus.Messages, corpus.Threads, corpus.Unfiled, corpus.Restricted
		actions, err := auditlog.New(t).AuditActionCounts(ctx, day)
		if err != nil {
			return err
		}
		for _, a := range auditActions() {
			out.Audit24h[a] = 0
		}
		for _, row := range actions {
			out.Audit24h[row.Action] = row.Entries
		}
		if out.MaskingEvents7d, err = maskingevents.New(t).MaskingEventCount(ctx, week); err != nil {
			return err
		}
		out.GateSkips7d, err = scangatedecisions.New(t).GateDecisionCount(ctx, scangatedecisions.GateDecisionCountParams{
			Decision: "SKIP", Since: week,
		})
		return err
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}
