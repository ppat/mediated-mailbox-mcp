package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/maskingevents"
	"github.com/ppat/mediated-mailbox-mcp/db/messages"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// Account is one account the mediator serves, as the accounts listing returns it (ADR-0035).
type Account struct {
	ID       string `json:"account_id"`
	Provider string `json:"provider"`
}

// Sources are what the operations read from. The reads of metadata and state read the index and the
// recorded state and never a provider, so none of them spends rate budget or reaches a body
// (ADR-0002, ADR-0034). The body operation alone reaches a provider, through Bodies, and only for a
// message the Redaction Gate released.
type Sources struct {
	// DB is the mediator's pool. Every read runs in a transaction of its account through db/tx, and an
	// index read in one snapshot of it, through tx.Snapshot.
	DB Database
	// Accounts returns the accounts the mediator serves, from the account snapshot in force.
	Accounts func() []Account
	// Policy returns the policy an account's messages are decided against, from the policy snapshot
	// in force. A call takes it once.
	Policy func(account string) policy.Composed
	// Lookups are the sender classifier's domain functions.
	Lookups classify.Lookups
	// Now returns the current time, which the system status measures ages and backoff against.
	Now func() time.Time
	// Bodies are what the body operation needs beyond the index.
	Bodies Bodies
}

// Database is what the operations read through, a pool or a connection, which opens a transaction at
// the database's default isolation level and with options.
type Database interface {
	tx.Beginner
	tx.BeginnerWithOptions
}

// pageSize is how many rows one page of a listing holds.
const pageSize = 100

