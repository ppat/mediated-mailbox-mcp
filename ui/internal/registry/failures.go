package registry

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/classification"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// The failure vocabularies as stored (ADR-0016), and the dispositions' wording (docs/UI.md section 11)
// in the same order.
func errorClasses() []string {
	return []string{"throttled", "provider_error", "gone", "scanner_timeout", "validation", "authentication"}
}

func dispositions() []string { return []string{"recovered", "pending", "gone", "abandoned"} }

func dispositionWording() []string { return []string{"recovered", "pending", "gone", "abandoned"} }

// FailureRow is one row of the failures dataset, one of a run's item failures with the message-row
// fields of its item's message (docs/UI.md sections 7.1, 7.2 and 8.4). The message fields are nil for
// a page item and for a message the index no longer holds. The stored vocabularies are sent as stored,
// so a value the wording does not know is shown rather than dropped (docs/UI.md section 7.1).
type FailureRow struct {
	Seq          int64    `json:"seq"`
	ItemKind     string   `json:"item_kind"`
	ItemID       string   `json:"item_id"`
	MessageID    *string  `json:"message_id"`
	FromEmail    *string  `json:"from_email"`
	Subject      *string  `json:"subject"`
	SentAt       *string  `json:"sent_at"`
	Labels       []string `json:"labels"`
	SenderClass  *string  `json:"sender_class"`
	ContentFlags []string `json:"content_flags"`
	ScanState    *string  `json:"scan_state"`
	Page         *int32   `json:"page"`
	ErrorClass   string   `json:"error_class"`
	Attempts     int32    `json:"attempts"`
	FirstAt      string   `json:"first_at"`
	LastAt       string   `json:"last_at"`
	Disposition  string   `json:"disposition"`
	RecoveredBy  *string  `json:"recovered_by"`
}

func failureRowType() schema.Type {
	return schema.Obj("FailureRow",
		schema.F("seq", schema.Int()),
		schema.F("item_kind", schema.Str()),
		schema.F("item_id", schema.Str()),
		schema.F("message_id", schema.Null(schema.Str())),
		schema.F("from_email", schema.Null(schema.Str())),
		schema.F("subject", schema.Null(schema.Str())),
		schema.F("sent_at", schema.Null(schema.Time())),
		schema.F("labels", schema.Null(schema.ArrayOf(schema.Str()))),
		schema.F("sender_class", schema.Null(schema.Str())),
		schema.F("content_flags", schema.Null(schema.ArrayOf(schema.Str()))),
		schema.F("scan_state", schema.Null(schema.Str())),
		schema.F("page", schema.Null(schema.Int())),
		schema.F("error_class", schema.Str()),
		schema.F("attempts", schema.Int()),
		schema.F("first_at", schema.Time()),
		schema.F("last_at", schema.Time()),
		schema.F("disposition", schema.Str()),
		schema.F("recovered_by", schema.Null(schema.Str())),
	)
}

// FailureDetail is the failures dataset's row detail, the row, the error summary as recorded, which is
// provider or scanner text and never a body, the item's message's sensitivity block, and the newest
// fifty audit rows of the message with their count (docs/UI.md sections 7.1, 8.4 and 17.1).
type FailureDetail struct {
	Account      string     `json:"account"`
	Dataset      string     `json:"dataset"`
	AsOf         string     `json:"as_of"`
	Run          string     `json:"run"`
	Row          FailureRow `json:"row"`
	ErrorSummary *string    `json:"error_summary"`
	// The message's sensitivity block (docs/UI.md section 7.1), each nil for a page item and for a
	// message the index no longer holds, and the scan fields nil for a message never scanned.
	RuleIDs        []string     `json:"rule_ids"`
	ScannedAt      *string      `json:"scanned_at"`
	ScannerVersion *int32       `json:"scanner_version"`
	Audit          []AuditEntry `json:"audit"`
	AuditCount     int64        `json:"audit_count"`
}

// AuditEntry is one audit row of a message, as a row detail lists it.
type AuditEntry struct {
	ID     int64  `json:"id"`
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
}

func failureDetailType() schema.Type {
	return schema.Obj("FailureDetail",
		schema.F("account", schema.Str()),
		schema.F("dataset", schema.Str("failures")),
		schema.F("as_of", schema.Time()),
		schema.F("run", schema.Str()),
		schema.F("row", failureRowType()),
		schema.F("error_summary", schema.Null(schema.Str())),
		schema.F("rule_ids", schema.Null(schema.ArrayOf(schema.Str()))),
		schema.F("scanned_at", schema.Null(schema.Time())),
		schema.F("scanner_version", schema.Null(schema.Int())),
		schema.F("audit", schema.ArrayOf(schema.Obj("AuditEntry",
			schema.F("id", schema.Int()),
			schema.F("at", schema.Time()),
			schema.F("actor", schema.Str()),
			schema.F("action", schema.Str()),
		))),
		schema.F("audit_count", schema.Int()),
	)
}

