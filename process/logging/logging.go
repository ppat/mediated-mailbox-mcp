// Package logging builds the logger every deployable writes its records through, as JSON to the
// writer its main.go gives, standard output, at the level a level variable holds, and reads the
// level's name from the configuration (ADR-0122, ADR-0051). Its case is argued in
// process/README.md.
//
// A deployable's main.go builds the level variable and the logger, makes the logger the process
// default for code the project does not own, and hands both to its entry package. The entry reads
// log_level through ParseLevel once the configuration has loaded and sets the variable, so every
// logger derived from the handed one follows it.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
)

// DefaultLevel is the name of the level a deployable logs at when its configuration names none.
// Routine progress, the UI's line per request among it, is written at info (ADR-0122).
const DefaultLevel = "info"

// New returns a logger writing one JSON object per record to w, at the level level holds.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// ParseLevel returns the level name names, one of debug, info, warn and error, spelled exactly so.
// Any other text is refused, the spellings slog's own parser accepts, such as INFO, Warn and
// info+2, included, as the configuration library refuses every value outside its designed range
// (ADR-0078).
func ParseLevel(name string) (slog.Level, error) {
	switch name {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log_level %q is not one of debug, info, warn and error", name)
	}
}

// ServerErrorLog returns the error log an http.Server is given, writing through logger's handler at
// warn. The server's own errors are a connection it could not serve, such as a failed TLS
// handshake, which an operator may act on, so they sit at warn (ADR-0122).
func ServerErrorLog(logger *slog.Logger) *log.Logger {
	return slog.NewLogLogger(logger.Handler(), slog.LevelWarn)
}
