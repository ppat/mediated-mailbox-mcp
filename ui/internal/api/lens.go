package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	runclassification "github.com/ppat/mediated-mailbox-mcp/db/jobruns/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	senderclassification "github.com/ppat/mediated-mailbox-mcp/db/senders/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// figuresResponse is a level 0 answer (docs/UI.md section 17.1).
type figuresResponse struct {
	Account string            `json:"account"`
	Dataset string            `json:"dataset"`
	Level   int               `json:"level"`
	AsOf    string            `json:"as_of"`
	Filters map[string]string `json:"filters"`
	Figures []registry.Figure `json:"figures"`
}

// rowsResponse is a level 3 answer, one page of rows.
type rowsResponse struct {
	Account string            `json:"account"`
	Dataset string            `json:"dataset"`
	Level   int               `json:"level"`
	AsOf    string            `json:"as_of"`
	Filters map[string]string `json:"filters"`
	Sort    string            `json:"sort"`
	Page    int               `json:"page"`
	Pages   int64             `json:"pages"`
	Total   any               `json:"total"`
	Rows    any               `json:"rows"`
}

// groupsResponse is a level 1 or 2 answer, every group of the request's grouped dimension.
type groupsResponse struct {
	Account string            `json:"account"`
	Dataset string            `json:"dataset"`
	Level   int               `json:"level"`
	AsOf    string            `json:"as_of"`
	Group   string            `json:"group"`
	Filters map[string]string `json:"filters"`
	Total   any               `json:"total"`
	Rows    any               `json:"rows"`
}

// countTotal is the total of a dataset that is not message-derived, the count alone, and
// sensitiveTotal a message-derived dataset's, with how many rows are restricted and how many flagged.
type countTotal struct {
	Count int64 `json:"count"`
}

type sensitiveTotal struct {
	Count      int64 `json:"count"`
	Restricted int64 `json:"restricted"`
	Flagged    int64 `json:"flagged"`
}

func totalOf(d registry.Dataset, t registry.Total) any {
	if d.MessageDerived {
		return sensitiveTotal{Count: t.Count, Restricted: t.Restricted, Flagged: t.Flagged}
	}
	return countTotal{Count: t.Count}
}

func totalType(d registry.Dataset) schema.Type {
	if d.MessageDerived {
		return schema.Obj("SensitiveCount",
			schema.F("count", schema.Int()),
			schema.F("restricted", schema.Int()),
			schema.F("flagged", schema.Int()),
		)
	}
	return schema.Obj("Count", schema.F("count", schema.Int()))
}

// keyType is a group key's declaration, the dimension's value as stored, null for its null group.
func keyType(dim lens.Dimension) schema.Type {
	t := schema.Str()
	if dim.Storage == "number" {
		t = schema.Int()
	}
	if dim.NullWording != "" {
		t = schema.Null(t)
	}
	return t
}

