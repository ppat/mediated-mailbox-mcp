#!/usr/bin/env node
// Falsification harness for the commit-taxonomy checks. Every defect found while deriving this
// repository's taxonomy is re-introduced here and the checks must catch it. Coverage that has never
// been falsified is not coverage, so when modelling is added, an injection is added with it.
//
// Each injection is proven to have taken effect before a catch is credited. Config defects are
// injected by overriding the readers a check calls, verified by requiring the same check to pass on
// the untouched tree first and to fail with the expected message after. Rule defects are injected
// into a generated commitlint config and verified by requiring commitlint to ACCEPT the message under
// the weakened config and REJECT it under the real one. An injection that leaves the verdict
// unchanged is reported as ineffective, never as a catch.
//
// Occupancy is guarded against vacuity first: a closure over zero cells of a manager would credit
// every catch below over nothing.

import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { CHECKS, internals, repoEnv } from './checks.mjs'

const root = process.cwd()
const SANDBOX = join(root, 'node_modules/.commit-taxonomy-self-test')
const base = () => repoEnv(root, { prCommits: 0, prBody: '' })
const withOverrides = (overrides) => repoEnv(root, { prCommits: 0, prBody: '', overrides })
const commit = (message, paths) => ({ sha: 'deadbeefcafe0000', message, paths })

// Reader overrides that swap one file's parsed content, leaving every other read intact.
const patchJson = (target, mutate) => {
  const b = base()
  return { json: (rel) => (rel === target ? mutate(structuredClone(b.json(rel))) : b.json(rel)) }
}
const patchYaml = (target, mutate) => {
  const b = base()
  return { yaml: (rel) => (rel === target ? mutate(structuredClone(b.yaml(rel))) : b.yaml(rel)) }
}
const patchText = (target, mutate) => {
  const b = base()
  return { text: (rel) => (rel === target ? mutate(b.text(rel)) : b.text(rel)) }
}
const patchCommitlint = (mutate) => {
  const b = base()
  return { commitlint: () => { const c = b.commitlint(); return mutate({ ...c, rules: JSON.parse(JSON.stringify(c.rules)) }) } }
}
const patchRenovate = (mutate) => patchJson('.github/renovate.json', mutate)

// Rules are located by what they claim, never by index, so a reordering of the file does not turn
// an injection into a no-op.
const ruleWhere = (config, predicate) => {
  const i = config.packageRules.findIndex(predicate)
  if (i === -1) throw new Error('self-test cannot find the rule it injects into; the config moved under it')
  return i
}
const internalPinsRule = (r) => Array.isArray(r.matchFileNames) && r.matchFileNames.includes('testsupport/**')
const actionsRule = (r) => Array.isArray(r.matchManagers) && r.matchManagers.includes('github-actions') && r.semanticCommitScope === 'github-actions'
const presetPinRule = (r) => Array.isArray(r.matchManagers) && r.matchManagers.includes('renovate-config')
const versionMovingRule = (r) => Array.isArray(r.matchUpdateTypes) && r.matchUpdateTypes.includes('rollback')
const goBunTypeResetRule = (r) => Array.isArray(r.matchUpdateTypes) && r.matchUpdateTypes.includes('digest') && r.semanticCommitType === 'fix' && Array.isArray(r.matchDepNames)
const goBunMajorRule = (r) => Array.isArray(r.matchUpdateTypes) && r.matchUpdateTypes.includes('major') && typeof r.commitMessagePrefix === 'string' && r.commitMessagePrefix.includes('!')

// A commitlint config that requires the real one and weakens exactly one rule. Written inside
// node_modules so that `extends: ['@commitlint/config-conventional']` still resolves.
const weakenedConfig = (name, body) => {
  mkdirSync(SANDBOX, { recursive: true })
  const path = join(SANDBOX, `${name}.cjs`)
  writeFileSync(path, `const base = require(${JSON.stringify(join(root, 'commitlint.config.js'))})\n${body}\n`)
  return path
}
const weakenRule = (name, rule) => weakenedConfig(name, `module.exports = { ...base, rules: { ...base.rules, '${rule}': [0] } }`)
const HEADER_ONLY_BREAKING = weakenedConfig('breaking-header-only', `
const headerOnly = (parsed) => {
  if (!/^\\w+(\\([^)]*\\))?!:/.test(parsed.header || '')) return [true]
  return [['feat', 'fix', 'perf', 'refactor', 'revert'].includes(parsed.type || ''), 'header-only breaking check']
}
module.exports = { ...base, plugins: [{ rules: { ...base.plugins[0].rules, 'local/breaking-type-restriction': headerOnly } }] }`)

