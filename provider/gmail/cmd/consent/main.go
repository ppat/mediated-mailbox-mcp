// Command consent runs the one-time installed-app consent for one Gmail account and prints the
// account's refresh token (ADR-0083). The grant it asks for is the modify scope and no other. A
// developer runs it by hand, outside every deployable, and puts the token in the GitHub Actions
// secret GMAIL_TEST_REFRESH_TOKEN for the contract suite's test account. A deployable's accounts are
// connected through the UI instead (ADR-0080).
//
// It reads the installed-app client's identifier and secret from the files the -client-id-file and
// -client-secret-file flags name, so neither passes through the command line or the environment.
// The -account flag names the address of the account the grant is meant for. Google is asked to
// offer that account first, and the grant is refused when it belongs to another. The command prints
// the consent page's address to standard error, receives Google's redirect on a loopback address,
// reads the address of the account the grant belongs to and prints it to standard error, and prints
// the refresh token alone to standard output. A browser signed into another account is the likeliest
// way to grant the wrong one, which is why the account is always shown.
//
//	go run ./provider/gmail/cmd/consent -account test@example.com -client-id-file client-id -client-secret-file client-secret
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "consent:", err)
		os.Exit(1)
	}
}

func run() error {
	idFile := flag.String("client-id-file", "", "the file holding the installed-app client's identifier")
	secretFile := flag.String("client-secret-file", "", "the file holding the installed-app client's secret")
	account := flag.String("account", "", "the address of the account the grant is meant for")
	flag.Parse()
	clientID, err := readValue(*idFile)
	if err != nil {
		return err
	}
	clientSecret, err := readValue(*secretFile)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *account == "" {
		return errors.New("-account is required")
	}
	client := &http.Client{Timeout: time.Minute}
	grant, err := gmail.Consent(ctx, client, clientID, clientSecret, *account, os.Stderr)
	if err != nil {
		return err
	}
	granted, err := gmail.Address(ctx, client, grant.AccessToken)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stderr, "The grant belongs to %s.\n", granted); err != nil {
		return err
	}
	if err := gmail.RequireAccount(granted, *account); err != nil {
		return fmt.Errorf("%w, so no refresh token is printed", err)
	}
	_, err = fmt.Println(grant.RefreshToken)
	return err
}

// readValue reads the one value a file holds, without the surrounding whitespace an editor or a
// download leaves.
func readValue(path string) (string, error) {
	if path == "" {
		return "", errors.New("both -client-id-file and -client-secret-file are required")
	}
	b, err := os.ReadFile(path) //nolint:gosec // the path is the file the operator names
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(b))
	if value == "" {
		return "", fmt.Errorf("the file %s is empty", path)
	}
	return value, nil
}
