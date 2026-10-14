#!/usr/bin/env node
// The self-test the `pr-labels` workflow runs before it touches a label. Each case is a way the
// mapping could go wrong silently, and each expectation is written from the rule in CLAUDE.md's
// Repository process, never from what the code returns.

import { deepStrictEqual, throws } from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { changedPaths, labelChanges, labelsAreOurs, parseComponents, wantedLabels } from './labels.mjs'

// A throwaway repository shaped like a pull request whose head merged a newer base commit. `main`
// starts at `fork`, the head moves a file out of provider/ and adds a file with a non-ASCII name
// under core/, `main` then gains a commit under api/, the head merges `main`, and `main` gains
// one more commit the head lacks. The event's base commit is `fork`. The caller's global and system
// git configuration are shut out, because commit signing or a hooks path there can break the setup,
// and `core.quotepath=false` there would let the quoted-path case pass without `-z`.
// Set on the process, so the diffs the cases run under test inherit it too.
process.env.GIT_CONFIG_GLOBAL = '/dev/null'
process.env.GIT_CONFIG_NOSYSTEM = '1'
function scratchRepository(dir) {
  const git = (...args) => execFileSync('git', ['-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'init.defaultBranch=main', ...args], { cwd: dir, encoding: 'utf8' }).trim()
  const commit = (path, message) => {
    mkdirSync(join(dir, dirname(path)), { recursive: true })
    writeFileSync(join(dir, path), message)
    git('add', '-A')
    git('commit', '-q', '-m', message)
  }
  git('init', '-q')
  commit('README.md', 'first')
  commit('provider/moved.go', 'fork')
  const fork = git('rev-parse', 'HEAD')
  git('switch', '-q', '-c', 'head')
  git('mv', 'provider/moved.go', 'moved.go')
  commit('core/é.go', 'head')
  git('switch', '-q', 'main')
  commit('api/main.go', 'base moves')
  git('switch', '-q', 'head')
  git('merge', '-q', '--no-edit', 'main')
  git('switch', '-q', 'main')
  commit('api/later.go', 'base moves after the merge')
  git('switch', '-q', 'head')
  return { dir, fork }
}
const scratchDir = mkdtempSync(join(tmpdir(), 'pr-labels-'))
process.on('exit', () => rmSync(scratchDir, { recursive: true, force: true }))
const scratch = scratchRepository(scratchDir)

const table = (rows) => [
  '## Code layout and conventions',
  '',
  '### Components',
  '',
  'Each row is one top-level directory.',
  '',
  '| Directory | Kind | Published as | Holds |',
  '| --- | --- | --- | --- |',
  ...rows,
  '',
  'A deployable\'s job word is a verb.',
  '',
  '### Inside a component',
  '',
  '| Convention | Rule |',
  '| --- | --- |',
  '| `not/a-component/` | a later table that must not be read |',
].join('\n')

const components = parseComponents(table([
  '| `core/` | Library | `mediated-mailbox-core` | The shared pure library |',
  '| `api/` | Deployable | `mediated-mailbox-api` | The API server |',
  '| `packaging/chart/`, `packaging/tests/chainsaw/` | Packaging | `mediated-mailbox` | The Helm chart and the chainsaw suite |',
]))

const cases = [
  ['the table is read up to its end and no further', () =>
    deepStrictEqual(components, [
      { directories: ['core/'], label: 'component:core' },
      { directories: ['api/'], label: 'component:api' },
      { directories: ['packaging/chart/', 'packaging/tests/chainsaw/'], label: 'component:packaging/chart' },
    ])],

  ['a second table in the same section is not read as components', () =>
    deepStrictEqual(parseComponents([
      '### Components', '',
      '| Directory | Kind |', '| --- | --- |', '| `core/` | Library |', '',
      'Prose between the tables.', '',
      '| Directory | Note |', '| --- | --- |', '| `other/` | not a component |',
    ].join('\n')), [{ directories: ['core/'], label: 'component:core' }])],

  ['a diff spanning two components calls for both labels', () =>
    deepStrictEqual(wantedLabels(['core/mail/port.go', 'api/main.go', 'ROADMAP.md'], components),
      ['component:api', 'component:core'])],

  ['a directory of several segments keeps every segment in its label', () =>
    deepStrictEqual(wantedLabels(['packaging/chart/Chart.yaml'], components), ['component:packaging/chart'])],

  ['a path under the chainsaw suite is labelled as the chart, and no label of its own is produced', () =>
    deepStrictEqual(wantedLabels(['packaging/tests/chainsaw/.chainsaw.yaml'], components), ['component:packaging/chart'])],

  ['the chart and the chainsaw suite together call for the chart label once', () =>
    deepStrictEqual(wantedLabels(['packaging/chart/Chart.yaml', 'packaging/tests/chainsaw/a/b.yaml'], components), ['component:packaging/chart'])],

  ['a second directory in a row is read from a comma-separated first cell', () =>
    deepStrictEqual(parseComponents(table(['| `a/`, `b/c/` | Library | | |'])), [{ directories: ['a/', 'b/c/'], label: 'component:a' }])],

  ['a directory listed in two rows fails', () =>
    throws(() => parseComponents(table(['| `a/`, `b/` | Library | | |', '| `b/` | Library | | |'])), /twice/)],

  ['a first cell mixing a directory with other text fails', () =>
    throws(() => parseComponents(table(['| `a/` and `b/` | Library | | |'])), /cannot read a component directory/)],

  ['the labels of an open pull request are the workflow\'s to set', () =>
    deepStrictEqual(labelsAreOurs('open'), true)],

  // The script itself, run as the workflow runs it, with no token and no repository: it must reach
  // its exit before it needs either, so a closed pull request can change nothing.
  ['the script exits cleanly on a closed pull request before it reads a token or touches a label', () => {
    const out = execFileSync('node', [join(import.meta.dirname, 'sync-pr-labels.mjs')], {
      encoding: 'utf8', env: { PATH: process.env.PATH, PR_NUMBER: '7', PR_STATE: 'closed' },
    })
    if (!/left alone/.test(out)) throw new Error(`no statement that the labels were left alone: ${out}`)
  }],

  ['the script does not exit early on an open pull request', () =>
    throws(() => execFileSync('node', [join(import.meta.dirname, 'sync-pr-labels.mjs')], {
      stdio: 'pipe', env: { PATH: process.env.PATH, PR_NUMBER: '7', PR_STATE: 'open' },
    }), /BASE_REF is not set/)],

  ['the labels of a closed pull request are left alone, merged ones included', () =>
    deepStrictEqual(['closed', 'merged', '', undefined].map(labelsAreOurs), [false, false, false, false])],

  ['a path under only the leading segments of a component directory is no component', () =>
    deepStrictEqual(wantedLabels(['packaging/README.md', 'packaging/tests/README.md'], components), [])],

  ['a directory whose name starts with a component\'s name is no component', () =>
    deepStrictEqual(wantedLabels(['core-extra/x.go', 'apis/x.go', 'core'], components), [])],

  ['a diff touching no component calls for no label', () =>
    deepStrictEqual(wantedLabels(['CLAUDE.md', 'docs/adr/README.md', '.github/workflows/lint.yaml', 'go.mod'], components), [])],

  ['many paths in one component call for its label once', () =>
    deepStrictEqual(wantedLabels(['core/a.go', 'core/b/c.go'], components), ['component:core'])],

  ['a stale component label is removed and a missing one added', () =>
    deepStrictEqual(labelChanges(['unit:F4', 'component:ui', 'component:core'], ['component:core', 'component:api']),
      { add: ['component:api'], remove: ['component:ui'] })],

  ['labels outside the component prefix are never touched', () =>
    deepStrictEqual(labelChanges(['unit:F4', 'automerge:off', 'componentry'], []), { add: [], remove: [] })],

  ['no component wanted removes every component label', () =>
    deepStrictEqual(labelChanges(['component:packaging/chart', 'unit:R1'], []), { add: [], remove: ['component:packaging/chart'] })],

  ['a missing heading fails rather than finding no components', () =>
    throws(() => parseComponents('# CLAUDE.md\n\nno table here\n'), /no '### Components' heading/)],

  ['a heading with no table fails rather than reading the next section\'s table', () =>
    throws(() => parseComponents('### Components\n\nprose only\n\n### Next\n\n| Directory | Kind |\n| --- | --- |\n| `next/` | Library |\n'),
      /has no component rows/)],

  ['a first column that is no longer Directory fails', () =>
    throws(() => parseComponents(table(['| `core/` | Library | | |']).replace('| Directory |', '| Path |')), /Directory column/)],

  ['a row whose first cell is not a backticked directory fails rather than being skipped', () =>
    throws(() => parseComponents(table(['| `core/` | Library | | |', '| core | Library | | |'])), /cannot read a component directory/)],

  ['a directory without its trailing slash fails', () =>
    throws(() => parseComponents(table(['| `core` | Library | | |'])), /cannot read a component directory/)],

  ['a directory listed twice fails', () =>
    throws(() => parseComponents(table(['| `core/` | Library | | |', '| `core/` | Library | | |'])), /twice/)],

  ['a path git would quote is read as written and labels its component', () =>
    deepStrictEqual(changedPaths('main', 'head', scratch.dir).filter((p) => p.startsWith('core/')), ['core/é.go'])],

  ['a file moved out of a component still labels the component it left', () =>
    deepStrictEqual(wantedLabels(changedPaths('main', 'head', scratch.dir), [...components, { directories: ['provider/'], label: 'component:provider' }]), ['component:core', 'component:provider'])],

  ['the diff against the base branch leaves out base commits, merged into the head or not', () =>
    deepStrictEqual(changedPaths('main', 'head', scratch.dir).sort(), ['core/é.go', 'moved.go', 'provider/moved.go'])],

  // The reason the workflow diffs against the branch. Against the event's older base commit, the
  // merged base commit counts as the head's own. If this stops holding, the case above proves less.
  ['the diff against an older base commit counts the merged base commit as the head\'s own', () =>
    deepStrictEqual(changedPaths(scratch.fork, 'head', scratch.dir).sort(), ['api/main.go', 'core/é.go', 'moved.go', 'provider/moved.go'])],

  // The repository's own table must parse, or the workflow would fail on every pull request.
  ['CLAUDE.md in this tree parses', () => {
    const own = parseComponents(readFileSync('CLAUDE.md', 'utf8'))
    if (own.length === 0) throw new Error('no components parsed')
  }],

  // The fold of the chainsaw suite into the chart, read from the table the workflow really reads.
  ['CLAUDE.md in this tree labels the chainsaw suite as the chart and produces no label of the suite\'s own', () => {
    const own = parseComponents(readFileSync('CLAUDE.md', 'utf8'))
    deepStrictEqual(wantedLabels(['packaging/tests/chainsaw/x.yaml'], own), ['component:packaging/chart'])
    if (own.some((c) => c.label === 'component:packaging/tests/chainsaw')) throw new Error('the suite has a label of its own')
  }],
]

let failed = 0
for (const [name, run] of cases) {
  try {
    run()
    console.log(`ok   ${name}`)
  } catch (error) {
    failed++
    console.error(`FAIL ${name}\n     ${error.message.split('\n').join('\n     ')}`)
  }
}
if (failed) {
  console.error(`\n${failed} of ${cases.length} self-test cases failed`)
  process.exit(1)
}
console.log(`\nall ${cases.length} self-test cases passed`)
