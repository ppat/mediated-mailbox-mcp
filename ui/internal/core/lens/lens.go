// Package lens is the dataset endpoint's pure core. It holds the declarative half of a registry entry
// and decides, from that declaration alone, whether a request is one the entry allows (ADR-0057,
// docs/UI.md sections 5 and 17). Every request the entry does not declare is refused here, before any
// statement runs, so the endpoint never becomes an open query surface.
package lens

import (
	"slices"
	"strconv"
	"strings"
)

// RowsPerPage is the one page size the endpoint serves (docs/UI.md section 5).
const RowsPerPage = 50

// Levels of the zoom ladder the dataset endpoint answers. L4 is the row-detail endpoint's.
const (
	Summary      = 0
	Distribution = 1
	Cohort       = 2
	Rows         = 3
)

// Presets are the range presets, in the order the range control offers them.
func Presets() []string { return []string{"24h", "7d", "30d", "90d", "all"} }

// Dimension is one column a dataset declares.
type Dimension struct {
	Name string
	// Storage is the stored type, text, number or time, or date for a time column's day bucket. A
	// filter on a number names whole numbers and a filter on a date names UTC dates, each YYYY-MM-DD.
	Storage string
	// Groupable, Filterable and Sortable say what a request may do with the dimension.
	Groupable  bool
	Filterable bool
	Sortable   bool
	// Wording is the dimension's name as shown, and NullWording the wording of its null group where
	// one exists.
	Wording     string
	NullWording string
	// Empty says the dimension can hold a value stored empty, whose group the filter value empty names.
	Empty bool
	// Values is the closed set of the column's values as stored, keying the browser's wording table.
	Values []string
}

// Sort is a sort column and its direction.
type Sort struct {
	Column     string
	Descending bool
}

// String is the sort as the URL writes it.
func (s Sort) String() string {
	if s.Descending {
		return s.Column + ",desc"
	}
	return s.Column + ",asc"
}

// Defaults are what a view shows before the operator changes it. Filters are applied by the browser's
// router when it canonicalizes a URL and are never applied by the endpoint, because an absent filter
// means no filter there (docs/UI.md section 17.2).
type Defaults struct {
	Group   string
	Level   int
	Range   string
	Sort    Sort
	Filters map[string][]string
}

// Descriptor is the declarative half of a registry entry.
type Descriptor struct {
	Name string
	// Parent names the required parent filter of a nested dataset, empty for none.
	Parent string
	// Ranged says the dataset takes range=, over RangeColumn.
	Ranged      bool
	RangeColumn string
	Dimensions  []Dimension
	Default     Defaults
	// Identity declares the row-detail endpoint's row identity for a dataset that declares a provenance
	// query, and is nil for any other (docs/UI.md section 17.1).
	Identity *RowIdentity
}

// RowIdentity is the identity a row-detail path carries. Storage is text or number, as a dimension's.
type RowIdentity struct {
	Name    string
	Storage string
}

// Dimension returns the dimension named name.
func (d Descriptor) Dimension(name string) (Dimension, bool) {
	i := slices.IndexFunc(d.Dimensions, func(x Dimension) bool { return x.Name == name })
	if i < 0 {
		return Dimension{}, false
	}
	return d.Dimensions[i], true
}

// Filter is one dimension filter. Values holds one value for equality and several for any-of.
// Exclude makes it exclusion of its one value.
type Filter struct {
	Dimension string
	Values    []string
	Exclude   bool
}

// Range is range= parsed. Preset is one of Presets, or empty when From and To hold two UTC dates,
// each YYYY-MM-DD, To inclusive.
type Range struct {
	Preset string
	From   string
	To     string
}

// String is the range as the URL writes it.
func (r Range) String() string {
	if r.Preset != "" {
		return r.Preset
	}
	return r.From + "," + r.To
}

