// Package crash is the crash harness (ADR-0045). It generates operation sequences with a crash step,
// reduces a failing sequence against an in-memory model of the machinery, and replays sequences
// against PostgreSQL (ADR-0069).
//
// A target is the machinery under test, described by a Target. The harness adds the crash operation
// to the target's own and runs every sequence against a world the target builds. After each crash it
// runs the target's real recovery path and checks persistence, that everything reported durable
// before the crash is intact. At the end of every sequence it checks forward progress, that the
// machinery completes its work and leaves nothing stuck. The crash model is deliberately coarse. A
// crash stops the process between the target's own steps, or, where the target offers a point inside
// a step, at that point.
//
// Sequences are drawn two ways in every run, gating and scheduled alike. rapid's own draw picks each
// next operation by t.Repeat, and the operation sampler of testsupport/property picks it under a set
// of weights drawn fresh for each sequence. Each draw runs through property.Check, so a failing
// sequence is reduced against the model, kept in the failing-case store with the arguments the world
// was built from, and replayed against PostgreSQL once reduced. A fixed number of sequences drawn
// from fixed seeds also replays against PostgreSQL, so the store the model stands for meets the same
// checks. Nothing is reduced against the database, which is 34 to 40 times slower than the model.
//
// It imports rapid, so only crash-sequence test files may import it.
package crash

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// Crash is the name of the crash operation the harness adds to every target's operations.
const Crash = "crash"

// Op is one operation of a sequence, named, with the argument drawn for it.
type Op struct {
	Name string
	Arg  int
}

// Case is one generated case, the arguments the world is built from and the operation sequence run
// against it. It holds no operation weights, so a kept case replays its sequence and nothing else,
// as ADR-0069's rule for a stored case requires.
type Case[S any] struct {
	Setup S
	Ops   []Op
}

// Operation is one of the machinery's operations.
type Operation[W any] struct {
	// Arg draws the operation's argument, and nil draws none.
	Arg *rapid.Generator[int]
	// Apply applies the operation to the world.
	Apply func(t rapid.TB, w W, arg int)
}

// Target is the machinery under test. S is the plain struct of arguments a world is built from,
// which the failing-case store keeps, and W is a world, the machinery with its store and whatever it
// talks to.
type Target[S, W any] struct {
	// Setup draws the arguments a world is built from.
	Setup func(t *rapid.T) S
	// Model builds a world whose store is the in-memory model, for the sequences the search draws
	// and reduces.
	Model func(t rapid.TB, s S) W
	// Real builds a world whose store is PostgreSQL, for the sequences replayed against it.
	Real func(t testing.TB, s S) W
	// Operations are the machinery's own operations, by name. None may be named Crash.
	Operations map[string]Operation[W]
	// CrashAt draws where a crash stops the process, and nil draws nothing. The target gives each
	// value its meaning.
	CrashAt *rapid.Generator[int]
	// Crash stops the world's process at the point at, losing everything the process held and
	// nothing the store made durable.
	Crash func(t rapid.TB, w W, at int)
	// Recover runs the machinery's real recovery path in a new process.
	Recover func(t rapid.TB, w W)
	// Persistence checks, after each recovery, that everything the world reported durable before the
	// crash is intact.
	Persistence func(t rapid.TB, w W)
	// Progress drives the machinery until its work is done and checks that it completed with nothing
	// stuck, at the end of every sequence.
	Progress func(t rapid.TB, w W)
}

// Config sets how a Check runs.
type Config struct {
	// Replays is how many sequences drawn from fixed seeds are also replayed against PostgreSQL, from
	// each draw.
	Replays int
	// MaxOps bounds the operations the sampler draws for one sequence. rapid's own draw runs as many
	// as t.Repeat does.
	MaxOps int
}

// Draws returns the two ways a sequence is drawn, keyed by name, for a Check and for the generator
// report run beside it.
func Draws[S, W any](target Target[S, W], cfg Config) map[string]func(*rapid.T) Case[S] {
	names := slices.Sorted(maps.Keys(target.Operations))
	names = append(names, Crash)
	arg := func(t *rapid.T, name string) int {
		switch {
		case name == Crash && target.CrashAt != nil:
			return target.CrashAt.Draw(t, "crash point")
		case name != Crash && target.Operations[name].Arg != nil:
			return target.Operations[name].Arg.Draw(t, name+" argument")
		default:
			return 0
		}
	}
	return map[string]func(*rapid.T) Case[S]{
		"rapid-draw": func(t *rapid.T) Case[S] {
			c := Case[S]{Setup: target.Setup(t)}
			actions := map[string]func(*rapid.T){}
			for _, name := range names {
				actions[name] = func(t *rapid.T) { c.Ops = append(c.Ops, Op{Name: name, Arg: arg(t, name)}) }
			}
			t.Repeat(actions)
			return c
		},
		"sampled-mix": func(t *rapid.T) Case[S] {
			c := Case[S]{Setup: target.Setup(t)}
			sampler := property.NewSampler(t, names)
			op := rapid.Custom(func(t *rapid.T) Op {
				name := sampler.Draw(t, "operation")
				return Op{Name: name, Arg: arg(t, name)}
			})
			c.Ops = rapid.SliceOfN(op, 0, cfg.MaxOps).Draw(t, "operations")
			return c
		},
	}
}

