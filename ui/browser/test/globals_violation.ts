// Violation file. The runner also provides two of its substitution helpers as globals in test files,
// with no import, and the browser lint bans reading them (ADR-0064, ADR-0072).
export const fromGlobals = [
  jest.fn(), // want oxlint "no-restricted-globals.*'jest'"
  vi.fn(), // want oxlint "no-restricted-globals.*'vi'"
];
