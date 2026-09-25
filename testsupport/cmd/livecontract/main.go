// Command livecontract runs one provider's contract suite against the real provider, the only way a
// live test runs (ADR-0043). Its live test skips unless this command started it, so no other test
// invocation reaches the provider. It passes the caller's environment, the provider's credential
// variables among it, through to the test, and prints none of it. Run it from the repository root.
//
//	go tool livecontract gmail
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "livecontract:", err)
	}
	os.Exit(code)
}

func run(args []string) (int, error) {
	if len(args) != 1 {
		return 2, errors.New("usage: go tool livecontract <provider>")
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return 2, errors.New("run from the repository root, where go.mod is")
	}
	inv, err := livecontract.Invocation(args[0], os.Environ())
	if err != nil {
		return 2, err
	}
	fmt.Fprintln(os.Stderr, "livecontract: go", strings.Join(inv.Args, " "))
	// The test runs in this process's group and this process waits for it. An interrupted or killed
	// run stops without its current case's cleanup, so the messages that case added stay in the test
	// account with their labels, the accepted limit of a run cut short (ADR-0043).
	cmd := exec.Command("go", inv.Args...)
	cmd.Env = inv.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}