// failures is a run's item failures, the run screen's ladder, read under the required parent filter
// run (docs/UI.md section 8.4). It is the first dataset carrying message-derived rows and the first to
// declare a provenance query.
func failures() Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:   "failures",
			Parent: "run",
			Dimensions: []lens.Dimension{
				{Name: "error_class", Storage: "text", Groupable: true, Filterable: true, Wording: "error class", Values: errorClasses()},
				{Name: "sender", Storage: "text", Groupable: true, Filterable: true, Wording: "sender", NullWording: "no message", Empty: true},
				{Name: "page_number", Storage: "number", Groupable: true, Filterable: true, Wording: "page", NullWording: "no page"},
				{Name: "disposition", Storage: "text", Groupable: true, Filterable: true, Wording: "disposition", Values: dispositions()},
				{Name: "last_at", Storage: "time", Sortable: true, Wording: "last error"},
				{Name: "attempts", Storage: "number", Sortable: true, Wording: "attempts"},
			},
			Default:  lens.Defaults{Group: "error_class", Level: lens.Distribution, Sort: lens.Sort{Column: "last_at", Descending: true}},
			Identity: &lens.RowIdentity{Name: "seq", Storage: "number"},
		},
		MessageDerived: true,
		Row:            failureRowType(),
		Summary:        failureSummary,
		Rows:           failureRows,
		Aggregates: map[string]Aggregate{
			"error_class": failuresByErrorClass,
			"sender":      failuresBySender,
			"page_number": failuresByPage,
			"disposition": failuresByDisposition,
		},
		Detail:     failureDetail,
		DetailType: failureDetailType(),
	}
}

// failureFilters are a read's filters as the failures statements take them.
type failureFilters struct {
	errorClassIn, errorClassOut, dispositionIn, dispositionOut []string
	sender                                                     nullable
	pageIn, pageOut                                            []int32
	pageInNone, pageOutNone                                    bool
}

func failureFiltersOf(r Read) (failureFilters, error) {
	var f failureFilters
	f.errorClassIn, f.errorClassOut = split(r.Request, "error_class")
	f.dispositionIn, f.dispositionOut = split(r.Request, "disposition")
	f.sender = splitNullable(r.Request, "sender")
	page := splitNullable(r.Request, "page_number")
	f.pageInNone, f.pageOutNone = page.inNone, page.outNone
	var err error
	if f.pageIn, err = whole(page.in); err != nil {
		return failureFilters{}, err
	}
	if f.pageOut, err = whole(page.out); err != nil {
		return failureFilters{}, err
	}
	return f, nil
}

func failureSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, Total{}, err
	}
	rows, err := q.Failures.FailureFigures(ctx, classification.FailureFiguresParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
	})
	if err != nil {
		return nil, Total{}, err
	}
	var total Total
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Disposition] = row.Failures
		total.Count += row.Failures
		total.Restricted += row.Restricted
		total.Flagged += row.Flagged
	}
	screen := "jobs/" + url.PathEscape(r.Request.ParentID)
	figures := []Figure{count("failures", "failures", total.Count, link(r.Account, screen, url.Values{"level": {"3"}}))}
	at := func(d string) string {
		return link(r.Account, screen, url.Values{"level": {"3"}, "disposition": {d}})
	}
	wording := dispositionWording()
	for i, d := range dispositions() {
		figures = append(figures, count(d, wording[i], counts[d], at(d)))
	}
	var unknown []string
	for d := range counts {
		if !slices.Contains(dispositions(), d) {
			unknown = append(unknown, d)
		}
	}
	slices.Sort(unknown)
	for _, d := range unknown {
		// A stored disposition the design does not know is shown, never dropped (docs/UI.md section 7.1).
		figures = append(figures, count(d, d, counts[d], at(d)))
	}
	return figures, total, nil
}

func failureRows(ctx context.Context, q Queries, r Read) (any, error) {
	out := []FailureRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Failures.FailureRows(ctx, classification.FailureRowsParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
		SortColumn: r.Request.Sort.Column, Descending: r.Request.Sort.Descending, RowOffset: first,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, failureRow(classification.FailureDetailRow{
			Seq: row.Seq, ItemKind: row.ItemKind, ItemID: row.ItemID, MessageID: row.MessageID, FromEmail: row.FromEmail,
			Subject: row.Subject, SentAt: row.SentAt, Labels: row.Labels, SenderClass: row.SenderClass,
			ContentFlags: row.ContentFlags, ScanState: row.ScanState, Page: row.Page, ErrorClass: row.ErrorClass,
			Attempts: row.Attempts, FirstAt: row.FirstAt, LastAt: row.LastAt, Disposition: row.Disposition, RecoveredBy: row.RecoveredBy,
		}))
	}
	return out, nil
}

