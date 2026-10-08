package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/messages"
	"github.com/ppat/mediated-mailbox-mcp/db/senders"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
)

// The index reads enumerate, sort, search, count and group the account's messages, and list its
// sender statistics, from the index alone (ADR-0109). They call no provider and return metadata only,
// as the redaction matrix allows it, and a message whose body is denied stays in every result
// (ADR-0001). Whatever they decide by sender class is decided under the policy the call took once, the
// one the messages they serve are presented under (ADR-0002). Each runs its statements in one snapshot
// of the index, so the domains it classifies are the ones its statements read (ADR-0109).

// indexOperations returns the index reads.
func (s Sources) indexOperations() []Operation {
	return []Operation{
		{
			Name:   "search_messages",
			Effect: StructuredRead,
			Path:   accountPrefix + "messages:search",
			Description: "Searches the account's messages, from the whole index rather than a recent window, and returns one page " +
				"of the messages the query selects, each with its metadata as the redaction matrix allows it. " + queryDescription + " " +
				"sort orders the page by date, sender or subject, date by default, and order is descending, the default, or ascending. " +
				"Messages tied on the sort value follow their message_id. Pass the next_cursor a page returned as cursor, with the same " +
				"query, sort and order, to read the next page. next_cursor is null on the last page.",
			Input: accountInput(`,"query":`+indexQuerySchema+`,"sort":{"type":"string","enum":["date","sender","subject"]},`+
				`"order":{"type":"string","enum":["descending","ascending"]},"cursor":{"type":"string"}`, ``),
			Output: pageSchema("messages", messageSchema),
			Handle: s.searchMessages,
		},
		{
			Name:   "count_messages",
			Effect: StructuredRead,
			Path:   accountPrefix + "message-counts:search",
			Description: "Counts the account's messages the query selects, from the whole index, and groups them. " + queryDescription + " " +
				"summary counts the whole selection, its messages, their threads, the unread among them and those with attachments, " +
				"with the dates of the oldest and the newest, which are null when nothing is selected. " +
				"group_by groups the selection by sender address, sender_domain, label, sender_class or the UTC month a message was sent in, " +
				"and each group carries the same counts under its key. A message counts in the group of every label it carries, and the " +
				"messages with no label form one group whose key is null, listed first. A month's key is the UTC instant it starts. " +
				"Groups come one page at a time, the group with the most messages first and ties by key. Pass the next_cursor a page " +
				"returned as cursor, with the same query and group_by, to read the next page. next_cursor is null on the last page " +
				"and without group_by.",
			Input: accountInput(`,"query":`+indexQuerySchema+`,`+
				`"group_by":{"type":"string","enum":["sender","sender_domain","label","sender_class","month"]},"cursor":{"type":"string"}`, ``),
			Output: json.RawMessage(`{"type":"object","properties":{"summary":` + groupSchema + `,"group_by":{"type":["string","null"]},` +
				`"groups":{"type":"array","items":` + groupSchema + `},"next_cursor":{"type":["string","null"]}},` +
				`"required":["summary","group_by","groups","next_cursor"]}`),
			Handle: s.countMessages,
		},
		{
			Name:   "list_senders",
			Effect: Read,
			Path:   accountPrefix + "senders",
			Description: "Lists the account's sender statistics, one per sender domain, built from the whole history, the sender " +
				"with the most messages first, one page at a time. Each carries its sender class under the policy in force, its number " +
				"of messages, its first and last message, the share of its messages carrying a List-Id, how many of its messages carry " +
				"each label, and samples of its display names and address local parts. Pass the next_cursor a page returned as cursor " +
				"to read the next page. next_cursor is null on the last page.",
			Input:  accountInput(`,"cursor":{"type":"string"}`, ``),
			Output: pageSchema("senders", senderSchema),
			Handle: s.listSenders,
		},
	}
}

// groupSchema is the JSON Schema of a group of messages, and of the summary of a selection.
const groupSchema = `{"type":"object","properties":{"key":{"type":["string","null"]},"messages":{"type":"integer"},` +
	`"threads":{"type":"integer"},"unread":{"type":"integer"},"with_attachments":{"type":"integer"},` +
	`"oldest_at":` + nullableTimestamp + `,"newest_at":` + nullableTimestamp + `},` +
	`"required":["key","messages","threads","unread","with_attachments","oldest_at","newest_at"]}`