// Operations returns every operation the client surface serves, the complete set both roots are
// generated from (ADR-0030). Every operation is a read, of the index, of the recorded state, or of a
// body the Redaction Gate releases, and every identifier one takes comes from another (ADR-0035). A message's identifier comes from the message
// listing or a thread, a thread's from the thread listing, and an account's from the accounts listing. The index query's labels come from the
// labels listing, its addresses from any served message, and its domains from the sender listing.
func Operations(s Sources) []Operation {
	return append([]Operation{
		{
			Name:   "list_accounts",
			Effect: Read,
			Path:   accountsPath,
			Description: "Lists every account the mediator serves, each with its identifier and provider. " +
				"Every other operation takes one of these identifiers as account_id.",
			Input:  json.RawMessage(`{"type":"object","properties":{}}`),
			Output: json.RawMessage(`{"type":"object","properties":{"accounts":{"type":"array","items":{"type":"object","properties":{"account_id":{"type":"string"},"provider":{"type":"string"}},"required":["account_id","provider"]}}},"required":["accounts"]}`),
			Handle: s.listAccounts,
		},
		{
			Name:   "list_labels",
			Effect: Read,
			Path:   accountPrefix + "labels",
			Description: "Lists every distinct label value the account's indexed messages hold, in order. " +
				"These are the values every other operation takes as a label.",
			Input:  accountInput(``, ``),
			Output: json.RawMessage(`{"type":"object","properties":{"labels":{"type":"array","items":{"type":"string"}}},"required":["labels"]}`),
			Handle: s.listLabels,
		},
		{
			Name:   "list_messages",
			Effect: Read,
			Path:   accountPrefix + "messages",
			Description: "Lists the account's messages, newest first, one page at a time, with each message's metadata as " +
				"the redaction matrix allows it. Pass the next_cursor a page returned as cursor to read the next page. " +
				"next_cursor is null on the last page.",
			Input:  accountInput(`,"cursor":{"type":"string"}`, ``),
			Output: pageSchema("messages", messageSchema),
			Handle: s.listMessages,
		},
		{
			Name:   "get_message",
			Effect: Read,
			Path:   accountPrefix + "messages/{message_id}",
			Description: "Returns one message's metadata as the redaction matrix allows it. " +
				"body_available is the Redaction Gate's verdict under the policy this read took. get_message_body decides again when it is called, " +
				"and can still withhold the body. No body, snippet or attachment filename is returned here.",
			Input:  accountInput(`,"message_id":{"type":"string"}`, `,"message_id"`),
			Output: json.RawMessage(messageSchema),
			Handle: s.getMessage,
		},
		s.bodyOperation(),
		{
			Name:   "list_threads",
			Effect: Read,
			Path:   accountPrefix + "threads",
			Description: "Lists the account's threads, the one with the latest message first, one page at a time, " +
				"each with its number of messages and the time of its latest. Pass the next_cursor a page returned as cursor " +
				"to read the next page. next_cursor is null on the last page.",
			Input: accountInput(`,"cursor":{"type":"string"}`, ``),
			Output: pageSchema("threads", `{"type":"object","properties":{"thread_id":{"type":"string"},"messages":{"type":"integer"},"latest_at":`+
				timestampSchema+`},"required":["thread_id","messages","latest_at"]}`),
			Handle: s.listThreads,
		},
		{
			Name:        "get_thread",
			Effect:      Read,
			Path:        accountPrefix + "threads/{thread_id}",
			Description: "Returns every message of one thread, oldest first, each with its metadata as the redaction matrix allows it.",
			Input:       accountInput(`,"thread_id":{"type":"string"}`, `,"thread_id"`),
			Output: json.RawMessage(`{"type":"object","properties":{"thread_id":{"type":"string"},"messages":{"type":"array","items":` +
				messageSchema + `}},"required":["thread_id","messages"]}`),
			Handle: s.getThread,
		},
		{
			Name:   "list_masking_events",
			Effect: Read,
			Path:   accountPrefix + "masking-events",
			Description: "Lists the subjects the account's messages had masked at rest, the latest first, one page at a time, " +
				"each with the rule and tier that fired and never the masked text. since, a UTC timestamp ending in Z, keeps the " +
				"events masked at or after it, and any other offset is refused. Pass the next_cursor a page returned as cursor " +
				"to read the next page. next_cursor is null on the last page.",
			Input: accountInput(`,"since":`+timestampSchema+`,"cursor":{"type":"string"}`, ``),
			Output: pageSchema("events", `{"type":"object","properties":{"message_id":{"type":"string"},"field":{"type":"string"},`+
				`"rule_id":{"type":"string"},"tier":{"type":"integer"},"masked_at":`+timestampSchema+`},`+
				`"required":["message_id","field","rule_id","tier","masked_at"]}`),
			Handle: s.listMaskingEvents,
		},
		{
			Name:   "get_system_status",
			Effect: Read,
			Path:   accountPrefix + "status",
			Description: "Returns the account's recorded operational state, so a client can tell why something is slow, " +
				"missing or denied. It reads backfill progress, the delta sync cursor's age and last successful tick, the content scan " +
				"backlog, the rate controller's state and the last provider authentication. It holds no mail content and no policy.",
			Input:  accountInput(``, ``),
			Output: json.RawMessage(statusSchema),
			Handle: s.getSystemStatus,
		},
	}, s.indexOperations()...)
}

// accountInput is the input schema of an operation on one account, whose properties are account_id
// and extra, and whose required arguments are account_id and required.
func accountInput(extra, required string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}` + extra +
		`},"required":["account_id"` + required + `],"additionalProperties":false}`)
}

// pageSchema is the output schema of a page of a listing whose rows sit under key.
func pageSchema(key, row string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"` + key + `":{"type":"array","items":` + row +
		`},"next_cursor":{"type":["string","null"]}},"required":["` + key + `","next_cursor"]}`)
}

// arguments decodes a call's arguments, account_id already removed, into v. names are the arguments
// the operation takes, spelled exactly as its input schema declares them. A key naming none of them
// exactly, one differing only in case included, is refused, and so is a null value, which no input
// schema admits. The decoder alone would match a key in another case and read a null as absent, so a
// call the API root refuses would reach the operation through the MCP root (ADR-0087).
func arguments(input json.RawMessage, v any, names ...string) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(input, &raw); err != nil {
		return Refuse("the arguments do not match the operation's input schema")
	}
	for key, value := range raw {
		if !slices.Contains(names, key) {
			return Refuse("the arguments name one the operation does not take")
		}
		if string(bytes.TrimSpace(value)) == "null" {
			return Refuse(key + " may not be null")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Refuse("the arguments do not match the operation's input schema")
	}
	return nil
}

// ok writes an operation's result.
func ok(v any) (json.RawMessage, error) { return json.Marshal(v) }

