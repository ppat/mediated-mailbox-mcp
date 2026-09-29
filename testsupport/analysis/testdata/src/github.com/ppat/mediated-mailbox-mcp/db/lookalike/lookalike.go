// Package lookalike sits among the subsections, and its New builds no Queries, so it is no
// subsection.
package lookalike

// Queries is not what New returns.
type Queries struct{}

// Reader is what New returns.
type Reader struct{}

// New returns a Reader.
func New(db any) *Reader { return &Reader{} }