// LensResponse is the dataset endpoint's declaration, one figures shape, one row page per dataset and
// one groups answer per groupable dimension of each.
func LensResponse(datasets []registry.Dataset) schema.Type {
	names := make([]string, len(datasets))
	for i, d := range datasets {
		names[i] = d.Name
	}
	variants := []schema.Type{schema.Obj("LensFigures",
		schema.F("account", schema.Str()),
		schema.F("dataset", schema.Str(names...)),
		schema.F("level", schema.Int()),
		schema.F("as_of", schema.Time()),
		schema.F("filters", schema.MapOf(schema.Str())),
		schema.F("figures", schema.ArrayOf(registry.FigureType())),
	)}
	for _, d := range datasets {
		variants = append(variants, schema.Obj(pascal(d.Name)+"Page",
			schema.F("account", schema.Str()),
			schema.F("dataset", schema.Str(d.Name)),
			schema.F("level", schema.Int()),
			schema.F("as_of", schema.Time()),
			schema.F("filters", schema.MapOf(schema.Str())),
			schema.F("sort", schema.Str()),
			schema.F("page", schema.Int()),
			schema.F("pages", schema.Int()),
			schema.F("total", totalType(d)),
			schema.F("rows", schema.ArrayOf(d.Row)),
		))
		for _, dim := range d.Dimensions {
			if !dim.Groupable {
				continue
			}
			name := pascal(d.Name) + "By" + pascal(dim.Name)
			group := []schema.Field{
				schema.F("key", schema.Obj(name+"Key", schema.F(dim.Name, keyType(dim)))),
				schema.F("count", schema.Int()),
			}
			if d.MessageDerived {
				group = append(group, schema.F("restricted", schema.Int()), schema.F("flagged", schema.Int()))
			}
			variants = append(variants, schema.Obj(name,
				schema.F("account", schema.Str()),
				schema.F("dataset", schema.Str(d.Name)),
				schema.F("level", schema.Int()),
				schema.F("as_of", schema.Time()),
				schema.F("group", schema.Str(dim.Name)),
				schema.F("filters", schema.MapOf(schema.Str())),
				schema.F("total", totalType(d)),
				schema.F("rows", schema.ArrayOf(schema.Obj(name+"Group", group...))),
			))
		}
	}
	return schema.OneOf(variants...)
}

// pascal is a name written in PascalCase as a component name's prefix, error_class as ErrorClass.
func pascal(name string) string {
	var b strings.Builder
	for _, part := range strings.Split(name, "_") {
		if part != "" {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

// lens is the dataset endpoint. The pure core decides whether the registry declares the request, and
// a request it does not is refused before any statement runs, the account check's included, which
// comes after it (ADR-0057).
func (s *Server) lens(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	requestInfo(r.Context()).account = account
	req, err := lens.Parse(s.descriptors, r.URL.Query())
	var refusal *lens.Refusal
	if errors.As(err, &refusal) {
		writeFailure(w, r, clientFault(http.StatusBadRequest, refusal.Code, refusal.Message))
		return
	}
	if err != nil {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, uiFault())
		return
	}
	if !s.admit(w, r) {
		return
	}
	var dataset registry.Dataset
	for _, d := range s.opts.Datasets {
		if d.Name == req.Dataset {
			dataset = d
		}
	}
	started := time.Now()
	var body any
	err = tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		// as_of is the time the read transaction began, and the range is read against it.
		now := s.opts.Clock()
		read := registry.Read{Account: account, Request: req, Bounds: bounds(req.Range, now), AsOf: now}
		// The statements are built here, from the transaction that set the account (ADR-0047).
		q := registry.Queries{
			Plans: reorgplans.New(t), Candidates: policycandidates.New(t),
			Runs: jobruns.New(t), Failures: runclassification.New(t), Audit: auditlog.New(t),
			Rules: manage.New(t), Changes: policychanges.New(t), Senders: senderclassification.New(t), Accounts: accounts.New(t),
		}
		figures, total, err := dataset.Summary(r.Context(), q, read)
		if err != nil {
			return err
		}
		switch req.Level {
		case lens.Summary:
			body = figuresResponse{Account: account, Dataset: req.Dataset, Level: req.Level, AsOf: registry.Stamp(now), Filters: filters(req), Figures: figures}
			return nil
		case lens.Rows:
			rows, err := dataset.Rows(r.Context(), q, read)
			if err != nil {
				return err
			}
			body = rowsResponse{
				Account: account, Dataset: req.Dataset, Level: req.Level, AsOf: registry.Stamp(now), Filters: filters(req),
				Sort: req.Sort.String(), Page: req.Page, Pages: registry.Pages(total.Count), Total: totalOf(dataset, total), Rows: rows,
			}
			return nil
		}
		// The pure core admits levels 1 and 2 only with a groupable dimension, and the statement-set
		// check requires an aggregate statement for every one.
		aggregate, ok := dataset.Aggregates[req.Group]
		if !ok {
			return errUnservedLevel
		}
		groups, err := aggregate(r.Context(), q, read)
		if err != nil {
			return err
		}
		body = groupsResponse{
			Account: account, Dataset: req.Dataset, Level: req.Level, AsOf: registry.Stamp(now), Group: req.Group,
			Filters: filters(req), Total: totalOf(dataset, total), Rows: groups,
		}
		return nil
	})
	s.metrics.readDuration.WithLabelValues(req.Dataset, strconv.Itoa(req.Level)).Observe(time.Since(started).Seconds())
	if errors.Is(err, errUnservedLevel) {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, uiFault())
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, body)
}

