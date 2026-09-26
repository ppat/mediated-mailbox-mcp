// Package run stands for a deployable's code outside its composition root.
package run

import (
	"net/http"
	"os"
	environment "os"
	"os/exec"
	"syscall"
)

func Run() {
	_, _ = os.LookupEnv("HOME") // want "reads the environment with os.LookupEnv"
	_ = environment.Environ()   // want "reads the environment with os.Environ"
	read := os.Getenv           // want "reads the environment with os.Getenv"
	pass(os.LookupEnv)          // want "reads the environment with os.LookupEnv"
	_ = os.ExpandEnv("$HOME")   // want "reads the environment with os.ExpandEnv"
	_ = os.Expand("$HOME", read)
	_, _ = syscall.Getenv("HOME") // want "reads the environment with syscall.Getenv"
	_ = syscall.Environ()         // want "reads the environment with syscall.Environ"
	cmd := exec.Command("true")
	_ = cmd.Environ()       // want "reads the environment with \\(\\*exec.Cmd\\).Environ"
	all := cmd.Environ      // want "reads the environment with \\(\\*exec.Cmd\\).Environ"
	_ = (*exec.Cmd).Environ // want "reads the environment with \\(\\*exec.Cmd\\).Environ"
	_ = all
	_, _ = os.UserHomeDir()
	_ = os.TempDir()
	_ = http.ProxyFromEnvironment
	_ = read("HOME")
	_ = os.Getpid()
	_ = syscall.Getpid()
}

func pass(func(string) (string, bool)) {}
