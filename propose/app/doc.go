// Package app is the entry package and composition root of the Heuristics Job, which holds nothing to
// run yet. When the job has a run, it holds it here and propose/main.go calls it. It sits outside
// internal, so a composition root other than its own can compose it. Every other package of this
// deployable sits under internal, apart from importtarget, which exists only under the banproof tag.
package app
