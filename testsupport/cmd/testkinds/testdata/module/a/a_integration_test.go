//go:build integration

package a

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(m.Run()) }

func TestIntegration(t *testing.T) {}
