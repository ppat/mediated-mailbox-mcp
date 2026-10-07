// Package core is the log section of a deployable's configuration and its pure validation, which
// checks the merged value before anything else starts (ADR-0078, ADR-0119). The section names the
// level a deployable logs at, one of four words, so no spelling a log library would also accept, such
// as an upper-case name or a level with an offset, passes for one of them.
package core

import (
	"errors"
	"strconv"
)

// Config is the log section.
type Config struct {
	// Level is the least severe level written, debug, info, warn or error.
	Level string `yaml:"level"`
}

// Level is a level the section names, from the most detailed to the most severe.
type Level int

// The levels, in order of severity.
const (
	Debug Level = iota
	Info
	Warn
	Error
)

// Default is the section's default, which logs at info.
func Default() Config {
	return Config{Level: "info"}
}

// Parse returns the level c names, and refuses any other text, a different letter case included.
func Parse(c Config) (Level, error) {
	switch c.Level {
	case "debug":
		return Debug, nil
	case "info":
		return Info, nil
	case "warn":
		return Warn, nil
	case "error":
		return Error, nil
	}
	return 0, errors.New("log.level " + strconv.Quote(c.Level) + " is not debug, info, warn or error")
}
