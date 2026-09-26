// Package tool stands for the test tooling, which may read the environment.
package tool

import "os"

func Read() string { return os.Getenv("HOME") }