// Offline copies of the extended presets, so a preset bump can be rehearsed before it is taken.
const offlinePresets = async (mutate) => {
  const dir = join(SANDBOX, `presets-${Math.random().toString(36).slice(2)}`)
  mkdirSync(dir, { recursive: true })
  const b = base()
  for (const ref of b.json('.github/renovate.json').extends ?? []) {
    const m = ref.match(/^github>ppat\/renovate-presets(?::([\w-]+))?#(.+)$/)
    if (!m) continue
    const [, name = 'default', tag] = m
    const preset = JSON.parse(await b.fetchText(`https://raw.githubusercontent.com/ppat/renovate-presets/${tag}/${name}.json`))
    writeFileSync(join(dir, `${name}.json`), JSON.stringify(mutate(name, preset), null, 2))
  }
  return dir
}
const renameUpstreamScopes = (_name, preset) => {
  const walk = (node) => {
    if (Array.isArray(node)) return node.forEach(walk)
    if (!node || typeof node !== 'object') return
    for (const [key, value] of Object.entries(node)) {
      if (key === 'semanticCommitScope' && value) node[key] = `renamed-${value}`
      else walk(value)
    }
  }
  walk(preset)
  return preset
}

// --- the catalogue ---------------------------------------------------------------------------

const CASES = [
  {
    id: 'I1', check: 'closure', expect: /scope 'deps' at .* is not in scope-enum/,
    defect: 'a local rule claims a scope the enum rejects',
    overrides: patchRenovate((c) => { c.packageRules[ruleWhere(c, internalPinsRule)].semanticCommitScope = 'deps'; return c }),
  },
  {
    id: 'I2', check: 'truth', expect: /testsupport\/cmd\/pgrun\/image\.go .* renders 'fix:', a claim that a shipped artifact changed/,
    defect: 'the test PostgreSQL image left unclaimed, so its digest bump renders fix: and cuts a release for a test substrate',
    overrides: patchRenovate((c) => { const r = c.packageRules[ruleWhere(c, internalPinsRule)]; r.matchFileNames = r.matchFileNames.filter((f) => f !== 'testsupport/**'); return c }),
  },
  {
    id: 'I3', check: 'truth', expect: /renovate-config \.github\/renovate\.json .* renders 'feat!:'/,
    defect: 'the preset pin left unclaimed, so a preset major renders feat!: and cuts a release with a breaking section',
    overrides: patchRenovate((c) => {
      c.packageRules.splice(ruleWhere(c, presetPinRule), 1)
      const r = c.packageRules[ruleWhere(c, internalPinsRule)]
      r.matchFileNames = r.matchFileNames.filter((f) => f !== '.github/**')
      return c
    }),
  },
  {
    id: 'I4', check: 'closure', expect: /'chore\(github-actions\)!: update something/,
    defect: 'the breaking marker left on a major of an action or the runner, which release-please bumps on and commitlint rejects',
    // Two rules strip it, the internal-pins claim over .github/** and the github-actions claim, so both are
    // removed: an injection the other rule absorbs would prove nothing.
    overrides: patchRenovate((c) => {
      delete c.packageRules[ruleWhere(c, actionsRule)].commitMessagePrefix
      const r = c.packageRules[ruleWhere(c, internalPinsRule)]
      r.matchFileNames = r.matchFileNames.filter((f) => f !== '.github/**')
      return c
    }),
  },
  {
    id: 'I5', check: 'truth', expect: /go\.mod go major renders 'feat:' without a breaking marker/,
    defect: 'the marker dropped from a shipped major by the go and bun groups setting the prefix back to unset',
    overrides: patchRenovate((c) => { c.packageRules.splice(ruleWhere(c, goBunMajorRule), 1); return c }),
  },
  {
    id: 'I5b', check: 'truth', expect: /go\.mod github\.com\/jackc\/pgx\/v5 rollback renders 'chore:', a hidden type, but go\.mod ships/,
    defect: 'a rollback of a shipped dependency typed chore by the preset, so a version move reaches the images with no release',
    overrides: patchRenovate((c) => { c.packageRules.splice(ruleWhere(c, versionMovingRule), 1); return c }),
  },
  {
    id: 'I5c', check: 'truth', expect: /mise mise\.toml go rollback renders 'chore:', a hidden type, but mise\.toml ships/,
    defect: 'the go and bun groups resetting only patch and digest to fix, so the internal claim over mise.toml types their rollback chore',
    overrides: patchRenovate((c) => { c.packageRules[ruleWhere(c, goBunTypeResetRule)].matchUpdateTypes = ['patch', 'digest']; return c }),
  },
  {
    id: 'I6', check: 'closure', expect: /semanticCommits resolves to 'disabled'/,
    defect: 'semantic commits switched off, so Renovate emits no type(scope) prefix at all',
    overrides: patchRenovate((c) => { c.semanticCommits = 'disabled'; return c }),
  },
  {
    id: 'I7', check: 'closure', expect: /'commitMessage'.*does not model it/,
    defect: 'a whole-message template, which replaces the prefix and bypasses every field modelled here',
    overrides: patchRenovate((c) => { c.commitMessage = '{{{commitMessageAction}}} {{{depName}}}'; return c }),
  },
  {
    id: 'I8', check: 'closure', expect: /'prTitle'.*does not model it/,
    defect: 'a pull request title template, which makes the linted commit header and the landing title two strings',
    overrides: patchRenovate((c) => { c.packageRules[ruleWhere(c, actionsRule)].prTitle = 'bump {{depName}}'; return c }),
  },
  {
    id: 'I9', check: 'closure', expect: /uses matchCurrentVersion, which the fold does not model/,
    defect: 'a matcher the fold cannot evaluate on a rule that sets a header field',
    overrides: patchRenovate((c) => { c.packageRules[ruleWhere(c, actionsRule)].matchCurrentVersion = '<1.0.0'; return c }),
  },
  {
    id: 'I10', check: 'closure', expect: /pull-request-title-pattern renders 'release 1\.2\.3', which commitlint rejects/,
    defect: 'a release-please title pattern rendering a header the gate rejects',
    overrides: patchJson('release-please-config.json', (c) => { c['pull-request-title-pattern'] = 'release ${version}'; return c }),
  },
  {
    id: 'I11a', check: 'self-consistency', expect: /entry for type 'style', which type-enum rejects/,
    defect: 'the changelog grows a section for a type the enum rejects',
    overrides: patchJson('release-please-config.json', (c) => { c['changelog-sections'].push({ type: 'style', section: 'Style' }); return c }),
  },
  {
    id: 'I11b', check: 'self-consistency', expect: /type 'style' is legal but has no changelog-sections entry/,
    defect: 'a type becomes legal with no changelog section, so a window holding only that type cuts no release',
    overrides: patchCommitlint((c) => { c.rules['type-enum'][2].push('style'); return c }),
  },
  {
    id: 'I11c', check: 'self-consistency', expect: /names 'deps', which the enums reject/,
    defect: 'a local commitlint rule reasons over a scope the enum rejects, so the predicate can never fire',
    overrides: (() => { const b = base(); return { commitlintSource: () => `${b.commitlintSource()}\nconst DEAD_SCOPES = ['deps']\n` } })(),
  },
  {
    id: 'I12a', check: 'self-consistency', expect: /commit-taxonomy job has a needs:/,
    defect: 'the job given a needs:, so an upstream failure makes it report skipped, which satisfies the required context',
    overrides: patchYaml('.github/workflows/lint.yaml', (w) => { w.jobs['commit-taxonomy'].needs = ['detect-changes']; return w }),
  },
  {
    id: 'I12e', check: 'self-consistency', expect: /commit-messages job has a needs:/,
    defect: 'the commit-messages job given a needs:, so it can report skipped, which satisfies the required context',
    overrides: patchYaml('.github/workflows/lint.yaml', (w) => { w.jobs['commit-messages'].needs = ['detect-changes']; return w }),
  },
  {
    id: 'I12f', check: 'self-consistency', expect: /commit-messages job has the condition/,
    defect: 'the commit-messages job given a condition that is false on some pull requests, so it reports skipped on them, which satisfies the required context',
    overrides: patchYaml('.github/workflows/lint.yaml', (w) => { w.jobs['commit-messages'].if = "${{ github.event_name == 'pull_request' && github.actor != 'renovate[bot]' }}"; return w }),
  },
  {
    id: 'I12b', check: 'self-consistency', expect: /lint\.yaml filters pull_request by paths/,
    defect: 'the workflow path-gated, so the required context can never clear on a pull request that misses the filter',
    overrides: patchYaml('.github/workflows/lint.yaml', (w) => { (w.on ??= {}).pull_request = { paths: ['.github/**'] }; return w }),
  },
  {
    id: 'I12c', check: 'self-consistency', expect: /pr-title\.yaml does not run on pull_request edited/,
    defect: 'the title lint no longer re-runs when the title is edited after the last push',
    overrides: patchYaml('.github/workflows/pr-title.yaml', (w) => { (w.on ?? w[true]).pull_request.types = ['opened', 'synchronize', 'reopened']; return w }),
  },
  {
    id: 'I12d', check: 'self-consistency', expect: /commit-msg hook runs @commitlint\/cli 20\.0\.0 and the gates run/,
    defect: 'the local commit-msg hook and the gates resolving different commitlint versions',
    overrides: patchText('.pre-commit-config.yaml', (t) => t.replace(/@commitlint\/cli@[^'"\s]+/, '@commitlint/cli@20.0.0')),
  },
  {
    id: 'I13a', check: 'message-shape', expect: /body paragraph is shaped like a conventional commit header/,
    defect: 'a body paragraph the release parser reads as a second commit',
    overrides: { commits: () => [commit('ci(internal-workflows): tidy\n\nfeat: add a feature nobody wrote\n', ['.github/workflows/lint.yaml'])] },
  },
  {
    id: 'I13b', check: 'message-shape', expect: /Release-As: footer overrides the computed version/,
    defect: 'a Release-As: footer overriding the computed version',
    overrides: { commits: () => [commit('ci(internal-workflows): tidy\n\nRelease-As: 9.9.9\n', ['.github/workflows/lint.yaml'])] },
  },
  {
    id: 'I13c', check: 'message-shape', expect: /BEGIN_COMMIT_OVERRIDE block, which replaces the release-facing message/,
    defect: 'an override block in the pull request body',
    overrides: { prBody: () => 'BEGIN_COMMIT_OVERRIDE\nfeat: whatever\nEND_COMMIT_OVERRIDE' },
  },
  {
    id: 'I14', check: 'empty-scope', expect: /type 'feat' asserts a shipped artifact changed, but no changed path ships/,
    defect: 'a claim type on the empty scope over a diff that reaches no consumer',
    overrides: { commits: () => [commit('feat: rework the test runner and the agent rules\n', ['testsupport/cmd/pgrun/image.go', 'CLAUDE.md', 'mediate/importtarget/target.go'])] },
  },
  {
    id: 'I15', check: 'named-scope', expect: /scope 'agents' does not cover README\.md, and no changed path is in its footprint/,
    defect: 'a named scope on a diff touching nothing in its footprint',
    overrides: { commits: () => [commit('docs(agents): describe the layout\n', ['README.md'])] },
  },
  {
    id: 'I15c', check: 'named-scope', expect: /scope 'renovate' means a version moved and nothing else did, but the diff also changes mise\.toml/,
    defect: 'a line-level scope on a diff that also hand-edits another surface',
    overrides: { commits: () => [commit('chore(renovate): bump the preset\n', ['.github/renovate.json', 'mise.toml'])] },
  },
  {
    id: 'I15b', check: 'named-scope', guard: true,
    defect: 'a named scope on a diff scoped to what motivated it, with paths outside the footprint beside it, must NOT fire',
    overrides: { commits: () => [commit('ci(internal-workflows): enforce the vocabulary\n', ['.github/workflows/lint.yaml', '.claude/rules/commits.md', 'docs/adr/README.md'])] },
  },
  {
    id: 'I16', check: 'truth', expect: /tests\/chainsaw\/kind\.yaml pinned-thing minor renders 'feat:'/,
    defect: 'a new comment-pinned version in an unshipped directory, which the emitter would type by update type',
    overrides: (() => {
      const b = base()
      const file = 'tests/chainsaw/kind.yaml'
      return {
        tracked: () => [...b.tracked(), file],
        text: (rel) => (rel === file ? '# renovate: datasource=github-releases depName=pinned-thing\nversion: "1.0.0"\n' : b.text(rel)),
      }
    })(),
  },
  {
    id: 'I17', check: 'closure', expect: /packaging\/chart\/Chart\.lock \(helmv3\) is read by a Renovate manager this check does not model/,
    defect: 'a dependency file only an unmodelled manager reads, whose headers would be emitted and never enumerated',
    overrides: (() => { const b = base(); return { tracked: () => [...b.tracked(), 'packaging/chart/Chart.lock'] } })(),
  },
  {
    id: 'I18', check: 'truth', expect: /claimed by github>ppat\/renovate-presets#[^ ]+ \(top level\), a shared preset/,
    defect: 'the shipped empty scope inherited from the preset rather than claimed locally, so a preset rename would move every shipped header',
    overrides: patchRenovate((c) => { delete c.semanticCommitScope; return c }),
  },
  // Rule defects, proven against commitlint itself.
  { id: 'R1', reject: 'feat(internal-workflows): speed up the lint job', accept: 'ci(internal-workflows): speed up the lint job', defect: 'a claim type on an internal scope' },
  { id: 'R2', reject: 'chore(internal-dependencies)!: update pre-commit packages (major)', accept: 'chore(internal-dependencies): update pre-commit packages (major)', defect: 'a dependency major cutting a release of the artifacts' },
  { id: 'R3', reject: 'chore(agents): tidy\n\nsome prose\nBREAKING-CHANGE: everything\n', accept: 'chore(agents): tidy\n\nsome prose\n', defect: 'a mid-body breaking marker, which the release layer honours and a header lint cannot see' },
  { id: 'R4', reject: 'build: lay out the tooling', accept: 'ci(internal-workflows): lay out the tooling', defect: 'a hidden type with no local meaning, whose use would vanish silently' },
  { id: 'R5', reject: 'style: reflow the comments', accept: 'docs: reflow the comments', defect: 'a rendered type for cosmetic change, which would cut a release' },
  { id: 'R6', reject: 'ci: tidy the workflow', accept: 'ci(internal-workflows): tidy the workflow', defect: 'continuous-integration machinery on the empty scope, where it would read as shipped or repository-level' },
  { id: 'R7', reject: 'feat(core): add the gate', accept: 'feat: add the gate', defect: 'a component name as a scope, which the labels carry and the enum refuses' },
  { id: 'R8', reject: 'chore(internal-workflows): tidy\n\nBREAKING CHANGE: the workflows moved\n', weakened: HEADER_ONLY_BREAKING, defect: 'the breaking-marker rule reverted to regexing the header only, which passes every footer spelling' },
  { id: 'R9', reject: 'feat(): rework everything', weakened: weakenRule('no-empty-parens-off', 'local/no-empty-parens'), defect: 'the empty-parens rule removed, so a header that looks scoped silently carries the empty scope\'s claim' },
  { id: 'R10', reject: 'fix(agents): correct the rule', weakened: weakenRule('pairing-off', 'local/type-scope-pairing'), defect: 'the pairing rule removed, so a claim type sits on an internal scope' },
]

// --- runner ----------------------------------------------------------------------------------

const results = []
const record = (id, verdict, detail) => { results.push({ id, verdict, detail }); console.log(`  ${verdict.padEnd(11)} ${id}  ${detail}`) }
const run = async (name, env) => {
  try { return await CHECKS[name](env) } catch (e) { return { fail: [`threw: ${e.message}`], note: [] } }
}

console.log('occupancy: every modelled manager must have cells, or a catch below is credited over nothing')
{
  const { cells } = internals.occupancy(base())
  const expected = { gomod: 2, dockerfile: 2, mise: 2, bun: 3, 'pre-commit': 1, 'github-actions': 2, 'custom.regex': 2, 'renovate-config': 1 }
  for (const [manager, minimum] of Object.entries(expected)) {
    const n = cells.filter((c) => c.manager === manager).length
    if (n < minimum) record(`occupancy-${manager}`, 'VACUOUS', `${n} cell(s) for ${manager}, fewer than the ${minimum} this tree is known to hold`)
    else console.log(`  ok          ${manager}: ${n} cells`)
  }
  for (const must of [['custom.regex', 'testsupport/cmd/pgrun/image.go'], ['renovate-config', '.github/renovate.json'], ['github-actions', '.github/workflows/lint.yaml'], ['bun', 'ui/browser/package.json'], ['mise', 'mise.toml'], ['gomod', 'go.mod']]) {
    if (!cells.some((c) => c.manager === must[0] && c.packageFile === must[1])) record(`occupancy-${must[1]}`, 'VACUOUS', `no ${must[0]} cell extracted from ${must[1]}`)
  }
}

console.log('\nbaseline: every check must pass on the untouched tree, or a catch below proves nothing')
for (const name of Object.keys(CHECKS)) {
  const { fail } = await run(name, base())
  if (fail.length) record(name, 'BASELINE-RED', fail.join(' / '))
  else console.log(`  ok          ${name}`)
}

console.log('\ninjections')
for (const c of CASES) {
  if (c.reject !== undefined) {
    const rejected = base().lint(c.reject)
    if (rejected.accepted) { record(c.id, 'NOT CAUGHT', `${c.defect}: commitlint accepted it`); continue }
    const control = c.weakened ? base().lint(c.reject, c.weakened) : base().lint(c.accept)
    if (!control.accepted) { record(c.id, 'INEFFECTIVE', `${c.defect}: the control is rejected too, so the rejection is not attributable`); continue }
    record(c.id, 'caught', c.defect)
    continue
  }
  const before = await run(c.check, base())
  const injected = await run(c.check, withOverrides(c.overrides))
  if (c.guard) {
    if (injected.fail.length) record(c.id, 'NOT CAUGHT', `${c.defect}: ${c.check} fired: ${injected.fail.join(' / ')}`)
    else record(c.id, 'held', c.defect)
    continue
  }
  if (before.fail.length) { record(c.id, 'INCONCLUSIVE', `${c.defect}: ${c.check} was already red before injection`); continue }
  if (!injected.fail.length) { record(c.id, 'NOT CAUGHT', `${c.defect}: ${c.check} still passes, so the injection changed nothing the check reads`); continue }
  if (!injected.fail.some((f) => c.expect.test(f))) { record(c.id, 'WRONG CATCH', `${c.defect}: ${c.check} failed for another reason: ${injected.fail.join(' / ')}`); continue }
  record(c.id, 'caught', c.defect)
}

// P1: the preset-bump rehearsal, guarded in both directions. After this repository claims every
// scope locally, an upstream rename must move no header, so the guard is that closure and truth stay
// green under a renamed preset, and the falsifier is that truth goes red once the local claims are
// gone, without which the guard would be satisfied by a check that reads nothing.
{
  const dir = await offlinePresets(renameUpstreamScopes)
  const renamed = repoEnv(root, { prCommits: 0, prBody: '', offlinePresets: dir })
  const guard = [...(await run('closure', renamed)).fail, ...(await run('truth', renamed)).fail]
  if (guard.length) record('P1-guard', 'NOT CAUGHT', `an upstream scope rename reaches this repository's headers: ${guard.join(' / ')}`)
  else record('P1-guard', 'held', 'an upstream scope rename moves no header here')

  const stripped = repoEnv(root, {
    prCommits: 0, prBody: '', offlinePresets: dir,
    overrides: patchRenovate((c) => { c.packageRules = c.packageRules.filter((r) => !r.semanticCommitScope); delete c.semanticCommitScope; return c }),
  })
  const falsifier = [...(await run('closure', stripped)).fail, ...(await run('truth', stripped)).fail]
  if (falsifier.length) record('P1-falsifier', 'caught', 'with the local claims removed, the renamed preset decides the scope')
  else record('P1-falsifier', 'NOT CAUGHT', 'the guard above is vacuous: removing the local claims changed nothing')

  const unmodelled = await offlinePresets((name, preset) => {
    if (name === 'default') preset.packageRules[0].commitMessagePrefix = '{{depName}}-chore:'
    return preset
  })
  const refuse = await run('closure', repoEnv(root, { prCommits: 0, prBody: '', offlinePresets: unmodelled }))
  if (refuse.fail.some((f) => /neither a semanticCommit\* template nor a literal header/.test(f))) record('P1-refuse', 'caught', 'a preset bump introducing an unmodelled commitMessagePrefix is refused rather than assumed harmless')
  else record('P1-refuse', 'NOT CAUGHT', `an unmodelled commitMessagePrefix passed silently: ${refuse.fail.join(' / ') || 'nothing reported'}`)
}

rmSync(SANDBOX, { recursive: true, force: true })

const bad = results.filter((r) => r.verdict !== 'caught' && r.verdict !== 'held')
console.log(`\n${results.length - bad.length} of ${results.length} assertions held`)
if (bad.length) {
  console.error('\nself-test FAILED: the checks below cannot be trusted:')
  for (const r of bad) console.error(`  - ${r.id} [${r.verdict}] ${r.detail}`)
  process.exit(1)
}
