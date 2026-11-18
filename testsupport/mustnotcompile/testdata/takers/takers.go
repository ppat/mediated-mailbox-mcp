// Package takers is CheckTakers' fixture. Held is the type whose takers are checked.
package takers

// Held is the type the check follows.
type Held struct{ n int }

// Self takes Held as its receiver.
func (h Held) Self() int { return h.n }

// Gives takes Held as a parameter.
func Gives(h Held) int { return h.n }

// Other takes something else.
func Other(n int) int { return n }

// Makes returns Held and takes nothing carrying it.
func Makes() Held { return Held{} }

// Wrapper carries Held in a field, so its methods take Held through their receivers.
type Wrapper struct{ h Held }

// ByValue takes Wrapper by value.
func (w Wrapper) ByValue() int { return w.h.n }

// ByPointer takes Wrapper by pointer.
func (w *Wrapper) ByPointer() int { return w.h.n }

// Maker returns Held from its method and holds none.
type Maker struct{}

// Make returns Held and takes nothing carrying it.
func (Maker) Make() Held { return Held{} }

// heldAlias is another name for Held.
type heldAlias = Held

// ViaAlias takes Held under its alias.
func ViaAlias(h heldAlias) int { return h.n }

// Called takes a function returning Held, which the check does not follow.
func Called(f func() Held) int { return f().n }

// kept holds Held.
var kept Held

var _ = kept
