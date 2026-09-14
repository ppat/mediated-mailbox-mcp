// Command organize is the reorganization workload, published as mediated-mailbox-organize.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
package main

func main() {}
