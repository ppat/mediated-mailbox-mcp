package process

// Option is what a test or a harness hands RunContext to substitute a shared thing it made, such as
// connections over the provider fake. The one kind there is fills an owner's slot before anything
// asks, so what a test runs is what the program runs, apart from what it handed in. An owner exports
// one function returning it, and Run takes none, so a main.go cannot substitute (ADR-0142).
type Option struct {
	fill func(p *proc)
}

// Substitute returns the option that fills the slot with v before anything asks, so every part that
// asks for it gets v and nothing is built. The process does not close v, since the test that made it
// does.
func (s *OnePerProcess[T]) Substitute(v T) Option {
	return Option{fill: func(p *proc) {
		p.slotsMu.Lock()
		defer p.slotsMu.Unlock()
		p.slots[s] = &slot{built: true, value: v}
	}}
}

// Revision returns the revision the configuration library computed for the top-level key, read from
// the process p belongs to, and "" for a key the configuration does not hold or for capabilities no
// process handed out. A shared thing whose settings carry a revision, such as the scanner, reads it
// here, never from whoever asked (ADR-0078, ADR-0142).
func Revision(p Capabilities, key string) string {
	c, ok := p.(*capabilities)
	if !ok {
		return ""
	}
	return c.proc.revisions[key]
}