func (s Sources) listAccounts(_ context.Context, _ string, input json.RawMessage) (json.RawMessage, error) {
	if err := arguments(input, &struct{}{}); err != nil {
		return nil, err
	}
	accounts := s.Accounts()
	if accounts == nil {
		accounts = []Account{}
	}
	return ok(struct {
		Accounts []Account `json:"accounts"`
	}{accounts})
}

func (s Sources) listLabels(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	if err := arguments(input, &struct{}{}); err != nil {
		return nil, err
	}
	var labels []string
	err := tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		labels, err = messages.New(t).Labels(ctx, account)
		return err
	})
	if err != nil {
		return nil, err
	}
	return ok(struct {
		Labels []string `json:"labels"`
	}{nonNil(labels)})
}

// cursorArgs are a listing's cursor argument.
type cursorArgs struct {
	Cursor string `json:"cursor"`
}

func (s Sources) listMessages(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args cursorArgs
	if err := arguments(input, &args, "cursor"); err != nil {
		return nil, err
	}
	at, from, err := s.after(args.Cursor, "list_messages", account, "")
	if err != nil {
		return nil, err
	}
	params := messages.MessagePageParams{AccountID: account, PageSize: pageSize}
	if from != nil {
		params.AfterSentAt = pgtype.Timestamptz{Time: at, Valid: true}
		params.AfterMessageID = from.Key
	}
	var rows []messages.MessagePageRow
	err = tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		rows, err = messages.New(t).MessagePage(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	p := s.Policy(account)
	out := make([]message, 0, len(rows))
	for _, r := range rows {
		out = append(out, present(account, fromRow(messages.MessageRow(r)), p, s.Lookups))
	}
	var next *string
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		c := encodeCursor(position{Listing: "list_messages", Account: account, At: formatUTC(last.SentAt.Time), Key: last.MessageID})
		next = &c
	}
	return ok(struct {
		Messages   []message `json:"messages"`
		NextCursor *string   `json:"next_cursor"`
	}{out, next})
}

func (s Sources) getMessage(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args struct {
		MessageID string `json:"message_id"`
	}
	if err := arguments(input, &args, "message_id"); err != nil {
		return nil, err
	}
	var rows []messages.MessageRow
	err := tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		rows, err = messages.New(t).Message(ctx, messages.MessageParams{AccountID: account, MessageID: args.MessageID})
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, Refuse("the account holds no message with that message_id")
	}
	return ok(present(account, fromRow(rows[0]), s.Policy(account), s.Lookups))
}

type thread struct {
	ThreadID string `json:"thread_id"`
	Messages int64  `json:"messages"`
	LatestAt string `json:"latest_at"`
}

func (s Sources) listThreads(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args cursorArgs
	if err := arguments(input, &args, "cursor"); err != nil {
		return nil, err
	}
	at, from, err := s.after(args.Cursor, "list_threads", account, "")
	if err != nil {
		return nil, err
	}
	params := messages.ThreadPageParams{AccountID: account, PageSize: pageSize}
	if from != nil {
		params.BeforeLatestAt = pgtype.Timestamptz{Time: at, Valid: true}
		params.BeforeThreadID = from.Key
	}
	var rows []messages.ThreadPageRow
	err = tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		rows, err = messages.New(t).ThreadPage(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]thread, 0, len(rows))
	for _, r := range rows {
		out = append(out, thread{ThreadID: r.ThreadID, Messages: r.Messages, LatestAt: formatUTC(r.LatestAt.Time)})
	}
	var next *string
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		c := encodeCursor(position{Listing: "list_threads", Account: account, At: formatUTC(last.LatestAt.Time), Key: last.ThreadID})
		next = &c
	}
	return ok(struct {
		Threads    []thread `json:"threads"`
		NextCursor *string  `json:"next_cursor"`
	}{out, next})
}

func (s Sources) getThread(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args struct {
		ThreadID string `json:"thread_id"`
	}
	if err := arguments(input, &args, "thread_id"); err != nil {
		return nil, err
	}
	var rows []messages.ThreadMessagesRow
	err := tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		rows, err = messages.New(t).ThreadMessages(ctx, messages.ThreadMessagesParams{AccountID: account, ThreadID: args.ThreadID})
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, Refuse("the account holds no thread with that thread_id")
	}
	p := s.Policy(account)
	out := make([]message, 0, len(rows))
	for _, r := range rows {
		out = append(out, present(account, fromRow(messages.MessageRow(r)), p, s.Lookups))
	}
	return ok(struct {
		ThreadID string    `json:"thread_id"`
		Messages []message `json:"messages"`
	}{args.ThreadID, out})
}