// senderSchema is the JSON Schema of one sender's statistics.
const senderSchema = `{"type":"object","properties":{"domain":{"type":"string"},` +
	`"sender_class":{"type":"string","enum":["normal","restricted"]},"messages":{"type":"integer"},` +
	`"first_seen":` + nullableTimestamp + `,"last_seen":` + nullableTimestamp + `,` +
	`"list_id_share":{"type":["number","null"]},` +
	`"label_distribution":{"type":"object","additionalProperties":{"type":"integer"}},` +
	`"display_names":{"type":"array","items":{"type":"string"}},` +
	`"local_part_sample":{"type":"array","items":{"type":"string"}}},` +
	`"required":["domain","sender_class","messages","first_seen","last_seen","list_id_share","label_distribution","display_names","local_part_sample"]}`

// The sorts search_messages takes, and the orders.
var (
	sorts  = []string{"date", "sender", "subject"}
	orders = []string{"descending", "ascending"}
)

func (s Sources) searchMessages(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args struct {
		Query  json.RawMessage `json:"query"`
		Sort   string          `json:"sort"`
		Order  string          `json:"order"`
		Cursor string          `json:"cursor"`
	}
	if err := arguments(input, &args, "query", "sort", "order", "cursor"); err != nil {
		return nil, err
	}
	sel, err := readQuery(account, args.Query)
	if err != nil {
		return nil, err
	}
	sortBy, order := args.Sort, args.Order
	if sortBy == "" {
		sortBy = "date"
	}
	if order == "" {
		order = "descending"
	}
	if !slices.Contains(sorts, sortBy) {
		return nil, Refuse("sort must be date, sender or subject")
	}
	if !slices.Contains(orders, order) {
		return nil, Refuse("order must be descending or ascending")
	}
	filter := sel.text + " sort=" + sortBy + " order=" + order
	from, found, err := decodeCursor(args.Cursor, "search_messages", account, filter)
	if err != nil {
		return nil, err
	}
	params := messages.SearchPageParams{SortBy: sortBy, Descending: order == "descending", FirstPage: !found, PageSize: pageSize}
	if found {
		params.AfterMessageID = from.Key
		switch sortBy {
		case "date":
			at, err := parseCursorTime(from.At)
			if err != nil {
				return nil, err
			}
			params.AfterSentAt = pgtype.Timestamptz{Time: at, Valid: true}
		case "sender":
			params.AfterSender = from.Text
		default:
			params.AfterSubject = from.Text
		}
	}
	p := s.Policy(account)
	var rows []messages.SearchPageRow
	err = tx.Run(ctx, tx.Snapshot(s.DB), account, func(t pgx.Tx) error {
		q := messages.New(t)
		c, err := s.classesFor(ctx, senders.New(t), account, sel.needsClasses(), p)
		if err != nil {
			return err
		}
		withFilter(&params, sel.resolve(c))
		rows, err = q.SearchPage(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]message, 0, len(rows))
	for _, r := range rows {
		out = append(out, present(account, fromRow(messages.MessageRow(r)), p, s.Lookups))
	}
	var next *string
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		pos := position{Listing: "search_messages", Account: account, Filter: filter, Key: last.MessageID}
		switch sortBy {
		case "date":
			pos.At = formatUTC(last.SentAt.Time)
		case "sender":
			pos.Text = last.FromEmail
		default:
			pos.Text = last.Subject.String
		}
		c := encodeCursor(pos)
		next = &c
	}
	return ok(struct {
		Messages   []message `json:"messages"`
		NextCursor *string   `json:"next_cursor"`
	}{out, next})
}

