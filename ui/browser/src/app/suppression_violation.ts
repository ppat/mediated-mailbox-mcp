// Violation file. Each comment below is a suppression directive one of the browser tools honours, and the
// search for them reports every one (ADR-0072). Nothing imports this module.
/* want suppression "eslint-disable" */ // eslint-disable-next-line no-console
/* want suppression "oxlint-disable" */ // oxlint-disable-next-line no-console
export const suppressed = { dangerouslySetInnerHTML: { __html: "" } }; /* want suppression "ast-grep-ignore" */ // ast-grep-ignore: markup-prop-ts
