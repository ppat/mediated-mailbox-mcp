// Command propose is the Heuristics Job, published as mediated-mailbox-propose.
//
// This file sets the process up. The deployable's composition root is its entry package, app.
// Nothing else in this component is package main. Every other package of this deployable apart from
// app, and importtarget under the banproof tag, sits under internal, so the compiler refuses an
// import of it from any other component.
package main

func main() {}