// Request is a dataset request the entry allows.
type Request struct {
	Dataset string
	Level   int
	// Group is set at levels 1 and 2 only.
	Group string
	// Range is set when the dataset is ranged.
	Range   Range
	Filters []Filter
	// ParentID is the parent filter's value for a nested dataset.
	ParentID string
	Sort     Sort
	// Page is 1-based and read at level 3 only.
	Page int
}

// Filter returns the request's filter on the named dimension.
func (r Request) Filter(dimension string) (Filter, bool) {
	i := slices.IndexFunc(r.Filters, func(f Filter) bool { return f.Dimension == dimension })
	if i < 0 {
		return Filter{}, false
	}
	return r.Filters[i], true
}

// Refusal is a request the entry does not allow, with the error contract's code.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Message }

func refuse(code, format string, args ...string) *Refusal {
	msg := format
	for _, a := range args {
		msg = strings.Replace(msg, "%s", a, 1)
	}
	return &Refusal{Code: code, Message: msg}
}

// The common parameters, which every other parameter must not be named.
const (
	paramDataset = "dataset"
	paramLevel   = "level"
	paramGroup   = "group"
	paramRange   = "range"
	paramSort    = "sort"
	paramPage    = "page"
)

// Parameters are the common parameters, which no dimension and no parent filter may be named, since a
// URL could not tell the two apart.
func Parameters() []string {
	return []string{paramDataset, paramLevel, paramGroup, paramRange, paramSort, paramPage}
}

// Parse decides whether query, the request's query parameters by name, is a request one of the
// catalogue's entries allows, and returns it. A parameter left out takes the entry's default, apart
// from filters. Anything the entry does not declare is refused.
func Parse(catalogue []Descriptor, query map[string][]string) (Request, error) {
	for name, values := range query {
		if len(values) != 1 {
			return Request{}, refuse("repeated_parameter", "the parameter %s is given more than once", name)
		}
	}
	one := func(name string) (string, bool) {
		v, ok := query[name]
		if !ok {
			return "", false
		}
		return v[0], true
	}
	name, ok := one(paramDataset)
	if !ok || name == "" {
		return Request{}, refuse("missing_dataset", "the request names no dataset")
	}
	i := slices.IndexFunc(catalogue, func(d Descriptor) bool { return d.Name == name })
	if i < 0 {
		return Request{}, refuse("unknown_dataset", "the registry declares no dataset %s", strconv.Quote(name))
	}
	d := catalogue[i]
	req := Request{Dataset: d.Name, Level: d.Default.Level, Sort: d.Default.Sort, Page: 1}

	if v, ok := one(paramLevel); ok {
		level, err := strconv.Atoi(v)
		if err != nil || level < Summary || level > Rows {
			return Request{}, refuse("invalid_level", "level %s is not 0, 1, 2 or 3", strconv.Quote(v))
		}
		req.Level = level
	}
	// A group the request names is checked at every level, since naming what the entry does not
	// declare is refused wherever it appears. Levels 0 and 3 then ignore it (docs/UI.md section 5).
	group, hasGroup := one(paramGroup)
	if hasGroup {
		dim, ok := d.Dimension(group)
		if !ok || !dim.Groupable {
			return Request{}, refuse("unknown_group", "the dataset %s declares no groupable dimension %s", d.Name, strconv.Quote(group))
		}
	} else {
		group = d.Default.Group
	}
	if req.Level == Distribution || req.Level == Cohort {
		if group == "" {
			return Request{}, refuse("missing_group", "level %s requires exactly one group", strconv.Itoa(req.Level))
		}
		req.Group = group
	}

	if v, ok := one(paramRange); ok || d.Ranged {
		if !d.Ranged {
			return Request{}, refuse("unknown_dimension", "the dataset %s takes no range", d.Name)
		}
		if !ok {
			v = d.Default.Range
		}
		r, err := parseRange(v)
		if err != nil {
			return Request{}, err
		}
		req.Range = r
	}

	if v, ok := one(paramSort); ok {
		s, err := parseSort(d, v)
		if err != nil {
			return Request{}, err
		}
		req.Sort = s
	}

	if v, ok := one(paramPage); ok {
		page, err := strconv.Atoi(v)
		if err != nil || page < 1 || strconv.Itoa(page) != v {
			return Request{}, refuse("invalid_page", "page %s is not a whole number from 1", strconv.Quote(v))
		}
		req.Page = page
	}

	var filterNames []string
	for key := range query {
		switch key {
		case paramDataset, paramLevel, paramGroup, paramRange, paramSort, paramPage:
			continue
		}
		filterNames = append(filterNames, key)
	}
	slices.Sort(filterNames)
	for _, key := range filterNames {
		value := query[key][0]
		if d.Parent != "" && key == d.Parent {
			if value == "" {
				return Request{}, refuse("invalid_filter", "the parent filter %s is empty", key)
			}
			req.ParentID = value
			continue
		}
		dim, ok := d.Dimension(key)
		if !ok || !dim.Filterable {
			return Request{}, refuse("unknown_dimension", "the dataset %s declares no filter %s", d.Name, strconv.Quote(key))
		}
		f, err := parseFilter(dim, value)
		if err != nil {
			return Request{}, err
		}
		req.Filters = append(req.Filters, f)
	}
	if d.Parent != "" && req.ParentID == "" {
		return Request{}, refuse("missing_parent", "the dataset %s requires the parent filter %s", d.Name, d.Parent)
	}
	if req.Level == Cohort && len(req.Filters) == 0 {
		return Request{}, refuse("missing_filter", "level 2 requires at least one dimension filter")
	}
	return req, nil
}

