// Package browser holds the browser bundle the UI serves, embedded from dist, where the bundle's build
// writes it (scripts/build.ts). It is the only Go in the browser half, and it sits here because an
// embed pattern cannot reach a parent directory, so the package that embeds the bundle sits beside
// it, and any composition root can import it.
//
// The pattern carries the all: prefix because a fresh clone holds only the checked-in placeholder in
// dist, and without the prefix a directory holding only a dot file fails to compile. The image build
// refuses a bundle that is only the placeholder (ui/Dockerfile).
package browser

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Bundle returns the built bundle, rooted at dist.
func Bundle() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
