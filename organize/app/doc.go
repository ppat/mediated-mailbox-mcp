// Package app is the entry package and composition root of the reorganization workload, which holds
// nothing to run yet. When the workload has a run, it holds it here and organize/main.go calls it. It
// sits outside internal, so a composition root other than its own can compose it, and every other
// package of this deployable sits under internal.
package app
