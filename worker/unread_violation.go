//go:build banproof && !integration

// This file is compiled whenever the integration tag is unset, as an image build leaves it, and the
// gating lint and go vet runs set that tag, so neither reads it. banproof's check that every file a
// build that ships compiles is one the lint reads is the one refusing it. It proves the rule with
// the banproof tag, which the check's tagged run sets, because a file without that tag would ship.
package main // want unlinted "its build constraints keep it out of the gating lint and vet runs"