// withFilter sets the filter terms of a search page's parameters from f.
func withFilter(p *messages.SearchPageParams, f messages.SearchSummaryParams) {
	p.AccountID, p.After, p.Before = f.AccountID, f.After, f.Before
	p.FromEmails, p.FromDomains = f.FromEmails, f.FromDomains
	p.Labels, p.ExcludedLabels, p.LabelsWithin = f.Labels, f.ExcludedLabels, f.LabelsWithin
	p.SubjectPatterns, p.Unread, p.Starred, p.HasAttachments = f.SubjectPatterns, f.Unread, f.Starred, f.HasAttachments
	p.Restricted, p.NormalDomains = f.Restricted, f.NormalDomains
}

// classesFor classifies the account's sender domains under p when needed is set, and otherwise returns
// no classes, which no term reads. The domains are read from the sender statistics, which hold exactly
// the domains the account's messages are stored under (ADR-0109).
func (s Sources) classesFor(ctx context.Context, q *senders.Queries, account string, needed bool, p policy.Composed) (classes, error) {
	if !needed {
		return classes{}, nil
	}
	domains, err := q.SenderDomains(ctx, account)
	if err != nil {
		return classes{}, err
	}
	return classifyDomains(domains, p, s.Lookups), nil
}

// group is the counts of one group of messages, or of a whole selection, whose key is null.
type group struct {
	Key             *string `json:"key"`
	Messages        int64   `json:"messages"`
	Threads         int64   `json:"threads"`
	Unread          int64   `json:"unread"`
	WithAttachments int64   `json:"with_attachments"`
	OldestAt        *string `json:"oldest_at"`
	NewestAt        *string `json:"newest_at"`
}

// counted returns a group with key and the counts of a row.
func counted(key *string, messages, threads, unread, attachments int64, oldest, newest pgtype.Timestamptz) group {
	return group{
		Key: key, Messages: messages, Threads: threads, Unread: unread, WithAttachments: attachments,
		OldestAt: timestamp(oldest), NewestAt: timestamp(newest),
	}
}

// groupings are the dimensions count_messages groups by.
var groupings = []string{"sender", "sender_domain", "label", "sender_class", "month"}

