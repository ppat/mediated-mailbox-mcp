package main

import "os"

// A second file of a deployable's package main is not its main.go.
func wiring() {
	_ = os.Getenv("HOME") // want "reads the environment with os.Getenv"
}
