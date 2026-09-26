// A deployable's composition root, which may read the environment.
package main

import "os"

func main() {
	_ = os.Environ()
	_, _ = os.LookupEnv("HOME")
	_ = os.Getenv("HOME")
	wiring()
}
