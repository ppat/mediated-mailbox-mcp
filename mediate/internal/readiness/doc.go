// Package readiness holds whether the mediator is ready for traffic. The mediator is ready once it
// has loaded its policy and its listeners are up, and stops being ready when it begins shutting down
// (ADR-0051). The composition root serves the answer on the readiness probe. Withholding traffic on
// that answer is the platform's.
package readiness