// failureRow is a stored failure as a row, from the detail statement's row, which carries every field
// the rows statement does and the error summary.
func failureRow(row classification.FailureDetailRow) FailureRow {
	out := FailureRow{
		Seq: row.Seq, ItemKind: row.ItemKind, ItemID: row.ItemID,
		MessageID: text(row.MessageID.Valid, row.MessageID.String), FromEmail: text(row.FromEmail.Valid, row.FromEmail.String),
		Subject: text(row.Subject.Valid, row.Subject.String), SentAt: optionalStamp(row.SentAt),
		SenderClass: text(row.SenderClass.Valid, row.SenderClass.String), ScanState: text(row.ScanState.Valid, row.ScanState.String),
		ErrorClass: row.ErrorClass, Attempts: row.Attempts, FirstAt: stamp(row.FirstAt), LastAt: stamp(row.LastAt),
		Disposition: row.Disposition, RecoveredBy: text(row.RecoveredBy.Valid, row.RecoveredBy.String),
	}
	// A message's labels and flags are never null, so a null array is the joined message's absence.
	if row.MessageID.Valid {
		out.Labels, out.ContentFlags = row.Labels, row.ContentFlags
		if out.Labels == nil {
			out.Labels = []string{}
		}
		if out.ContentFlags == nil {
			out.ContentFlags = []string{}
		}
	}
	if row.Page.Valid {
		p := row.Page.Int32
		out.Page = &p
	}
	return out
}

func failuresByErrorClass(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Failures.FailuresByErrorClass(ctx, classification.FailuresByErrorClassParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SensitiveGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, SensitiveGroup{Key: map[string]any{"error_class": row.ErrorClass}, Count: row.Failures, Restricted: row.Restricted, Flagged: row.Flagged})
	}
	return out, nil
}

func failuresBySender(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Failures.FailuresBySender(ctx, classification.FailuresBySenderParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SensitiveGroup, 0, len(rows))
	for _, row := range rows {
		// The statement marks the group of items with no message, and keys every other group by its
		// domain as the database lowers it, a stored empty domain included.
		var key any
		if !row.NoDomain {
			key = row.FromDomain
		}
		out = append(out, SensitiveGroup{Key: map[string]any{"sender": key}, Count: row.Failures, Restricted: row.Restricted, Flagged: row.Flagged})
	}
	return out, nil
}

func failuresByPage(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Failures.FailuresByPage(ctx, classification.FailuresByPageParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SensitiveGroup, 0, len(rows))
	for _, row := range rows {
		var key any
		if row.Page.Valid {
			key = row.Page.Int32
		}
		out = append(out, SensitiveGroup{Key: map[string]any{"page_number": key}, Count: row.Failures, Restricted: row.Restricted, Flagged: row.Flagged})
	}
	return out, nil
}

func failuresByDisposition(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := failureFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Failures.FailuresByDisposition(ctx, classification.FailuresByDispositionParams{
		AccountID: r.Account, RunID: r.Request.ParentID,
		ErrorClassIn: f.errorClassIn, ErrorClassOut: f.errorClassOut, DispositionIn: f.dispositionIn, DispositionOut: f.dispositionOut,
		SenderIn: f.sender.in, SenderInNone: f.sender.inNone, SenderOut: f.sender.out, SenderOutNone: f.sender.outNone,
		PageIn: f.pageIn, PageInNone: f.pageInNone, PageOut: f.pageOut, PageOutNone: f.pageOutNone,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SensitiveGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, SensitiveGroup{Key: map[string]any{"disposition": row.Disposition}, Count: row.Failures, Restricted: row.Restricted, Flagged: row.Flagged})
	}
	return out, nil
}

// failureDetail is the failures dataset's provenance query. A message or operation item carries the
// audit rows of its message, and a page item none.
func failureDetail(ctx context.Context, q Queries, account string, req lens.RowRequest, asOf time.Time) (any, error) {
	seq, err := strconv.ParseInt(req.Row, 10, 64)
	if err != nil {
		// The pure core admits only whole numbers, so this is not reached.
		return nil, err
	}
	row, err := q.Failures.FailureDetail(ctx, classification.FailureDetailParams{AccountID: account, RunID: req.ParentID, Seq: seq})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoRow
	}
	if err != nil {
		return nil, err
	}
	out := FailureDetail{
		Account: account, Dataset: "failures", AsOf: Stamp(asOf), Run: req.ParentID, Row: failureRow(row),
		ErrorSummary: text(row.ErrorSummary.Valid, row.ErrorSummary.String), ScannedAt: optionalStamp(row.ScannedAt),
		Audit: []AuditEntry{},
	}
	if row.MessageID.Valid {
		out.RuleIDs = row.RuleIds
		if out.RuleIDs == nil {
			out.RuleIDs = []string{}
		}
	}
	if row.ScannerVersion.Valid {
		v := row.ScannerVersion.Int32
		out.ScannerVersion = &v
	}
	if row.ItemKind == "message" || row.ItemKind == "op" {
		entries, err := q.Audit.MessageAuditRows(ctx, auditlog.MessageAuditRowsParams{AccountID: account, MessageID: pgtype.Text{String: row.ItemID, Valid: true}})
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			out.Audit = append(out.Audit, AuditEntry{ID: e.ID, At: stamp(e.Ts), Actor: e.Actor, Action: e.Action})
		}
		if out.AuditCount, err = q.Audit.MessageAuditCount(ctx, auditlog.MessageAuditCountParams{AccountID: account, MessageID: pgtype.Text{String: row.ItemID, Valid: true}}); err != nil {
			return nil, err
		}
	}
	return out, nil
}