type maskingEvent struct {
	MessageID string `json:"message_id"`
	Field     string `json:"field"`
	RuleID    string `json:"rule_id"`
	Tier      int32  `json:"tier"`
	MaskedAt  string `json:"masked_at"`
}

func (s Sources) listMaskingEvents(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args struct {
		Since  *string `json:"since"`
		Cursor string  `json:"cursor"`
	}
	if err := arguments(input, &args, "since", "cursor"); err != nil {
		return nil, err
	}
	params := maskingevents.MaskingEventPageParams{AccountID: account, PageSize: pageSize}
	filter := ""
	if args.Since != nil {
		since, err := parseUTC("since", *args.Since)
		if err != nil {
			return nil, err
		}
		params.Since = pgtype.Timestamptz{Time: since, Valid: true}
		filter = formatUTC(since)
	}
	at, from, err := s.after(args.Cursor, "list_masking_events", account, filter)
	if err != nil {
		return nil, err
	}
	if from != nil {
		id, err := strconv.ParseInt(from.Key, 10, 64)
		if err != nil {
			return nil, Refuse("cursor is not a cursor this listing returned")
		}
		params.BeforeMaskedAt = pgtype.Timestamptz{Time: at, Valid: true}
		params.BeforeID = id
	}
	var rows []maskingevents.MaskingEventPageRow
	err = tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		var err error
		rows, err = maskingevents.New(t).MaskingEventPage(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]maskingEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, maskingEvent{MessageID: r.MessageID, Field: r.Field, RuleID: r.RuleID, Tier: r.Tier, MaskedAt: formatUTC(r.MaskedAt.Time)})
	}
	var next *string
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		c := encodeCursor(position{Listing: "list_masking_events", Account: account, Filter: filter, At: formatUTC(last.MaskedAt.Time), Key: strconv.FormatInt(last.ID, 10)})
		next = &c
	}
	return ok(struct {
		Events     []maskingEvent `json:"events"`
		NextCursor *string        `json:"next_cursor"`
	}{out, next})
}

// after reads a listing's cursor and returns the time of the position it names, or nil for none.
func (s Sources) after(cursor, listing, account, filter string) (time.Time, *position, error) {
	p, found, err := decodeCursor(cursor, listing, account, filter)
	if err != nil || !found {
		return time.Time{}, nil, err
	}
	at, err := parseCursorTime(p.At)
	if err != nil {
		return time.Time{}, nil, err
	}
	return at, &p, nil
}

// parseCursorTime reads the time a cursor's position carries, refusing a cursor that carries none.
func parseCursorTime(at string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, Refuse("cursor is not a cursor this listing returned")
	}
	return t, nil
}

// fromRow returns a message row as the presentation reads it.
func fromRow(r messages.MessageRow) stored {
	return stored{
		MessageID:       r.MessageID,
		ThreadID:        r.ThreadID,
		FromEmail:       r.FromEmail,
		FromName:        text(r.FromName),
		Subject:         text(r.Subject),
		SentAt:          r.SentAt.Time,
		Labels:          r.Labels,
		Flags:           r.Flags,
		HasAttachments:  r.HasAttachments,
		AttachmentMedia: attachmentMedia(r.AttachmentMediaTypes, r.AttachmentExtensions),
		ContentFlags:    r.ContentFlags,
		ScanState:       r.ScanState,
	}
}

// attachmentMedia pairs each stored media type with the extension at the same position, as the
// statement returns them (ADR-0123).
func attachmentMedia(mediaTypes, extensions []string) []mail.AttachmentMedia {
	out := make([]mail.AttachmentMedia, 0, len(mediaTypes))
	for i, t := range mediaTypes {
		if i < len(extensions) {
			out = append(out, mail.AttachmentMedia{MediaType: t, Extension: extensions[i]})
		}
	}
	return out
}