// parseSort reads sort=column,asc or sort=column,desc, whose column the entry declares sortable.
func parseSort(d Descriptor, v string) (Sort, error) {
	column, direction, ok := strings.Cut(v, ",")
	if !ok || direction != "asc" && direction != "desc" {
		return Sort{}, refuse("invalid_sort", "sort %s is not column,asc or column,desc", strconv.Quote(v))
	}
	dim, found := d.Dimension(column)
	if !found || !dim.Sortable {
		return Sort{}, refuse("unknown_sort", "the dataset %s declares no sortable column %s", d.Name, strconv.Quote(column))
	}
	return Sort{Column: column, Descending: direction == "desc"}, nil
}

// None is the filter value naming a dimension's null group, where the dimension has one.
const None = "none"

// Empty is the filter value naming the group of a value stored empty, where the dimension can hold
// one. The statements compare it as the empty string.
const Empty = "empty"

// parseFilter reads the filter grammar. dim=value is equality, dim=a,b any of, dim=!value exclusion.
// none names a dimension's null group, and is refused on a dimension that has none. empty names the
// group of a value stored empty, and is refused on a dimension that cannot hold one. Any other value
// of a number or a date dimension must be one, so a value no statement could compare is refused before
// any runs.
func parseFilter(dim Dimension, value string) (Filter, error) {
	f := Filter{Dimension: dim.Name}
	if rest, ok := strings.CutPrefix(value, "!"); ok {
		if rest == "" || strings.Contains(rest, ",") {
			return Filter{}, refuse("invalid_filter", "the exclusion %s names one value", strconv.Quote(value))
		}
		f.Values, f.Exclude = []string{rest}, true
	} else {
		f.Values = strings.Split(value, ",")
		if slices.Contains(f.Values, "") {
			return Filter{}, refuse("invalid_filter", "the filter %s holds an empty value", dim.Name)
		}
	}
	for _, v := range f.Values {
		if v == None {
			if dim.NullWording == "" {
				return Filter{}, refuse("invalid_filter", "the filter %s names none, and the dimension has no null group", dim.Name)
			}
			continue
		}
		if v == Empty {
			if !dim.Empty {
				return Filter{}, refuse("invalid_filter", "the filter %s names empty, and the dimension holds no value stored empty", dim.Name)
			}
			continue
		}
		if !ofStorage(dim.Storage, v) {
			return Filter{}, refuse("invalid_filter", "the filter %s holds %s, which is not a value of its kind", dim.Name, strconv.Quote(v))
		}
	}
	return f, nil
}

