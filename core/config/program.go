package config

// Program is what the process reads of a program's configuration type C (ADR-0142). C holds the
// process part, checks itself, and gives its defaults on a node with a number of processors, so the
// process loads, checks and starts every program the same way.
type Program[C any] interface {
	HasProcess
	Check() error
	Defaults(processors int) C
}

// HasProcess is a configuration that holds the process part. Every program's type does, inline.
type HasProcess interface {
	ProcessPart() Process
}

// HasOperations is a configuration that holds the operations part, which a process that runs until it
// is told to stop serves its operations endpoints from.
type HasOperations interface {
	OperationsPart() Operations
}

// HasServing is a configuration that holds the serving part, which a process whose spec lists
// endpoints serves its application server from.
type HasServing interface {
	ServingPart() Serving
}

// HasDatabase is a configuration that holds the database part, which every pool of the process is
// opened from (ADR-0078).
type HasDatabase interface {
	DatabasePart() Database
}

// HasMigratorDatabase is the migrator's configuration and no other, since only this package can write
// its unexported method and only Migrator does. The pool opened from it is the migrator's, with no
// schema check, so a deployable's configuration cannot ask for one (ADR-0140, ADR-0142).
type HasMigratorDatabase interface {
	HasDatabase
	migrator()
}

// ProcessPart returns the process part. It is promoted to every type that embeds the part.
func (p Process) ProcessPart() Process { return p }

// OperationsPart returns the operations part. It is promoted to every type that embeds the part.
func (o Operations) OperationsPart() Operations { return o }

// ServingPart returns the serving part. It is promoted to every type that embeds the part.
func (s Serving) ServingPart() Serving { return s }

// DatabasePart returns the API server's database part.
func (c API) DatabasePart() Database { return c.Database }

// DatabasePart returns the UI's database part.
func (c UI) DatabasePart() Database { return c.Database }

// DatabasePart returns the worker's one database part, which sizes each job kind's pool alike.
func (c Worker) DatabasePart() Database { return c.Database }

// DatabasePart returns the migrator's database part.
func (c Migrator) DatabasePart() Database { return c.Database }

func (Migrator) migrator() {}

// Defaults returns the API server's defaults on a node with processors processors.
func (API) Defaults(processors int) API { return DefaultAPI(processors) }

// Defaults returns the UI's defaults on a node with processors processors.
func (UI) Defaults(processors int) UI { return DefaultUI(processors) }

// Defaults returns the worker's defaults, which do not depend on the node's processors.
func (Worker) Defaults(int) Worker { return DefaultWorker() }

// Defaults returns the migrator's defaults on a node with processors processors.
func (Migrator) Defaults(processors int) Migrator { return DefaultMigrator(processors) }

// HasAccounts is a configuration that holds the accounts part, which the Session reloads on, and the
// database part it reads the accounts from. The UI's type has no accounts part, so the UI cannot ask
// for the Session (ADR-0090, ADR-0142).
type HasAccounts interface {
	HasDatabase
	AccountsPart() Accounts
}

// HasConnections is a configuration that holds the accounts part and the credential part, which the
// connections open each stored credential with. The UI's type holds the credential part and no
// accounts part, so the UI cannot ask for the connections (ADR-0081, ADR-0142).
type HasConnections interface {
	HasAccounts
	KeysPart() Keys
}

// HasScanner is a configuration that holds the scanner part, which the process's one scanner is built
// from (ADR-0120, ADR-0142).
type HasScanner interface {
	ScannerPart() Scanner
}

// AccountsPart returns the accounts part. It is promoted to every type that embeds the part.
func (a Accounts) AccountsPart() Accounts { return a }

// KeysPart returns the API server's credential part.
func (c API) KeysPart() Keys { return c.Credential }

// ScannerPart returns the API server's scanner part.
func (c API) ScannerPart() Scanner { return c.Scanner }

// ScannerPart returns the worker's one scanner part, which every job kind that masks or scans reads.
func (c Worker) ScannerPart() Scanner { return c.Scanner }
