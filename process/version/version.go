// Package version holds the release version every binary of the project reports through its
// --version flag (ADR-0078). process/README.md argues why the process family holds it.
//
// The release cut writes it. release-please's generic updater, which release-please-config.json
// names in its extra-files, replaces the version on the line marked below in the same pull request
// that writes the chart's version and appVersion, so a release's binaries, images and chart name
// one version (ADR-0049). A binary built between releases, a latest image among them, reports the
// version of the newest release at or before the commit it was built from (ADR-0052).
package version

// Number is the release version, with no leading v.
const Number = "0.0.1" // x-release-please-version
