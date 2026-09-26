package main

import "os"

// A second file of the composition root's package is not the composition root.
func wiring() {
	_ = os.Getenv("HOME") // want "reads the environment with os.Getenv"
}
