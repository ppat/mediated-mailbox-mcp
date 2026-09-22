// The two header fields have different natures. The type is release-operative: release-please
// sizes the version bump from it and decides from it whether a release happens at all. The scope
// is read by nothing that routes a release, because this repository releases one lockstep set of
// images and the chart, so it is a claim about the diff that only has to be true. The local rules
// below keep the two from contradicting each other: a type that asserts shipped behaviour changed
// cannot sit on a scope that reaches no consumer. The deciding rules are .claude/rules/commits.md's.

// Types that render a consumer-facing changelog line asserting a shipped artifact changed.
const CLAIM_TYPES = ['feat', 'fix', 'perf', 'refactor', 'revert']

// The scopes that reach no consumer. The empty scope is the shipped surface and the repository-level
// residue, so it is the only scope a claim type may carry.
const INTERNAL_SCOPES = ['agents', 'github-actions', 'internal-dependencies', 'internal-workflows', 'release', 'renovate']

// Which scopes each non-claim type may sit on. Continuous-integration machinery is always the
// internal-workflows surface, prose is repository-level or written for an agent, and test code sits
// in the tree beside what it tests. chore is housekeeping on any surface.
const SCOPES_BY_TYPE = {
  chore: ['', ...INTERNAL_SCOPES],
  ci: ['internal-workflows'],
  docs: ['', 'agents'],
  test: [''],
}

const describeScope = (scope) => (scope ? `scope '${scope}'` : 'the empty scope')

const validateTypeScopePairing = (parsedCommit) => {
  const type = parsedCommit.type || ''
  const scope = parsedCommit.scope || ''
  if (CLAIM_TYPES.includes(type)) {
    return [
      scope === '',
      `type '${type}' asserts that a shipped artifact changed and cannot sit on ${describeScope(scope)}, ` +
      'which reaches no consumer: drop the scope if the diff ships, or type it chore, ci, docs or test.',
    ]
  }
  const allowed = SCOPES_BY_TYPE[type]
  if (!allowed) {
    return [true]
  }
  return [
    allowed.includes(scope),
    `type '${type}' cannot sit on ${describeScope(scope)}: it takes ${allowed.map((s) => s || 'no scope').join(', ')}.`,
  ]
}

// A breaking marker bumps the version and renders in the changelog regardless of the type's hidden
// flag, so it asserts as much as a claim type does and takes only a claim type, which the pairing
// rule then confines to the empty scope.
//
// It is read from BOTH the raw header and parsedCommit.notes: the parser exposes the '!' only in the
// header text, and the 'BREAKING CHANGE:' and 'BREAKING-CHANGE:' spellings only in notes. A
// header-only test passes every footer spelling.
const validateBreakingTypeRestriction = (parsedCommit) => {
  const hasBang = /^\w+(\([^)]*\))?!:/.test(parsedCommit.header || '')
  const hasNote = (parsedCommit.notes || []).length > 0
  if (!hasBang && !hasNote) {
    return [true]
  }
  const type = parsedCommit.type || ''
  return [
    CLAIM_TYPES.includes(type),
    `a breaking marker bumps the version and renders even for a hidden type, so it cannot sit on type '${type}': ` +
    `it takes ${CLAIM_TYPES.join(', ')}, on the empty scope.`,
  ]
}

// '()' parses as NO scope rather than as an invalid one, so scope-enum accepts it: '' is a member.
// Here the empty scope is a positive claim, that the change ships or is repository-level, so a
// header that looks scoped would silently carry it. Only a raw-header test can see the parentheses.
const validateNoEmptyParens = (parsedCommit) => [
  !/^\w+\(\s*\)!?:/.test(parsedCommit.header || ''),
  "'()' is not a scope: it parses as no scope, which here claims the change ships or is repository-level. Name a scope or drop the parentheses.",
]

// Renovate and release-please write bodies nobody can rewrap, so the body limit exempts the scopes
// only those bots emit. Every other body is written by a person or an agent and wraps at 120.
const BOT_SCOPES = ['github-actions', 'internal-dependencies', 'release', 'renovate']
const BODY_MAX_LINE_LENGTH = 120

const validateBodyMaxLineLength = (parsedCommit) => {
  const { scope, body } = parsedCommit
  if (!body || BOT_SCOPES.includes(scope || '')) {
    return [true]
  }
  return [
    body.split('\n').every((line) => line.length <= BODY_MAX_LINE_LENGTH),
    `commit message body line length must not exceed ${BODY_MAX_LINE_LENGTH}`,
  ]
}

module.exports = {
  extends: ['@commitlint/config-conventional'],
  plugins: [
    {
      rules: {
        'local/type-scope-pairing': validateTypeScopePairing,
        'local/breaking-type-restriction': validateBreakingTypeRestriction,
        'local/no-empty-parens': validateNoEmptyParens,
        'local/body-max-line-length': validateBodyMaxLineLength,
      },
    },
  ],
  rules: {
    'header-max-length': [2, 'always', 120],

    'footer-max-line-length': [0, 'always'],

    'body-max-line-length': [0],
    'local/body-max-line-length': [2, 'always'],

    // This set must EQUAL the key set of changelog-sections in release-please-config.json. A type
    // with no section renders nothing, so a window holding only that type cuts no release, silently
    // and green, and a section for a type this list rejects is dead config. The commit-taxonomy check
    // asserts the equality.
    //
    // build and style are excluded. The build tooling here is continuous-integration machinery or a
    // shipped Dockerfile, so build would be the wrong-but-tempting answer for a ci or a feat change,
    // and it is hidden, so the miss would be silent. style renders and cuts a release, so a cosmetic
    // header would republish every image.
    'type-enum': [2, 'always',
      ['chore', 'ci', 'docs', 'feat', 'fix', 'perf', 'refactor', 'revert', 'test']
    ],

    // The internal surfaces, in the vocabulary every ppat repository shares, plus the empty scope
    // for the shipped surface and the repository-level residue. The component a diff touches is
    // carried by the component labels, never by the scope.
    'scope-enum': [2, 'always',
      [
        '',
        'agents',
        'github-actions',
        'internal-dependencies',
        'internal-workflows',
        'release',
        'renovate'
      ]
    ],

    'local/type-scope-pairing': [2, 'always'],
    'local/breaking-type-restriction': [2, 'always'],
    'local/no-empty-parens': [2, 'always'],

    'body-case': [0, 'always']
  }
}
