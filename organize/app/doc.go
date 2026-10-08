// Package app is the entry package and composition root of the Reorg Engine's apply and rollback
// job kind. It sits outside internal, so a composition root other than its own can compose it, and
// every other package of this deployable sits under internal.
package app
