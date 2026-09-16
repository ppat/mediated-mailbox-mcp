// Package provider is the provider adapter library, published as mediated-mailbox-provider. It is a
// narrow, named exception to the rule that shared code is pure.
//
// Each adapter is one subpackage and implements the Provider Port declared in core/mail, together
// with its rate profile. The provider fake and the contract suite are subpackages too. Nothing
// belongs in this package itself.
package provider
