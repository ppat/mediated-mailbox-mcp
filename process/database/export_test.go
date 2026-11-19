package database

// The names the package's external tests read, which the package does not export.
var (
	PoolConfig = poolConfig
	OpenPool   = openPool
)

type Tracer = statementTracer
