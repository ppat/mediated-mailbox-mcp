//go:build integration

package dbmetrics_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}
