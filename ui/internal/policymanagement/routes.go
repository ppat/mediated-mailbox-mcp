package policymanagement

import (
	"net/http"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Database is the UI's connection pool as policy management uses it. An account's rules are read and
// written in the transaction helper's transaction set to the account, the base policy's in a
// base-policy transaction (ADR-0047, ADR-0112), and a read that classifies senders runs over a
// repeatable-read snapshot, so one response sees one state of the policy and the index (ADR-0136). The
// accounts listing, which belongs to no account, is read on it directly (ADR-0091).
type Database interface {
	tx.Beginner
	tx.BeginnerWithOptions
	accounts.DBTX
}

// Options are everything the policy routes read. The read API endpoint builds them from the
// configuration, and its tests build them by hand, with a fixed clock, which no configuration carries.
type Options struct {
	Database Database
	// Clock is policy management's time, which the history's range reads.
	Clock func() time.Time
	// Lookups are the functions the sender classifier normalizes domains with, which policy
	// management matches senders against rules with (ADR-0004).
	Lookups classify.Lookups
	// Identity is who a policy write is recorded as made by (ADR-0084).
	Identity Identity
}

// handlers are the policy routes' handlers over what they read.
type handlers struct {
	opts Options
}

// snapshot is the database as a source of repeatable-read, read-only transactions, over which every
// read that classifies a sender runs (ADR-0136).
func (s *handlers) snapshot() tx.Beginner { return tx.Snapshot(s.opts.Database) }

// Routes are the policy routes, each carrying its handler, in the order the read API mounts them: an
// account's policy reads, writes, export and import, then the base policy's.
func Routes(o Options) []server.Route {
	s := &handlers{opts: o}
	return []server.Route{
		{
			Pattern: paths.Accounts + "/{account}/policy/match", Operation: "getPolicyMatch", Scoped: true, Query: []string{"suffix"},
			Summary:  "What each suffix typed in Add a rule matches in the account, and the rules already matching it",
			Response: matchAnswerType(), Handler: s.getMatch,
		},
		{
			Pattern: paths.Accounts + "/{account}/policy/release", Operation: "getPolicyRelease", Scoped: true, Query: []string{"scope", "rule", "keep"},
			Summary:  "What keeping only some of a rule's suffixes, or none, would release in the account",
			Response: registry.CountsType(), Handler: s.getRelease,
		},
		{
			Pattern: paths.Setup + "/policy/match", Operation: "getBaseMatch", Query: []string{"suffix"},
			Summary:  "Whether each suffix typed in Add a base rule is valid or a public suffix, and the base rules already matching it",
			Response: baseMatchAnswerType(), Handler: s.getBaseMatch,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/policy/rules", Operation: "addRule", Scoped: true,
			Summary: "Adds a rule to the account's own rules or the base policy, with its history row",
			Request: new(ruleRequestType(true, "AddRule")), Response: ruleAnswerType(), Refusal: new(RefusalType()), Handler: s.postAccountRule,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/policy/rules/{rule}", Operation: "editRule", Scoped: true,
			Summary: "Sets a rule's suffixes, with its history row",
			Request: new(editRequestType(true, "EditRule")), Response: ruleAnswerType(), Refusal: new(RefusalType()), Handler: s.postAccountEdit,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/policy/rules/{rule}/lift", Operation: "liftRule", Scoped: true,
			Summary: "Lifts a rule, with its history row",
			Request: new(liftRequestType(true, "LiftRule")), Response: ruleAnswerType(), Handler: s.postAccountLift,
		},
		{
			Pattern: paths.Accounts + "/{account}/policy/export", Operation: "exportPolicy", Scoped: true, Download: policyFileType,
			Summary: "Downloads the account's own rules as a policy file", Handler: s.getAccountExport,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/policy/import/preview", Operation: "previewImport", Scoped: true,
			Summary: "Reads and checks a policy file and answers what importing it into the account's own rules would change",
			Request: new(previewRequestType()), Response: previewType(), Refusal: new(RefusalType()), Handler: s.postAccountPreview,
		},
		{
			Method: http.MethodPost, Pattern: paths.Accounts + "/{account}/policy/import", Operation: "importPolicy", Scoped: true,
			Summary: "Makes the account's own rules equal to a policy file, in one transaction",
			Request: new(importRequestType()), Response: importedType(), Refusal: new(RefusalType()), Handler: s.postAccountImport,
		},
		{
			Pattern: paths.Setup + "/policy", Operation: "getBasePolicy", Query: []string{"search"},
			Summary:  "The base rules and every account's identifier, reading no account's state",
			Response: basePolicyType(), Handler: s.getBasePolicy,
		},
		{
			Pattern: paths.Setup + "/policy/history", Operation: "getBaseHistory", Query: []string{"range", "rule"},
			Summary:  "The base policy's changes, newest first",
			Response: baseHistoryType(), Handler: s.getBaseHistory,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/policy/rules", Operation: "addBaseRule",
			Summary: "Adds a base rule, with its history row",
			Request: new(ruleRequestType(false, "AddBaseRule")), Response: ruleAnswerType(), Refusal: new(RefusalType()), Handler: s.postBaseRule,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/policy/rules/{rule}", Operation: "editBaseRule",
			Summary: "Sets a base rule's suffixes, with its history row",
			Request: new(editRequestType(false, "EditBaseRule")), Response: ruleAnswerType(), Refusal: new(RefusalType()), Handler: s.postBaseEdit,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/policy/rules/{rule}/lift", Operation: "liftBaseRule",
			Summary: "Lifts a base rule, with its history row",
			Request: new(liftRequestType(false, "LiftBaseRule")), Response: ruleAnswerType(), Handler: s.postBaseLift,
		},
		{
			Pattern: paths.Setup + "/policy/export", Operation: "exportBasePolicy", Download: policyFileType,
			Summary: "Downloads the base rules as a policy file", Handler: s.getBaseExport,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/policy/import/preview", Operation: "previewBaseImport",
			Summary: "Reads and checks a policy file and answers what importing it into the base policy would change",
			Request: new(previewRequestType()), Response: previewType(), Refusal: new(RefusalType()), Handler: s.postBasePreview,
		},
		{
			Method: http.MethodPost, Pattern: paths.Setup + "/policy/import", Operation: "importBasePolicy",
			Summary: "Makes the base rules equal to a policy file, in one transaction",
			Request: new(importRequestType()), Response: importedType(), Refusal: new(RefusalType()), Handler: s.postBaseImport,
		},
	}
}
