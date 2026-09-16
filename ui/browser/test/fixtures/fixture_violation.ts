// Violation file. A fixture module is a contract-typed recording, so it declares no any and asserts no
// type, which the browser lint bans in every file under test/fixtures (ADR-0064, ADR-0072).
type Row = { subject: string };

export const untyped: any = {}; // want oxlint "no-explicit-any"
export const asserted = {} as Row; // want oxlint "consistent-type-assertions"
export const angled = <Row>{}; // want oxlint "consistent-type-assertions"