var errUnservedLevel = errors.New("the registry admitted a group no statement serves")

// filters are the request's applied filters as the URL writes them, the range among them.
func filters(req lens.Request) map[string]string {
	out := map[string]string{}
	if req.Range != (lens.Range{}) {
		out["range"] = req.Range.String()
	}
	for _, f := range req.Filters {
		v := strings.Join(f.Values, ",")
		if f.Exclude {
			v = "!" + v
		}
		out[f.Dimension] = v
	}
	return out
}

// bounds turns a range into instants against now. A preset ends at no bound, since nothing is
// recorded after now, and two dates run from the first's midnight to the midnight after the second,
// in UTC.
func bounds(r lens.Range, now time.Time) registry.Bounds {
	ago := func(d time.Duration) registry.Bounds {
		start := now.Add(-d)
		return registry.Bounds{Start: &start}
	}
	switch r.Preset {
	case "24h":
		return ago(24 * time.Hour)
	case "7d":
		return ago(7 * 24 * time.Hour)
	case "30d":
		return ago(30 * 24 * time.Hour)
	case "90d":
		return ago(90 * 24 * time.Hour)
	case "all", "":
		if r.From == "" {
			return registry.Bounds{}
		}
	}
	from, errFrom := time.Parse(time.DateOnly, r.From)
	to, errTo := time.Parse(time.DateOnly, r.To)
	if errFrom != nil || errTo != nil {
		// The pure core admits only calendar dates, so both parse.
		return registry.Bounds{}
	}
	end := to.AddDate(0, 0, 1)
	return registry.Bounds{Start: &from, End: &end}
}

// rowDetail is the row-detail endpoint of one dataset that declares a provenance query (docs/UI.md
// section 17.1). The pure core decides whether the registry declares the request, and a request it
// does not is refused before any statement runs, the account check's included, which comes after it.
func (s *Server) rowDetail(dataset registry.Dataset) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		account := r.PathValue("account")
		requestInfo(r.Context()).account = account
		req, err := lens.ParseRow(s.descriptors, dataset.Name, r.PathValue("row"), r.URL.Query())
		var refusal *lens.Refusal
		if errors.As(err, &refusal) {
			writeFailure(w, r, clientFault(http.StatusBadRequest, refusal.Code, refusal.Message))
			return
		}
		if err != nil {
			requestInfo(r.Context()).err = err
			writeFailure(w, r, uiFault())
			return
		}
		if !s.admit(w, r) {
			return
		}
		started := time.Now()
		var body any
		err = tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
			var err error
			q := registry.Queries{
				Plans: reorgplans.New(t), Candidates: policycandidates.New(t),
				Runs: jobruns.New(t), Failures: runclassification.New(t), Audit: auditlog.New(t),
				Rules: manage.New(t), Changes: policychanges.New(t), Senders: senderclassification.New(t), Accounts: accounts.New(t),
			}
			body, err = dataset.Detail(r.Context(), q, account, req, s.opts.Clock())
			return err
		})
		s.metrics.readDuration.WithLabelValues(dataset.Name, "4").Observe(time.Since(started).Seconds())
		if errors.Is(err, registry.ErrNoRow) {
			writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_row", "the account holds no such row"))
			return
		}
		if err != nil {
			s.databaseFailure(w, r, err)
			return
		}
		writeJSON(w, r, body)
	}
}