// text returns a nullable text column's value, or nil for null.
func text(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// timestamp returns a nullable timestamp column's value written in UTC, or nil for null.
func timestamp(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := formatUTC(t.Time)
	return &s
}

// status is an account's recorded operational state (ADR-0034). Every field is read from a table its
// owning workload records, and none holds mail content or policy. A section with no recorded row is
// null.
type status struct {
	AccountID   string     `json:"account_id"`
	Backfill    *backfill  `json:"backfill"`
	Sync        *syncState `json:"sync"`
	ScanBacklog int64      `json:"scan_backlog"`
	Rate        *rateState `json:"rate"`
	LastAuth    *lastAuth  `json:"last_authentication"`
}

type backfill struct {
	Pass1Complete bool `json:"pass1_complete"`
	Pass2Complete bool `json:"pass2_complete"`
}

type syncState struct {
	CursorWrittenAt     *string  `json:"cursor_written_at"`
	CursorAgeSeconds    *float64 `json:"cursor_age_seconds"`
	LastSucceededTickAt *string  `json:"last_succeeded_tick_at"`
}

type rateState struct {
	Current      float32 `json:"current_rate"`
	Target       float32 `json:"target_rate"`
	BackoffUntil *string `json:"backoff_until"`
	InBackoff    bool    `json:"in_backoff"`
}

type lastAuth struct {
	At      *string `json:"at"`
	Outcome *string `json:"outcome"`
}

func (s Sources) getSystemStatus(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	if err := arguments(input, &struct{}{}); err != nil {
		return nil, err
	}
	out := status{AccountID: account}
	now := s.Now()
	err := tx.Run(ctx, s.DB, account, func(t pgx.Tx) error {
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		default:
			out.Backfill = &backfill{Pass1Complete: progress.BackfillPass1Complete, Pass2Complete: progress.BackfillPass2Complete}
			out.Sync = &syncState{CursorWrittenAt: timestamp(progress.SyncCursorAt)}
			if progress.SyncCursorAt.Valid {
				age := now.Sub(progress.SyncCursorAt.Time).Seconds()
				out.Sync.CursorAgeSeconds = &age
			}
			out.LastAuth = &lastAuth{At: timestamp(progress.LastAuthAt), Outcome: text(progress.LastAuthOutcome)}
		}
		tick, err := jobruns.New(t).LastSucceededTick(ctx, account)
		if err != nil {
			return err
		}
		if tick.Valid {
			if out.Sync == nil {
				out.Sync = &syncState{}
			}
			out.Sync.LastSucceededTickAt = timestamp(tick)
		}
		if out.ScanBacklog, err = messages.New(t).ScanBacklog(ctx, account); err != nil {
			return err
		}
		rate, err := ratestate.New(t).RateStatus(ctx, account)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		default:
			out.Rate = &rateState{
				Current:      rate.CurrentRate,
				Target:       rate.TargetRate,
				BackoffUntil: timestamp(rate.BackoffUntil),
				InBackoff:    rate.BackoffUntil.Valid && rate.BackoffUntil.Time.After(now),
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ok(out)
}

// nullableTimestamp is the JSON Schema of a timestamp that may be absent.
const nullableTimestamp = `{"type":["string","null"],"format":"date-time","pattern":"Z$"}`

// statusSchema is the JSON Schema of the system status.
const statusSchema = `{"type":"object","properties":{"account_id":{"type":"string"},` +
	`"backfill":{"type":["object","null"],"properties":{"pass1_complete":{"type":"boolean"},"pass2_complete":{"type":"boolean"}}},` +
	`"sync":{"type":["object","null"],"properties":{"cursor_written_at":` + nullableTimestamp + `,` +
	`"cursor_age_seconds":{"type":["number","null"]},"last_succeeded_tick_at":` + nullableTimestamp + `}},` +
	`"scan_backlog":{"type":"integer"},` +
	`"rate":{"type":["object","null"],"properties":{"current_rate":{"type":"number"},"target_rate":{"type":"number"},` +
	`"backoff_until":` + nullableTimestamp + `,"in_backoff":{"type":"boolean"}}},` +
	`"last_authentication":{"type":["object","null"],"properties":{"at":` + nullableTimestamp + `,"outcome":{"type":["string","null"]}}}},` +
	`"required":["account_id","backfill","sync","scan_backlog","rate","last_authentication"]}`
