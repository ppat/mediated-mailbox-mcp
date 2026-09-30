// Package target sits in a directory whose name starts with an underscore, which ./... leaves out of
// every lint, so that propose/unlinted_violation.go has a package to import that its import list
// admits.
package target
