// Command mediated-mailbox-cli is the command-line program an operator runs by hand, outside every
// deployment, with a subcommand per task. Its one subcommand is keygen, which writes the key pair
// credentials are sealed to (ADR-0088). It reads no configuration, and its -version flag prints the
// release version and runs nothing (ADR-0078).
//
// A subcommand's error is printed to standard error after the subcommand's name, and the program
// exits 1. A missing or unknown subcommand, or a flag the program does not take, prints the usage to
// standard error and exits 2.
//
//	mediated-mailbox-cli keygen -private-key-file credential.key -public-key-file credential.pub
//	mediated-mailbox-cli -version
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/cli/internal/keygen"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

const usage = `Usage: mediated-mailbox-cli [-version] <subcommand> [flags]

Subcommands:
  keygen  write a new key pair that credentials are sealed to

Flags:
`

func main() {
	if code := run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

// run runs the subcommand args name and returns the program's exit status. It writes what the
// program reports, when there is anything, to stderr in one write, and a run that succeeded exits 1
// when that write fails. A subcommand's flag parser writes its own errors and usage to os.Stderr.
func run(args []string, stdout, stderr io.Writer) int {
	var report strings.Builder
	status := dispatch(args, stdout, &report)
	if report.Len() == 0 {
		return status
	}
	if _, err := io.WriteString(stderr, report.String()); err != nil && status == 0 {
		return 1
	}
	return status
}

// dispatch runs the subcommand args name, writing what the program reports to report, and returns
// the exit status.
func dispatch(args []string, stdout io.Writer, report *strings.Builder) int {
	flags := flag.NewFlagSet("mediated-mailbox-cli", flag.ContinueOnError)
	flags.SetOutput(report)
	flags.Usage = func() {
		report.WriteString(usage)
		flags.PrintDefaults()
	}
	printVersion := flags.Bool("version", false, "print the release version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *printVersion {
		if _, err := fmt.Fprintln(stdout, version.Number); err != nil {
			return 1
		}
		return 0
	}
	if flags.NArg() == 0 {
		report.WriteString("mediated-mailbox-cli: no subcommand named\n")
		flags.Usage()
		return 2
	}
	name, rest := flags.Arg(0), flags.Args()[1:]
	var err error
	switch name {
	case "keygen":
		err = keygen.Run(rest, stdout)
	default:
		fmt.Fprintf(report, "mediated-mailbox-cli: unknown subcommand %q\n", name)
		flags.Usage()
		return 2
	}
	if err != nil {
		fmt.Fprintf(report, "%s: %v\n", name, err)
		return 1
	}
	return 0
}
