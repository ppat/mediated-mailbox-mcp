package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
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
	Total   total             `json:"total"`
	Rows    any               `json:"rows"`
}

// total is a page's total. Datasets that are not message-derived carry the count alone.
type total struct {
	Count int64 `json:"count"`
}

// LensResponse is the dataset endpoint's declaration, one figures shape and one row page per
// dataset. Levels 1 and 2 join it with the first dataset that declares a groupable dimension.
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
			schema.F("total", schema.Obj("Count", schema.F("count", schema.Int()))),
			schema.F("rows", schema.ArrayOf(d.Row)),
		))
	}
	return schema.OneOf(variants...)
}

// pascal is a dataset name as a component name's prefix.
func pascal(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
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
		read := registry.Read{Account: account, Request: req, Bounds: bounds(req.Range, now)}
		q := registry.Queries{Plans: reorgplans.New(t), Candidates: policycandidates.New(t)}
		figures, count, err := dataset.Summary(r.Context(), q, read)
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
				Sort: req.Sort.String(), Page: req.Page, Pages: registry.Pages(count), Total: total{Count: count}, Rows: rows,
			}
			return nil
		}
		// The pure core admits levels 1 and 2 only for a groupable dimension, and no dataset declares
		// one yet, so no request reaches here.
		return errUnservedLevel
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

var errUnservedLevel = errors.New("the registry admitted a level no statement serves")

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
