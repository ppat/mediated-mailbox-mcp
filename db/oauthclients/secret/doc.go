// Package secret is the data-access subsection for writing an installation's OAuth client secret,
// which is sealed (ADR-0016, ADR-0081). It holds delta sync's re-seal of a secret by compare-and-set
// on the stored bytes and nothing else (ADR-0089, ADR-0092). Only delta sync's list admits it, so no
// other role is planned against a statement that writes a client secret.
package secret