func (s Sources) countMessages(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args struct {
		Query   json.RawMessage `json:"query"`
		GroupBy string          `json:"group_by"`
		Cursor  string          `json:"cursor"`
	}
	if err := arguments(input, &args, "query", "group_by", "cursor"); err != nil {
		return nil, err
	}
	sel, err := readQuery(account, args.Query)
	if err != nil {
		return nil, err
	}
	if args.GroupBy != "" && !slices.Contains(groupings, args.GroupBy) {
		return nil, Refuse("group_by must be sender, sender_domain, label, sender_class or month")
	}
	filter := sel.text + " group_by=" + args.GroupBy
	from, found, err := decodeCursor(args.Cursor, "count_messages", account, filter)
	if err != nil {
		return nil, err
	}
	p := s.Policy(account)
	var summary group
	var groups []group
	var next *string
	err = tx.Run(ctx, tx.Snapshot(s.DB), account, func(t pgx.Tx) error {
		q := messages.New(t)
		c, err := s.classesFor(ctx, senders.New(t), account, sel.needsClasses() || args.GroupBy == "sender_class", p)
		if err != nil {
			return err
		}
		base := sel.resolve(c)
		whole, err := q.SearchSummary(ctx, base)
		if err != nil {
			return err
		}
		summary = counted(nil, whole.Messages, whole.Threads, whole.Unread, whole.WithAttachments, whole.OldestAt, whole.NewestAt)
		var sqlPaged bool
		switch args.GroupBy {
		case "sender", "sender_domain":
			sqlPaged = true
			groups, err = s.pagedGroups(ctx, q, args.GroupBy, base, from, found)
		case "label", "month", "sender_class":
			groups, err = s.wholeGroups(ctx, q, args.GroupBy, base, sel, c)
		default:
			groups = []group{}
		}
		if err != nil || args.GroupBy == "" {
			return err
		}
		if !sqlPaged {
			groups = pageOf(groups, from, found)
		}
		if len(groups) == pageSize {
			last := groups[len(groups)-1]
			pos := position{Listing: "count_messages", Account: account, Filter: filter, Count: last.Messages, Null: last.Key == nil}
			if last.Key != nil {
				pos.Key = *last.Key
			}
			c := encodeCursor(pos)
			next = &c
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var groupBy *string
	if args.GroupBy != "" {
		groupBy = &args.GroupBy
	}
	return ok(struct {
		Summary    group   `json:"summary"`
		GroupBy    *string `json:"group_by"`
		Groups     []group `json:"groups"`
		NextCursor *string `json:"next_cursor"`
	}{summary, groupBy, groups, next})
}

// pagedGroups reads one page of the groups by sender address or sender domain, which can be as many as
// the account's senders, so the statement pages them.
func (s Sources) pagedGroups(ctx context.Context, q *messages.Queries, by string, f messages.SearchSummaryParams, from position, found bool) ([]group, error) {
	out := []group{}
	if by == "sender" {
		params := messages.CountBySenderParams{FirstPage: !found, AfterCount: from.Count, AfterKey: from.Key, PageSize: pageSize}
		params.AccountID, params.After, params.Before = f.AccountID, f.After, f.Before
		params.FromEmails, params.FromDomains = f.FromEmails, f.FromDomains
		params.Labels, params.ExcludedLabels, params.LabelsWithin = f.Labels, f.ExcludedLabels, f.LabelsWithin
		params.SubjectPatterns, params.Unread, params.Starred, params.HasAttachments = f.SubjectPatterns, f.Unread, f.Starred, f.HasAttachments
		params.Restricted, params.NormalDomains = f.Restricted, f.NormalDomains
		rows, err := q.CountBySender(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			key := r.FromEmail
			out = append(out, counted(&key, r.Messages, r.Threads, r.Unread, r.WithAttachments, r.OldestAt, r.NewestAt))
		}
		return out, nil
	}
	params := messages.CountBySenderDomainParams{FirstPage: !found, AfterCount: from.Count, AfterKey: from.Key, PageSize: pageSize}
	params.AccountID, params.After, params.Before = f.AccountID, f.After, f.Before
	params.FromEmails, params.FromDomains = f.FromEmails, f.FromDomains
	params.Labels, params.ExcludedLabels, params.LabelsWithin = f.Labels, f.ExcludedLabels, f.LabelsWithin
	params.SubjectPatterns, params.Unread, params.Starred, params.HasAttachments = f.SubjectPatterns, f.Unread, f.Starred, f.HasAttachments
	params.Restricted, params.NormalDomains = f.Restricted, f.NormalDomains
	rows, err := q.CountBySenderDomain(ctx, params)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		key := r.FromDomain
		out = append(out, counted(&key, r.Messages, r.Threads, r.Unread, r.WithAttachments, r.OldestAt, r.NewestAt))
	}
	return out, nil
}

// wholeGroups reads every group by label, month or sender class, in the order a page takes them. The
// groups are as many as the account's labels, the months its mail spans, or the two classes, so the
// service layer pages them.
func (s Sources) wholeGroups(ctx context.Context, q *messages.Queries, by string, f messages.SearchSummaryParams, sel selection, c classes) ([]group, error) {
	out := []group{}
	switch by {
	case "label":
		unlabelled := f
		unlabelled.LabelsWithin = []string{}
		none, err := q.SearchSummary(ctx, unlabelled)
		if err != nil {
			return nil, err
		}
		if none.Messages > 0 {
			out = append(out, counted(nil, none.Messages, none.Threads, none.Unread, none.WithAttachments, none.OldestAt, none.NewestAt))
		}
		rows, err := q.CountByLabel(ctx, messages.CountByLabelParams(f))
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			key := r.Label
			out = append(out, counted(&key, r.Messages, r.Threads, r.Unread, r.WithAttachments, r.OldestAt, r.NewestAt))
		}
	case "month":
		rows, err := q.CountByMonth(ctx, messages.CountByMonthParams(f))
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			key := formatUTC(r.Month.Time)
			out = append(out, counted(&key, r.Messages, r.Threads, r.Unread, r.WithAttachments, r.OldestAt, r.NewestAt))
		}
	default:
		for _, class := range []string{"normal", "restricted"} {
			if sel.class != nil && *sel.class != class {
				continue
			}
			one := f
			one.Restricted = []bool{class == "restricted"}
			one.NormalDomains = c.normal
			row, err := q.SearchSummary(ctx, one)
			if err != nil {
				return nil, err
			}
			if row.Messages > 0 {
				key := class
				out = append(out, counted(&key, row.Messages, row.Threads, row.Unread, row.WithAttachments, row.OldestAt, row.NewestAt))
			}
		}
	}
	slices.SortStableFunc(out, compareGroups)
	return out, nil
}

