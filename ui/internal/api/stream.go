package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// The stream's event names (docs/UI.md section 17.5).
const (
	eventRun  = "run"
	eventRate = "rate"
	eventPlan = "plan"
)

// runEvent is a run's whole current state, with its account.
type runEvent struct {
	Account string `json:"account"`
	run
}

// rateEvent is the rate state's whole current state, with its account.
type rateEvent struct {
	Account string `json:"account"`
	rateBlock
}

// planEvent is an applying plan's progress, the op log's rows against the plan's operations.
type planEvent struct {
	Account string `json:"account"`
	PlanID  string `json:"plan_id"`
	Status  string `json:"status"`
	Applied int64  `json:"applied"`
	Of      int64  `json:"of"`
}

func eventTypes() map[string]schema.Type {
	withAccount := func(name string, t schema.Type) schema.Type {
		return schema.Obj(name, append([]schema.Field{schema.F("account", schema.Str())}, t.Fields...)...)
	}
	return map[string]schema.Type{
		eventRun:  withAccount("RunEvent", runType()),
		eventRate: withAccount("RateEvent", rateType()),
		eventPlan: schema.Obj("PlanEvent",
			schema.F("account", schema.Str()),
			schema.F("plan_id", schema.Str()),
			schema.F("status", schema.Str("DRAFT", "APPROVED", "APPLYING", "APPLIED", "ROLLED_BACK", "REJECTED", "APPLY_REFUSED")),
			schema.F("applied", schema.Int()),
			schema.F("of", schema.Int()),
		),
	}
}

// streamEvents is the live stream, one per account (ADR-0058). It polls the recorded state every
// StreamInterval and sends one event per object whose state differs from what this stream last sent
// it, with the object's whole current state, never a delta, so a missed event costs nothing. The first
// poll sends every object it reads, so a subscriber that connects or reconnects starts from the
// current state. The objects are the runs that are running or finished since the previous poll, the
// rate state, and the plans applying, each sent once more when it leaves APPLYING. A poll that
// changed nothing writes a comment instead, so the stream is never idle. A failed poll ends the
// stream, and the browser reconnects.
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := controller.Flush(); err != nil {
		requestInfo(r.Context()).err = err
		return
	}
	s.metrics.subscribers.Inc()
	defer s.metrics.subscribers.Dec()

	st := &streamState{sent: map[string][]byte{}, known: map[string]bool{}, since: s.opts.Clock()}
	ticker := time.NewTicker(s.opts.StreamInterval)
	defer ticker.Stop()
	for {
		events, err := s.poll(r.Context(), account, st)
		if err != nil {
			if r.Context().Err() == nil {
				requestInfo(r.Context()).err = err
				requestInfo(r.Context()).origin = originDatabase
			}
			return
		}
		for _, e := range events {
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, e.data); err != nil {
				return
			}
		}
		if len(events) == 0 {
			// A comment on a poll that changed nothing keeps a proxy in front from closing the stream as
			// idle, and tells the server of a client that has gone away. The browser ignores it.
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

// streamState is what one stream has sent. sent holds each object's last sent state, keyed by kind
// and identity, and known the plans the stream follows until they leave APPLYING.
type streamState struct {
	sent  map[string][]byte
	known map[string]bool
	since time.Time
}

type event struct {
	name string
	data []byte
}

// poll reads the recorded state once and returns the events for what changed since the last poll.
func (s *Server) poll(ctx context.Context, account string, st *streamState) ([]event, error) {
	started := s.opts.Clock()
	var events []event
	changed := func(key, name string, v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if bytes.Equal(st.sent[key], data) {
			return nil
		}
		st.sent[key] = data
		events = append(events, event{name: name, data: data})
		return nil
	}
	err := tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		runs, err := jobruns.New(t).StreamRuns(ctx, jobruns.StreamRunsParams{
			AccountID: account, Since: pgtype.Timestamptz{Time: st.since, Valid: true},
		})
		if err != nil {
			return err
		}
		for _, row := range runs {
			e := runEvent{Account: account, run: run{
				RunID: row.RunID, Workload: row.Workload, Pass: optionalText(row.Pass), State: row.State,
				PlanID: optionalUUID(row.PlanID), PlanDescription: optionalText(row.PlanDescription),
				PlanStatus: optionalText(row.PlanStatus), ResumedFrom: optionalText(row.ResumedFrom),
				StartedAt: registry.Stamp(row.StartedAt.Time), FinishedAt: optionalStamp(row.FinishedAt),
				HeartbeatAt: optionalStamp(row.HeartbeatAt), Checkpoint: rawJSON(row.Checkpoint),
				Counters: rawJSON(row.Counters), LastError: optionalText(row.LastError),
			}}
			if err := changed("run:"+row.RunID, eventRun, e); err != nil {
				return err
			}
		}
		rate, err := s.rate(ctx, ratestate.New(t), account)
		if err != nil {
			return err
		}
		if rate != nil {
			if err := changed("rate", eventRate, rateEvent{Account: account, rateBlock: *rate}); err != nil {
				return err
			}
		}
		known := make([]pgtype.UUID, 0, len(st.known))
		for id := range st.known {
			var u pgtype.UUID
			if err := u.Scan(id); err != nil {
				return err
			}
			known = append(known, u)
		}
		plans, err := reorgplans.New(t).StreamPlans(ctx, reorgplans.StreamPlansParams{AccountID: account, Known: known})
		if err != nil {
			return err
		}
		for _, row := range plans {
			id := row.PlanID.String()
			if err := changed("plan:"+id, eventPlan, planEvent{Account: account, PlanID: id, Status: row.Status, Applied: row.Applied, Of: row.Total}); err != nil {
				return err
			}
			if row.Status == "APPLYING" {
				st.known[id] = true
			} else {
				delete(st.known, id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// A run finishing while this poll read is caught by the next, which reads from this poll's start
	// less one interval, since the workloads stamp their own times.
	st.since = started.Add(-s.opts.StreamInterval)
	return events, nil
}
