// Package pgx stands for the database driver's transaction type, as the txhelper cases need it.
package pgx

// Tx is a transaction.
type Tx interface{ Commit() error }