// Check runs the target's crash sequences, each draw as a subtest. The model search runs through
// property.Check, and when it fails, its reduced sequence is replayed against PostgreSQL from a
// cleanup, because rapid ends a failing run before anything placed after it runs. Once the search
// passes, cfg.Replays sequences of each draw, from fixed seeds, run against PostgreSQL, and one that
// fails there is run against the model too, so the failure says which of the two holds the fault.
func Check[S, W any](t *testing.T, target Target[S, W], cfg Config) {
	t.Helper()
	if _, clash := target.Operations[Crash]; clash {
		t.Fatalf("the target has an operation named %q, the harness's own", Crash)
	}
	for name, draw := range sortedDraws(Draws(target, cfg)) {
		t.Run(name, func(t *testing.T) {
			var (
				last Case[S]
				// drew is set once the search draws a case, and searched once the search passed, so
				// the cleanup diagnoses a reduced sequence only when the search against the model is
				// what failed, never when a replay did.
				drew, searched bool
			)
			t.Cleanup(func() {
				if !drew || searched {
					return
				}
				w := target.Real(t, last.Setup)
				failed, report := probe(t.Name(), func(tb rapid.TB) { Run(tb, target, w, last) })
				if failed {
					t.Logf("the reduced sequence fails against the model, and against PostgreSQL too: %s\n%s", Describe(last), report)
					return
				}
				t.Errorf("the reduced sequence fails against the model and not against PostgreSQL, so the model differs from the store it stands for: %s", Describe(last))
			})
			property.Check(t, func(rt *rapid.T) Case[S] {
				c := draw(rt)
				last, drew = c, true
				return c
			}, func(tb rapid.TB, c Case[S]) {
				Run(tb, target, target.Model(tb, c.Setup), c)
			})
			searched = true
			gen := rapid.Custom(draw)
			for seed := range cfg.Replays {
				c := gen.Example(seed)
				w := target.Real(t, c.Setup)
				failed, report := probe(t.Name(), func(tb rapid.TB) { Run(tb, target, w, c) })
				if !failed {
					continue
				}
				m := target.Model(t, c.Setup)
				if modelFailed, _ := probe(t.Name(), func(tb rapid.TB) { Run(tb, target, m, c) }); modelFailed {
					t.Errorf("the sequence from seed %d fails against PostgreSQL and against the model: %s\n%s", seed, Describe(c), report)
					continue
				}
				t.Errorf("the sequence from seed %d fails against PostgreSQL and passes against the model, so PostgreSQL has a fault the model lacks: %s\n%s", seed, Describe(c), report)
			}
		})
	}
}

// sortedDraws iterates the draws in the order of their names.
func sortedDraws[S any](draws map[string]func(*rapid.T) Case[S]) func(func(string, func(*rapid.T) Case[S]) bool) {
	return func(yield func(string, func(*rapid.T) Case[S]) bool) {
		for _, name := range slices.Sorted(maps.Keys(draws)) {
			if !yield(name, draws[name]) {
				return
			}
		}
	}
}

// Run runs the case's sequence against the world. After each crash it runs recovery and checks
// persistence, and at the end it checks forward progress.
func Run[S, W any](t rapid.TB, target Target[S, W], w W, c Case[S]) {
	t.Helper()
	for i, op := range c.Ops {
		if op.Name == Crash {
			target.Crash(t, w, op.Arg)
			target.Recover(t, w)
			target.Persistence(t, w)
			if t.Failed() {
				t.Fatalf("persistence failed after operation %d of %s", i+1, Describe(c))
			}
			continue
		}
		o, ok := target.Operations[op.Name]
		if !ok {
			t.Fatalf("operation %d is %q, which the target does not have", i+1, op.Name)
		}
		o.Apply(t, w, op.Arg)
	}
	target.Progress(t, w)
}

// Describe describes a case's operation sequence, never the weights it was drawn under, which
// reduction flattens and which would contradict the sequence, as ADR-0069's rule for a failure
// report requires.
func Describe[S any](c Case[S]) string {
	ops := make([]string, len(c.Ops))
	for i, op := range c.Ops {
		ops[i] = fmt.Sprintf("%s(%d)", op.Name, op.Arg)
	}
	return fmt.Sprintf("setup %+v, operations [%s]", c.Setup, strings.Join(ops, " "))
}

// probe runs f against a recorder standing in for a test, and reports whether f failed and what it
// logged. A fatal call ends f's goroutine, as it does under testing and rapid.
func probe(name string, f func(rapid.TB)) (bool, string) {
	r := &recorder{name: name}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if v := recover(); v != nil {
				r.failed = true
				r.log = append(r.log, fmt.Sprint("panic: ", v))
			}
		}()
		f(r)
	}()
	<-done
	return r.failed, strings.Join(r.log, "\n")
}

type recorder struct {
	name   string
	failed bool
	log    []string
}

func (r *recorder) Helper()                       {}
func (r *recorder) Name() string                  { return r.name }
func (r *recorder) Logf(format string, a ...any)  { r.log = append(r.log, fmt.Sprintf(format, a...)) }
func (r *recorder) Log(a ...any)                  { r.log = append(r.log, fmt.Sprint(a...)) }
func (r *recorder) Skipf(format string, a ...any) { r.Logf(format, a...); runtime.Goexit() }
func (r *recorder) Skip(a ...any)                 { r.Log(a...); runtime.Goexit() }
func (r *recorder) SkipNow()                      { runtime.Goexit() }
func (r *recorder) Errorf(format string, a ...any) {
	r.failed = true
	r.Logf(format, a...)
}
func (r *recorder) Error(a ...any) { r.failed = true; r.Log(a...) }
func (r *recorder) Fatalf(format string, a ...any) {
	r.failed = true
	r.Logf(format, a...)
	runtime.Goexit()
}
func (r *recorder) Fatal(a ...any) { r.failed = true; r.Log(a...); runtime.Goexit() }
func (r *recorder) FailNow()       { r.failed = true; runtime.Goexit() }
func (r *recorder) Fail()          { r.failed = true }
func (r *recorder) Failed() bool   { return r.failed }
