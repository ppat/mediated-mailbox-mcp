package api

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// run is a job run's whole recorded state, the registry's Run, which the jobs endpoint, the stream's
// run event and the runs dataset's rows share (docs/UI.md sections 8.3 and 17.5).
type run = registry.Run

func runType() schema.Type { return registry.RunType() }

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