// compareGroups orders groups as a page takes them, the group whose key is null first, then the one
// with the most messages, then by key.
func compareGroups(a, b group) int {
	switch {
	case (a.Key == nil) != (b.Key == nil):
		if a.Key == nil {
			return -1
		}
		return 1
	case a.Key == nil:
		return 0
	case a.Messages != b.Messages:
		if a.Messages > b.Messages {
			return -1
		}
		return 1
	default:
		return strings.Compare(*a.Key, *b.Key)
	}
}

// pageOf returns the page of groups, in order, that follows the group a cursor names, or the first
// page when it names none.
func pageOf(groups []group, from position, found bool) []group {
	start := 0
	if found {
		last := group{Messages: from.Count}
		if !from.Null {
			key := from.Key
			last.Key = &key
		}
		start = len(groups)
		for i, g := range groups {
			if compareGroups(last, g) < 0 {
				start = i
				break
			}
		}
	}
	end := min(start+pageSize, len(groups))
	return groups[start:end]
}

// senderStats is one sender's statistics as the client surface serves them.
type senderStats struct {
	Domain            string          `json:"domain"`
	SenderClass       string          `json:"sender_class"`
	Messages          int64           `json:"messages"`
	FirstSeen         *string         `json:"first_seen"`
	LastSeen          *string         `json:"last_seen"`
	ListIDShare       *float64        `json:"list_id_share"`
	LabelDistribution json.RawMessage `json:"label_distribution"`
	DisplayNames      []string        `json:"display_names"`
	LocalPartSample   []string        `json:"local_part_sample"`
}

func (s Sources) listSenders(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
	var args cursorArgs
	if err := arguments(input, &args, "cursor"); err != nil {
		return nil, err
	}
	from, found, err := decodeCursor(args.Cursor, "list_senders", account, "")
	if err != nil {
		return nil, err
	}
	params := senders.SenderPageParams{AccountID: account, FirstPage: !found, AfterCount: from.Count, AfterDomain: from.Key, PageSize: pageSize}
	p := s.Policy(account)
	var rows []senders.SenderPageRow
	err = tx.Run(ctx, tx.Snapshot(s.DB), account, func(t pgx.Tx) error {
		var err error
		rows, err = senders.New(t).SenderPage(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]senderStats, 0, len(rows))
	for _, r := range rows {
		// A domain's class is the class of every address at it, and one the classifier cannot read is
		// restricted (ADR-0109).
		restricted := classify.Classify(p, "@"+r.Domain, s.Lookups).Class().Restricted()
		class := "normal"
		if restricted {
			class = "restricted"
		}
		row := senderStats{
			Domain: r.Domain, SenderClass: class, Messages: r.MessageCount,
			FirstSeen: timestamp(r.FirstSeen), LastSeen: timestamp(r.LastSeen),
			LabelDistribution: json.RawMessage(`{}`),
			DisplayNames:      nonNil(r.DisplayNames), LocalPartSample: nonNil(r.LocalPartSample),
		}
		if r.HasListIDRatio.Valid {
			share := float64(r.HasListIDRatio.Float32)
			row.ListIDShare = &share
		}
		var counts map[string]int64
		if json.Unmarshal(r.LabelDistribution, &counts) == nil && counts != nil {
			if text, err := json.Marshal(counts); err == nil {
				row.LabelDistribution = text
			}
		}
		out = append(out, row)
	}
	var next *string
	if len(rows) == pageSize {
		last := rows[len(rows)-1]
		c := encodeCursor(position{Listing: "list_senders", Account: account, Count: last.MessageCount, Key: last.Domain})
		next = &c
	}
	return ok(struct {
		Senders    []senderStats `json:"senders"`
		NextCursor *string       `json:"next_cursor"`
	}{out, next})
}
