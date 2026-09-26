package main

import (
	"os"
	"testing"
)

// A test file may read the environment.
func TestMain(m *testing.M) {
	_ = os.Getenv("HOME")
	os.Exit(m.Run())
}
