package settings

import (
	"os"
	"testing"
)

// A test file may read the environment.
func TestLoad(t *testing.T) {
	_ = os.Getenv("HOME")
}
