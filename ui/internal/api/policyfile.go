package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// maxPolicyFile is the largest policy file an import reads, and maxImportBody the largest request body
// carrying one, which allows for the file's escaping in JSON (docs/UI.md section 8.7).
const (
	maxPolicyFile = 1 << 20
	maxImportBody = 8 << 20
)

// The problems of a file that is not the form at all, and of one over the size an import reads.
const (
	kindNotTheForm = "not_the_form"
	kindTooLarge   = "too_large"
)

// fileRule is one rule of a policy file, with the line it starts on.
type fileRule struct {
	rule  rules.Rule
	class string
	line  int
}

// fileProblem is a file a reader refuses before any rule is checked, with the line it fails at.
type fileProblem struct {
	kind string
	line int
}

// parseFile reads a policy file, ADR-0004's form, a mapping holding rules alone, a list of rules each
// holding id, domain_suffix and class and nothing else (docs/UI.md section 8.7). A file not of this
// form is refused whole, with the line it fails at.
func parseFile(text string) ([]fileRule, *fileProblem) {
	if len(text) > maxPolicyFile {
		return nil, &fileProblem{kind: kindTooLarge}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		var line int
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			line = yamlErrorLine(err.Error())
		}
		return nil, &fileProblem{kind: kindNotTheForm, line: line}
	}
	if len(doc.Content) != 1 {
		return nil, &fileProblem{kind: kindNotTheForm, line: 1}
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode || len(top.Content) != 2 || top.Content[0].Value != "rules" || top.Content[1].Kind != yaml.SequenceNode {
		return nil, &fileProblem{kind: kindNotTheForm, line: top.Line}
	}
	var out []fileRule
	for _, item := range top.Content[1].Content {
		if item.Kind != yaml.MappingNode {
			return nil, &fileProblem{kind: kindNotTheForm, line: item.Line}
		}
		r := fileRule{line: item.Line}
		seen := map[string]bool{}
		for i := 0; i+1 < len(item.Content); i += 2 {
			key, value := item.Content[i], item.Content[i+1]
			if seen[key.Value] {
				return nil, &fileProblem{kind: kindNotTheForm, line: key.Line}
			}
			seen[key.Value] = true
			switch {
			case key.Value == "id" && value.Kind == yaml.ScalarNode && value.Tag == "!!str":
				r.rule.ID = value.Value
			case key.Value == "class" && value.Kind == yaml.ScalarNode && value.Tag == "!!str":
				r.class = value.Value
			case key.Value == "domain_suffix" && value.Kind == yaml.SequenceNode:
				r.rule.Suffixes = []string{}
				for _, suffix := range value.Content {
					if suffix.Kind != yaml.ScalarNode || suffix.Tag != "!!str" {
						return nil, &fileProblem{kind: kindNotTheForm, line: suffix.Line}
					}
					r.rule.Suffixes = append(r.rule.Suffixes, suffix.Value)
				}
			default:
				return nil, &fileProblem{kind: kindNotTheForm, line: key.Line}
			}
		}
		if !seen["id"] || !seen["class"] || !seen["domain_suffix"] {
			return nil, &fileProblem{kind: kindNotTheForm, line: item.Line}
		}
		out = append(out, r)
	}
	return out, nil
}

