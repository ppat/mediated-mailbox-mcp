// An operator command is a package main, and not a deployable's composition root.
package main

import "os"

func main() {
	_ = os.Getenv("HOME") // want "reads the environment with os.Getenv"
}
