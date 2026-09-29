package api

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// run is a job run's whole recorded state, the same shape on the jobs endpoint and in the stream's
// run event (docs/UI.md sections 8.3 and 17.5). The checkpoint and the counters are the workload's
// own JSON (ADR-0016), which the cards read their fields from. last_error is provider or scanner text,
// never a body.
type run struct {
	RunID           string          `json:"run_id"`
	Workload        string          `json:"workload"`
	Pass            *string         `json:"pass"`
	State           string          `json:"state"`
	PlanID          *string         `json:"plan_id"`
	PlanDescription *string         `json:"plan_description"`
	PlanStatus      *string         `json:"plan_status"`
	ResumedFrom     *string         `json:"resumed_from"`
	StartedAt       string          `json:"started_at"`
	FinishedAt      *string         `json:"finished_at"`
	HeartbeatAt     *string         `json:"heartbeat_at"`
	Checkpoint      json.RawMessage `json:"checkpoint"`
	Counters        json.RawMessage `json:"counters"`
	LastError       *string         `json:"last_error"`
}

func runType() schema.Type {
	return schema.Obj("Run",
		schema.F("run_id", schema.Str()),
		schema.F("workload", schema.Str("backfill", "sync", "apply", "heuristics")),
		schema.F("pass", schema.Null(schema.Str("pass1", "pass2", "tick", "gap_recovery", "apply", "rollback"))),
		schema.F("state", schema.Str("running", "succeeded", "failed")),
		schema.F("plan_id", schema.Null(schema.Str())),
		schema.F("plan_description", schema.Null(schema.Str())),
		schema.F("plan_status", schema.Null(schema.Str())),
		schema.F("resumed_from", schema.Null(schema.Str())),
		schema.F("started_at", schema.Time()),
		schema.F("finished_at", schema.Null(schema.Time())),
		schema.F("heartbeat_at", schema.Null(schema.Time())),
		schema.F("checkpoint", schema.Any()),
		schema.F("counters", schema.Any()),
		schema.F("last_error", schema.Null(schema.Str())),
	)
}

// optionalText is a nullable stored text, nil when null.
func optionalText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// optionalStamp is a nullable stored instant as the read API writes it, nil when null.
func optionalStamp(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := registry.Stamp(t.Time)
	return &s
}

// optionalUUID is a nullable stored identifier, nil when null.
func optionalUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}

// rawJSON is stored JSON, or JSON null for a null column.
func rawJSON(b []byte) json.RawMessage {
	if b == nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
