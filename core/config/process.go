package config

import (
	"errors"
	"strconv"
	"strings"
)

// Process is the part of the configuration every process holds, at the top of the tree.
type Process struct {
	// LogLevel is the lowest level the process logs at, debug, info, warn or error (ADR-0122).
	LogLevel string `yaml:"log_level"`
}

// DefaultProcess is the process part's defaults. Routine progress, the UI's line per request among
// it, is written at info (ADR-0122).
func DefaultProcess() Process {
	return Process{LogLevel: "info"}
}

// Check refuses a log_level that names no level. The spellings slog's own parser accepts, such as
// INFO, Warn and info+2, are refused, as every value outside its designed range is (ADR-0078).
func (p Process) Check() error {
	switch p.LogLevel {
	case "debug", "info", "warn", "error":
		return nil
	}
	return errors.New("log_level " + strconv.Quote(p.LogLevel) + " is not one of debug, info, warn and error")
}

// Operations is the part of the configuration a process holds when it runs until it is told to stop,
// so it serves its operations endpoints, at the top of the tree beside Process. A process that runs
// once serves none, since nothing probes or scrapes a process that exits on its own.
type Operations struct {
	// ProbeListen is the address the operations endpoints listen on, over plain HTTP (ADR-0051).
	ProbeListen string `yaml:"probe_listen"`
}

// DefaultOperations is the operations part's defaults.
func DefaultOperations() Operations {
	return Operations{ProbeListen: ":8080"}
}

// Check refuses an empty probe_listen.
func (o Operations) Check() error {
	if strings.TrimSpace(o.ProbeListen) == "" {
		return errors.New("probe_listen is empty")
	}
	return nil
}

// Serving is the part of the configuration a program holds when it serves requests, at the top of
// the tree beside Process.
type Serving struct {
	// Listen is the address the application server listens on.
	Listen string `yaml:"listen"`
	// TLSCert and TLSKey name the mounted files holding the certificate chain and the private key.
	// Both set, the application server serves TLS, and neither set, plain HTTP (ADR-0127).
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
}

// DefaultServing is the serving part's defaults. The key pair has no default, since a default path
// assumes the environment.
func DefaultServing() Serving {
	return Serving{Listen: ":8443"}
}

// ServesTLS reports whether s names a key pair. Check refuses one that names only one of its two
// files, so the certificate alone decides.
func (s Serving) ServesTLS() bool {
	return strings.TrimSpace(s.TLSCert) != ""
}

// Check refuses an empty listen, and one of the two TLS files set without the other, a blank path
// counting as unset (ADR-0127).
func (s Serving) Check() error {
	if strings.TrimSpace(s.Listen) == "" {
		return errors.New("listen is empty")
	}
	if cert, key := strings.TrimSpace(s.TLSCert) != "", strings.TrimSpace(s.TLSKey) != ""; cert != key {
		set, missing := "tls_cert", "tls_key"
		if key {
			set, missing = "tls_key", "tls_cert"
		}
		return errors.New(set + " is set and " + missing + " is not, and the process serves TLS with both or plain HTTP with neither")
	}
	return nil
}