// ofStorage reports whether v is a value a column of the stored type can hold. Text holds any.
func ofStorage(storage, v string) bool {
	switch storage {
	case "number":
		return isWhole(v)
	case "date":
		return isDate(v)
	}
	return true
}

// isWhole reports whether v is a whole number written in digits alone, as a stored integer prints.
func isWhole(v string) bool {
	n, err := strconv.ParseInt(v, 10, 32)
	return err == nil && n >= 0 && strconv.FormatInt(n, 10) == v
}

// ofRowStorage reports whether v is a row identity of the stored type. A number identity is a big
// serial, so it may be any whole number a 64-bit integer holds.
func ofRowStorage(storage, v string) bool {
	if storage != "number" {
		return true
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return err == nil && n >= 0 && strconv.FormatInt(n, 10) == v
}

// RowRequest is a row-detail request the entry allows.
type RowRequest struct {
	Dataset  string
	Row      string
	ParentID string
}

// ParseRow decides whether a row-detail request, the dataset's name, the row's identity from the path
// and the query parameters by name, is one the catalogue allows. The dataset must declare a row
// identity, the identity must be of its kind, and the query holds the parent filter of a nested
// dataset and nothing else (docs/UI.md section 17.1).
func ParseRow(catalogue []Descriptor, dataset, row string, query map[string][]string) (RowRequest, error) {
	i := slices.IndexFunc(catalogue, func(d Descriptor) bool { return d.Name == dataset && d.Identity != nil })
	if i < 0 {
		return RowRequest{}, refuse("unknown_dataset", "the registry declares no row detail for %s", strconv.Quote(dataset))
	}
	d := catalogue[i]
	if row == "" || !ofRowStorage(d.Identity.Storage, row) {
		return RowRequest{}, refuse("invalid_row", "%s is not a %s of %s", strconv.Quote(row), d.Identity.Name, d.Name)
	}
	req := RowRequest{Dataset: d.Name, Row: row}
	for name, values := range query {
		if name != d.Parent || d.Parent == "" {
			return RowRequest{}, refuse("unknown_parameter", "the row detail of %s takes no parameter %s", d.Name, strconv.Quote(name))
		}
		if len(values) != 1 {
			return RowRequest{}, refuse("repeated_parameter", "the parameter %s is given more than once", name)
		}
		if values[0] == "" {
			return RowRequest{}, refuse("invalid_filter", "the parent filter %s is empty", name)
		}
		req.ParentID = values[0]
	}
	if d.Parent != "" && req.ParentID == "" {
		return RowRequest{}, refuse("missing_parent", "the dataset %s requires the parent filter %s", d.Name, d.Parent)
	}
	return req, nil
}

// parseRange reads a preset or two UTC dates, from before or on to.
func parseRange(v string) (Range, error) {
	if slices.Contains(Presets(), v) {
		return Range{Preset: v}, nil
	}
	from, to, ok := strings.Cut(v, ",")
	if !ok || !isDate(from) || !isDate(to) || to < from {
		return Range{}, refuse("invalid_range", "range %s is not a preset or two UTC dates from,to", strconv.Quote(v))
	}
	return Range{From: from, To: to}, nil
}

// isDate reports whether s is a calendar date written YYYY-MM-DD.
func isDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range s {
		if i != 4 && i != 7 && (c < '0' || c > '9') {
			return false
		}
	}
	year, errY := strconv.Atoi(s[0:4])
	month, errM := strconv.Atoi(s[5:7])
	day, errD := strconv.Atoi(s[8:10])
	if errY != nil || errM != nil || errD != nil || year < 1 || month < 1 || month > 12 || day < 1 {
		return false
	}
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[month-1]
	if month == 2 && year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		days = 29
	}
	return day <= days
}
