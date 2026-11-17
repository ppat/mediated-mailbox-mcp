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
	"io"
	"log"
	"log/slog"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
)

// New returns a logger writing one JSON object per record to w, at the level level holds.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// ParseLevel returns the level name names, one of debug, info, warn and error, spelled exactly so.
// The names and the refusal of any other text are config.Process's check, which refuses the
// spellings slog's own parser accepts, such as INFO, Warn and info+2, so a level is refused in one
// place (ADR-0078). ParseLevel maps a name the check admits to its level.
func ParseLevel(name string) (slog.Level, error) {
	if err := (config.Process{LogLevel: name}).Check(); err != nil {
		return 0, err
	}
	switch name {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	}
	return slog.LevelError, nil
}

// ServerErrorLog returns the error log an http.Server is given, writing through logger's handler at
// warn. The server's own errors are a connection it could not serve, such as a failed TLS
// handshake, which an operator may act on, so they sit at warn (ADR-0122).
func ServerErrorLog(logger *slog.Logger) *log.Logger {
	return slog.NewLogLogger(logger.Handler(), slog.LevelWarn)
}
