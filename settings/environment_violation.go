//go:build banproof

package settings

import (
	"os"
	"os/exec"
	"syscall"
)

// This file reads the environment from a shared library on purpose, once with each function the
// rule refuses, and banproof requires the wants below from go vet. A setting read here would skip
// the layers and every refusal they make.
func fromEnvironment() []string {
	threshold := os.Getenv(Prefix + "SCANNER__THRESHOLD")   // want vetcheck "reads the environment with os.Getenv outside a deployable's composition root"
	_, found := os.LookupEnv(Prefix + "SCANNER__THRESHOLD") // want vetcheck "reads the environment with os.LookupEnv outside a deployable's composition root"
	_ = found
	path := os.ExpandEnv("$HOME/config.yaml")   // want vetcheck "reads the environment with os.ExpandEnv outside a deployable's composition root"
	home, _ := syscall.Getenv("HOME")           // want vetcheck "reads the environment with syscall.Getenv outside a deployable's composition root"
	all := syscall.Environ()                    // want vetcheck "reads the environment with syscall.Environ outside a deployable's composition root"
	inherited := exec.Command("true").Environ() // want vetcheck "reads the environment with \\(\\*exec.Cmd\\).Environ outside a deployable's composition root"
	all = append(all, inherited...)
	return append(append(all, os.Environ()...), threshold, path, home) // want vetcheck "reads the environment with os.Environ outside a deployable's composition root"
}