// yamlErrorLine reads the line a YAML syntax error names, 0 when it names none.
func yamlErrorLine(msg string) int {
	_, rest, ok := strings.Cut(msg, "line ")
	if !ok {
		return 0
	}
	n := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// checkFile checks every rule of a file with the checks of a write, over the file's rules as the
// scope's rules once imported, and names each problem with its rule and line (ADR-0110).
func checkFile(scope string, file []fileRule) []ProblemAnswer {
	list := make([]rules.Rule, len(file))
	classes := make([]string, len(file))
	for i, f := range file {
		list[i], classes[i] = f.rule, f.class
	}
	var out []ProblemAnswer
	for _, p := range rules.CheckClasses(scope, list, classes) {
		line := file[p.Rule].line
		a := ProblemAnswer{Kind: string(p.Kind), RuleID: file[p.Rule].rule.ID, Line: &line}
		if p.Suffix != "" {
			suffix := p.Suffix
			a.Suffix = &suffix
		}
		out = append(out, a)
	}
	return out
}

// exportFile writes a scope's rules as a policy file, sorted by identifier, each with its identifier,
// its suffixes and its class and nothing else (docs/UI.md section 8.7).
func exportFile(stored []registry.StoredRule) ([]byte, error) {
	sorted := slices.SortedFunc(slices.Values(stored), func(a, b registry.StoredRule) int { return strings.Compare(a.ID, b.ID) })
	list := &yaml.Node{Kind: yaml.SequenceNode}
	for _, r := range sorted {
		suffixes := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, s := range r.Suffixes {
			suffixes.Content = append(suffixes.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s})
		}
		list.Content = append(list.Content, &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "id"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.ID},
			{Kind: yaml.ScalarNode, Value: "domain_suffix"},
			suffixes,
			{Kind: yaml.ScalarNode, Value: "class"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.Class},
		}})
	}
	doc := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "rules"}, list}}
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// computedAgainst is the canonical form of a scope's stored rules a preview was computed against,
// every rule's identifier, class and suffixes in identifier order. An import compares it with the
// scope's rules read in its own transaction, so it carries the rules themselves rather than anything
// weaker (docs/UI.md section 8.7).
func computedAgainst(stored []registry.StoredRule) string {
	type entry struct {
		ID       string   `json:"id"`
		Class    string   `json:"class"`
		Suffixes []string `json:"suffixes"`
	}
	sorted := slices.SortedFunc(slices.Values(stored), func(a, b registry.StoredRule) int { return strings.Compare(a.ID, b.ID) })
	out := make([]entry, len(sorted))
	for i, r := range sorted {
		out[i] = entry{ID: r.ID, Class: r.Class, Suffixes: nonNil(r.Suffixes)}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

// ImportRequest previews a file, and with the stored rules its preview was computed against and the
// confirmation of its lifts, applies it (docs/UI.md section 17.4).
type ImportRequest struct {
	File            string  `json:"file"`
	ComputedAgainst *string `json:"computed_against,omitempty"`
	Confirmation    *string `json:"confirmation,omitempty"`
}

func previewRequestType() schema.Type {
	return schema.Obj("ImportPreviewRequest", schema.F("file", schema.Str()))
}

func importRequestType() schema.Type {
	return schema.Obj("ImportRequest",
		schema.F("file", schema.Str()),
		schema.F("computed_against", schema.Str()),
		schema.F("confirmation", schema.Null(schema.Str())),
	)
}

// LiftedRule is a rule an import lifts, with what it alone releases in the account for an account's
// scope, null for the base scope.
type LiftedRule struct {
	RuleID   string           `json:"rule_id"`
	Suffixes []string         `json:"suffixes"`
	Released *registry.Counts `json:"released"`
}

// EditedRule is a rule an import edits, with the suffixes it adds and removes and, for an account's
// scope, what each removed suffix alone releases.
type EditedRule struct {
	RuleID  string          `json:"rule_id"`
	Before  []string        `json:"before"`
	After   []string        `json:"after"`
	Added   []string        `json:"added"`
	Removed []RemovedSuffix `json:"removed"`
}

// RemovedSuffix is one suffix an import removes from a rule.
type RemovedSuffix struct {
	Suffix   string           `json:"suffix"`
	Released *registry.Counts `json:"released"`
}

// Preview is what an import would change, lifts first (docs/UI.md section 8.7). Lifts counts the
// restrictions it lifts, which its confirmation is typed as, Released is what the whole import releases
// in the account for an account's scope, Accounts every account a base import's lifts reach, and
// ComputedAgainst the stored rules it was computed against.
type Preview struct {
	Added           []RuleAnswer     `json:"added"`
	Edited          []EditedRule     `json:"edited"`
	Lifted          []LiftedRule     `json:"lifted"`
	Unchanged       int              `json:"unchanged"`
	Lifts           int              `json:"lifts"`
	Released        *registry.Counts `json:"released"`
	Accounts        []string         `json:"accounts"`
	ComputedAgainst string           `json:"computed_against"`
}

func previewType() schema.Type {
	counts := schema.Obj("Counts", schema.F("senders", schema.Int()), schema.F("messages", schema.Int()))
	return schema.Obj("ImportPreview",
		schema.F("added", schema.ArrayOf(ruleAnswerType())),
		schema.F("edited", schema.ArrayOf(schema.Obj("EditedRule",
			schema.F("rule_id", schema.Str()),
			schema.F("before", schema.ArrayOf(schema.Str())),
			schema.F("after", schema.ArrayOf(schema.Str())),
			schema.F("added", schema.ArrayOf(schema.Str())),
			schema.F("removed", schema.ArrayOf(schema.Obj("RemovedSuffix",
				schema.F("suffix", schema.Str()),
				schema.F("released", schema.Null(counts)),
			))),
		))),
		schema.F("lifted", schema.ArrayOf(schema.Obj("LiftedRule",
			schema.F("rule_id", schema.Str()),
			schema.F("suffixes", schema.ArrayOf(schema.Str())),
			schema.F("released", schema.Null(counts)),
		))),
		schema.F("unchanged", schema.Int()),
		schema.F("lifts", schema.Int()),
		schema.F("released", schema.Null(counts)),
		schema.F("accounts", schema.ArrayOf(schema.Str())),
		schema.F("computed_against", schema.Str()),
	)
}

// Imported is an applied import's answer, what it added, edited and lifted, from which the screen words
// its outcome and puts the lifts back (docs/UI.md section 8.7).
type Imported struct {
	Added  []RuleAnswer `json:"added"`
	Edited []EditedRule `json:"edited"`
	Lifted []LiftedRule `json:"lifted"`
	Lifts  int          `json:"lifts"`
}

func importedType() schema.Type {
	p := previewType()
	return schema.Obj("Imported", p.Fields[0], p.Fields[1], p.Fields[2], schema.F("lifts", schema.Int()))
}

// previewRequest is a preview's body, the file alone.
type previewRequest struct {
	File string `json:"file"`
}

// readImport reads an import's body, or a preview's when preview is set, and its file, and answers the
// request when either is refused.
func readImport(w http.ResponseWriter, r *http.Request, scope string, preview bool) (ImportRequest, []fileRule, bool) {
	var body ImportRequest
	d := json.NewDecoder(io.LimitReader(r.Body, maxImportBody))
	d.DisallowUnknownFields()
	var err error
	if preview {
		var p previewRequest
		err = d.Decode(&p)
		body.File = p.File
	} else {
		err = d.Decode(&body)
	}
	if err != nil {
		requestInfo(r.Context()).err = err
		writeFailure(w, r, clientFault(http.StatusBadRequest, "malformed_body", "the request body is not the JSON this request takes"))
		return ImportRequest{}, nil, false
	}
	file, problem := parseFile(body.File)
	if problem != nil {
		a := ProblemAnswer{Kind: problem.kind}
		if problem.line > 0 {
			line := problem.line
			a.Line = &line
		}
		writeRefusal(w, r, "file_refused", "the file is not a policy file", []ProblemAnswer{a})
		return ImportRequest{}, nil, false
	}
	if problems := checkFile(scope, file); len(problems) > 0 {
		writeRefusal(w, r, "file_refused", "the file's rules fail the policy's checks", problems)
		return ImportRequest{}, nil, false
	}
	return body, file, true
}

// preview computes what making a scope's stored rules equal to file would change. For an account's
// scope each lift carries what it alone releases in the account, and the whole import's release is
// counted (docs/UI.md section 8.7).
func (s *Server) preview(ctx context.Context, q registry.Queries, account string, isBase bool, stored []registry.StoredRule, file []fileRule) (Preview, rules.Diff, error) {
	list := make([]rules.Rule, len(file))
	for i, f := range file {
		list[i] = f.rule
	}
	d := rules.Compare(toRules(stored), list)
	p := Preview{Added: []RuleAnswer{}, Edited: []EditedRule{}, Lifted: []LiftedRule{}, Unchanged: d.Unchanged, Lifts: d.Lifts(), Accounts: []string{}, ComputedAgainst: computedAgainst(stored)}
	var composed []registry.StoredRule
	var senders rules.Senders
	if !isBase {
		var err error
		if composed, err = registry.ComposedRules(ctx, q.Rules, account); err != nil {
			return Preview{}, d, err
		}
		if senders, _, err = registry.SenderClasses(ctx, q, account, s.opts.Lookups); err != nil {
			return Preview{}, d, err
		}
	}
	released := func(id string, after []string) *registry.Counts {
		if isBase {
			return nil
		}
		c := registry.RuleRelease(composed, senders, false, id, after)
		return &c
	}
	for _, r := range d.Added {
		p.Added = append(p.Added, RuleAnswer{Scope: scopeOf(isBase), RuleID: r.ID, Suffixes: r.Suffixes})
	}
	for _, e := range d.Edited {
		edited := EditedRule{RuleID: e.ID, Before: e.Before, After: e.After, Added: nonNil(e.Added), Removed: []RemovedSuffix{}}
		for _, removed := range e.Removed {
			rest := slices.DeleteFunc(slices.Clone(e.Before), func(x string) bool { return x == removed })
			edited.Removed = append(edited.Removed, RemovedSuffix{Suffix: removed, Released: released(e.ID, rest)})
		}
		p.Edited = append(p.Edited, edited)
	}
	for _, r := range d.Lifted {
		p.Lifted = append(p.Lifted, LiftedRule{RuleID: r.ID, Suffixes: r.Suffixes, Released: released(r.ID, nil)})
	}
	if !isBase {
		var after []string
		for _, r := range composed {
			if r.Base {
				after = append(after, r.Suffixes...)
			}
		}
		for _, r := range list {
			after = append(after, r.Suffixes...)
		}
		c := senders.Released(rules.Suffixes(registry.Composed(composed)), after)
		p.Released = &registry.Counts{Senders: c.Senders, Messages: c.Messages}
	}
	return p, d, nil
}

// postAccountPreview is POST /api/{account}/policy/import/preview.
func (s *Server) postAccountPreview(w http.ResponseWriter, r *http.Request) {
	s.writePreview(w, r, r.PathValue("account"), false)
}

// postBasePreview is POST /api/setup/policy/import/preview.
func (s *Server) postBasePreview(w http.ResponseWriter, r *http.Request) {
	s.writePreview(w, r, "", true)
}

func (s *Server) writePreview(w http.ResponseWriter, r *http.Request, account string, isBase bool) {
	_, file, ok := readImport(w, r, scopeOf(isBase), true)
	if !ok {
		return
	}
	var out Preview
	err := s.readScope(r.Context(), account, isBase, func(q registry.Queries, stored []registry.StoredRule) error {
		var err error
		out, _, err = s.preview(r.Context(), q, account, isBase, stored, file)
		return err
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	if isBase {
		listed, err := s.listAccountIDs(r.Context())
		if err != nil {
			s.databaseFailure(w, r, err)
			return
		}
		out.Accounts = listed
	}
	writeJSON(w, r, out)
}

// errStalePreview is an import whose scope's rules changed since its preview was computed.
var errStalePreview = errors.New("the scope's rules changed since the preview")

// postAccountImport is POST /api/{account}/policy/import.
func (s *Server) postAccountImport(w http.ResponseWriter, r *http.Request) {
	s.writeImport(w, r, r.PathValue("account"), false)
}

// postBaseImport is POST /api/setup/policy/import.
func (s *Server) postBaseImport(w http.ResponseWriter, r *http.Request) {
	s.writeImport(w, r, "", true)
}

// writeImport applies a file as one transaction, each change with its history row, so an import that
// fails leaves neither rules nor history (ADR-0110, ADR-0102). The scope's rules read in the
// transaction must be the ones the preview was computed against, and an import that lifts anything
// must carry lift {k} for the count it lifts.
func (s *Server) writeImport(w http.ResponseWriter, r *http.Request, account string, isBase bool) {
	body, file, ok := readImport(w, r, scopeOf(isBase), false)
	if !ok {
		return
	}
	if body.ComputedAgainst == nil {
		writeFailure(w, r, clientFault(http.StatusBadRequest, "malformed_body", "an import carries the stored rules its preview was computed against"))
		return
	}
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var out Imported
	err := s.inScope(ctx, account, isBase, func(sw scopeWrites) error {
		stored, err := sw.stored(ctx)
		if err != nil {
			return err
		}
		if computedAgainst(stored) != *body.ComputedAgainst {
			return errStalePreview
		}
		list := make([]rules.Rule, len(file))
		for i, f := range file {
			list[i] = f.rule
		}
		d := rules.Compare(toRules(stored), list)
		if k := d.Lifts(); k > 0 && !confirms(body.Confirmation, rules.Confirmation(k)) {
			return errConfirmationRequired
		}
		out = Imported{Added: []RuleAnswer{}, Edited: []EditedRule{}, Lifted: []LiftedRule{}, Lifts: d.Lifts()}
		written := 0
		step := func() error {
			written++
			if written == 1 {
				return s.fault("after an import's first change")
			}
			return nil
		}
		for _, rule := range d.Added {
			if err := sw.add(ctx, rule, actor); err != nil {
				return err
			}
			if err := sw.record(ctx, actionAdded, rule.ID, nil, rule.Suffixes, actor); err != nil {
				return err
			}
			out.Added = append(out.Added, RuleAnswer{Scope: scopeOf(isBase), RuleID: rule.ID, Suffixes: rule.Suffixes})
			if err := step(); err != nil {
				return err
			}
		}
		for _, e := range d.Edited {
			n, err := sw.edit(ctx, e.ID, e.Before, e.After)
			if err != nil {
				return err
			}
			if n == 0 {
				return errStalePreview
			}
			if err := sw.record(ctx, actionEdited, e.ID, e.Before, e.After, actor); err != nil {
				return err
			}
			edited := EditedRule{RuleID: e.ID, Before: e.Before, After: e.After, Added: nonNil(e.Added), Removed: []RemovedSuffix{}}
			for _, removed := range e.Removed {
				edited.Removed = append(edited.Removed, RemovedSuffix{Suffix: removed})
			}
			out.Edited = append(out.Edited, edited)
			if err := step(); err != nil {
				return err
			}
		}
		for _, rule := range d.Lifted {
			n, err := sw.lift(ctx, rule.ID, rule.Suffixes)
			if err != nil {
				return err
			}
			if n == 0 {
				return errStalePreview
			}
			if err := sw.record(ctx, actionLifted, rule.ID, rule.Suffixes, nil, actor); err != nil {
				return err
			}
			out.Lifted = append(out.Lifted, LiftedRule{RuleID: rule.ID, Suffixes: rule.Suffixes})
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errStalePreview):
		writeFailure(w, r, clientFault(http.StatusConflict, "stale_preview", "The policy changed since this preview. Preview again."))
	case errors.Is(err, errConfirmationRequired):
		writeFailure(w, r, clientFault(http.StatusBadRequest, "confirmation_required", "an import that lifts restrictions needs lift {k} for the count it lifts"))
	case err != nil:
		if key, ok := violated(err, uniqueViolation); ok && key == ruleKey {
			writeFailure(w, r, clientFault(http.StatusConflict, "stale_preview", "The policy changed since this preview. Preview again."))
			return
		}
		s.databaseFailure(w, r, err)
	default:
		writeJSON(w, r, out)
	}
}

// readScope reads a scope's stored rules in a read of its own, the account's transaction or a
// base-policy transaction, with the account's queries for an account's scope.
func (s *Server) readScope(ctx context.Context, account string, isBase bool, fn func(q registry.Queries, stored []registry.StoredRule) error) error {
	if isBase {
		return s.inScope(ctx, account, true, func(sw scopeWrites) error {
			stored, err := sw.stored(ctx)
			if err != nil {
				return err
			}
			return fn(registry.Queries{}, stored)
		})
	}
	return s.inAccount(ctx, account, func(q registry.Queries) error {
		all, err := registry.ComposedRules(ctx, q.Rules, account)
		if err != nil {
			return err
		}
		return fn(q, slices.DeleteFunc(all, func(r registry.StoredRule) bool { return r.Base }))
	})
}

// getAccountExport is GET /api/{account}/policy/export.
func (s *Server) getAccountExport(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	s.writeExport(w, r, account, false, "policy-"+account+".yaml")
}

// getBaseExport is GET /api/setup/policy/export.
func (s *Server) getBaseExport(w http.ResponseWriter, r *http.Request) {
	s.writeExport(w, r, "", true, "policy-base.yaml")
}

// writeExport downloads a scope's rules as a policy file. It changes nothing.
func (s *Server) writeExport(w http.ResponseWriter, r *http.Request, account string, isBase bool, name string) {
	var file []byte
	err := s.readScope(r.Context(), account, isBase, func(_ registry.Queries, stored []registry.StoredRule) error {
		var err error
		file, err = exportFile(stored)
		return err
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", policyFileType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(file); err != nil {
		requestInfo(r.Context()).err = err
	}
}

// policyFileType is the media type an export is downloaded as.
const policyFileType = "application/yaml"
