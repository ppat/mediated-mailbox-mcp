// An operator command is a package main, and its main.go is not a deployable's.
package main

import "os"

func main() {
	_ = os.Getenv("HOME") // want "reads the environment with os.Getenv"
}
