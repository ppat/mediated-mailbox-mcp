package policyload_test

import (
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
)

// lookups stand in for the classifier's domain functions. Every name the tests classify is a
// lowercase ASCII name with a registrable domain, so each function returns the name it is given.
var lookups = classify.Lookups{
	ToUnicode:   func(domain string) (string, error) { return strings.ToLower(domain), nil },
	ToASCII:     func(domain string) (string, error) { return domain, nil },
	Registrable: func(domain string) (string, error) { return domain, nil },
}
