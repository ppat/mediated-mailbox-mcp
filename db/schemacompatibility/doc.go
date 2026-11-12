// Package schemacompatibility is the data-access subsection for the compatibility floor and the newest
// migration the chain's runner records, which describe the schema and belong to no account. Every
// process that runs until it is told to stop reads them on each pool it opens and refuses a schema its
// own chain cannot run on (ADR-0140). Its statement is not account-scoped.
package schemacompatibility
