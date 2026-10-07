// Package logging builds the one logger a deployable's composition root hands every shell that logs:
// JSON lines written to the writer the root names, standard output in a deployable, at the level its
// configuration's log section sets (ADR-0051, ADR-0119). The section and its pure validation are
// logging/core's. Its case is argued in its README.
package logging

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/ppat/mediated-mailbox-mcp/logging/core"
	"github.com/ppat/mediated-mailbox-mcp/settings"
)

// levelPath is the configuration path of the level, under the section every deployable names log.
const levelPath = "log.level"

// New returns a logger writing JSON lines to w at the level c names. A level it refuses is refused
// with the layer that set it, taken from values, the configuration library's record of every value,
// because the refusal comes before a deployable logs its effective configuration (ADR-0078).
func New(w io.Writer, c core.Config, values []settings.Value) (*slog.Logger, error) {
	level, err := core.Parse(c)
	if err != nil {
		for _, v := range values {
			if v.Path == levelPath {
				return nil, fmt.Errorf("%w, set by the %s", err, v.Source)
			}
		}
		return nil, err
	}
	return build(w, handlerLevel(level)), nil
}

// Initial returns the logger a composition root logs through before its configuration is loaded,
// writing JSON lines to w at the default level, so a start refused before then is still logged in
// the same form.
func Initial(w io.Writer) *slog.Logger {
	return build(w, slog.LevelInfo)
}

func build(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// handlerLevel is the handler's level for each level the section names.
func handlerLevel(l core.Level) slog.Level {
	switch l {
	case core.Debug:
		return slog.LevelDebug
	case core.Info:
		return slog.LevelInfo
	case core.Warn:
		return slog.LevelWarn
	case core.Error:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
