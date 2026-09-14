// The browser app's entry point, the one module scripts/build.ts bundles. Everything the bundle holds
// is reached from here, so a module nothing here imports never reaches dist or the Go binary.
//
// It holds no code until the first screen lands, so .oxlintrc.json switches unicorn/no-empty-file off for
// this file alone.
