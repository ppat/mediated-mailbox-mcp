// Command organize is the Reorg Engine's apply and rollback job kind, published as
// mediated-mailbox-organize.
//
// This file sets the process up. The deployable's composition root is its entry package, app.
// Nothing else in this component is package main. Every other package of this deployable apart from
// app sits under internal, so the compiler refuses an import of it from any other component.
package main

func main() {}
