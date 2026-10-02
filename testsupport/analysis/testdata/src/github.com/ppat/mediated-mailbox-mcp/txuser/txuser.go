// Package txuser holds the txhelper cases. Each want names a finding the analyser must report, and a
// line without one must report nothing.
package txuser

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential"
	"github.com/ppat/mediated-mailbox-mcp/db/lookalike"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients"
	"github.com/ppat/mediated-mailbox-mcp/db/oauthclients/secret"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/other/queries"
)

type pool struct{}

// Inside builds every subsection's queries from the transaction the literal is handed, in the
// literal, in a closure inside it and through parentheses.
func Inside(ctx context.Context, p pool) error {
	return tx.Run(ctx, p, "a", func(t pgx.Tx) error {
		q := credential.New(t)
		_ = q.Sealed(ctx)
		done := make(chan struct{})
		go func() {
			_ = accounts.New(t).Other(ctx)
			close(done)
		}()
		<-done
		return credential.New((t)).Sealed(ctx)
	})
}

// WrongHandle builds queries inside the literal on the pool.
func WrongHandle(ctx context.Context, p pool) error {
	return tx.Run(ctx, p, "a", func(t pgx.Tx) error {
		_ = t
		return credential.New(p).Sealed(ctx) // want `calls credential.New on a handle other than the transaction tx.Run hands the enclosing literal`
	})
}

// Outside builds queries outside every literal passed to tx.Run.
func Outside(ctx context.Context, p pool) error {
	if err := credential.New(p).Sealed(ctx); err != nil { // want `calls credential.New outside a function literal passed to tx.Run`
		return err
	}
	return tx.Run(ctx, p, "a", func(pgx.Tx) error { return nil })
}

// Exceptions runs the three exempt statements on any handle, and nothing else.
func Exceptions(ctx context.Context, p pool) error {
	if err := accounts.New(p).Accounts(ctx); err != nil {
		return err
	}
	if err := oauthclients.New(p).OAuthClients(ctx); err != nil {
		return err
	}
	if err := secret.New(p).ReplaceSealedClient(ctx); err != nil {
		return err
	}
	if err := secret.New(p).Other(ctx); err != nil { // want `calls secret.New outside a function literal passed to tx.Run`
		return err
	}
	if err := accounts.New(p).Other(ctx); err != nil { // want `calls accounts.New outside a function literal passed to tx.Run`
		return err
	}
	listing := accounts.New(p) // want `calls accounts.New outside a function literal passed to tx.Run`
	return listing.Accounts(ctx)
}

// Values uses New as a value and rebinds queries with WithTx.
func Values(ctx context.Context, p pool) error {
	build := oauthclients.New // want `uses oauthclients.New as a value`
	_ = build
	return tx.Run(ctx, p, "a", func(t pgx.Tx) error {
		return credential.New(t).WithTx(t).Sealed(ctx) // want `rebinds credential queries with WithTx`
	})
}

func read(pgx.Tx) error { return nil }

// Named passes tx.Run a declared function.
func Named(ctx context.Context, p pool) error {
	return tx.Run(ctx, p, "a", read) // want `passes tx.Run a function that is not a function literal`
}

// Lookalikes calls a New that builds no Queries under db, and one that builds Queries outside db.
func Lookalikes(ctx context.Context, p pool) error {
	_ = lookalike.New(p)
	return queries.New(p).Read(ctx)
}

// Replaced assigns to the transaction the literal is handed, ranges into it and takes its address,
// each of which could put another transaction in its place.
func Replaced(ctx context.Context, p pool, others []pgx.Tx) error {
	return tx.Run(ctx, p, "a", func(t pgx.Tx) error {
		t = others[0]             // want `assigns to the transaction tx.Run hands the literal`
		for _, t = range others { // want `assigns to the transaction tx.Run hands the literal`
		}
		held := &t // want `takes the address of the transaction tx.Run hands the literal`
		_ = held
		other := others[0]
		kept := &other
		_ = kept
		return credential.New(t).Sealed(ctx)
	})
}

// Shadowed declares a variable of the parameter's name, which is another handle.
func Shadowed(ctx context.Context, p pool, other pgx.Tx) error {
	return tx.Run(ctx, p, "a", func(t pgx.Tx) error {
		_ = t
		{
			t := other
			return credential.New(t).Sealed(ctx) // want `calls credential.New on a handle other than the transaction tx.Run hands the enclosing literal`
		}
	})
}

// Nested builds queries inside a nested literal, once from its own transaction and once from the
// enclosing literal's, which set another account.
func Nested(ctx context.Context, p pool) error {
	return tx.Run(ctx, p, "a", func(outer pgx.Tx) error {
		return tx.Run(ctx, p, "b", func(inner pgx.Tx) error {
			if err := credential.New(inner).Sealed(ctx); err != nil {
				return err
			}
			return credential.New(outer).Sealed(ctx) // want `calls credential.New on the transaction of an enclosing literal passed to tx.Run, not the innermost one`
		})
	})
}
