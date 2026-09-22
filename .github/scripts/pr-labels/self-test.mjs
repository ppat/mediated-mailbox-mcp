#!/usr/bin/env node
// The self-test the `pr-labels` workflow runs before it touches a label. Each case is a way the
// mapping could go wrong silently, and each expectation is written from the rule in CLAUDE.md's
// Repository process, never from what the code returns.

import { deepStrictEqual, throws } from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { labelChanges, parseComponents, wantedLabels } from './labels.mjs'

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

const directories = parseComponents(table([
  '| `core/` | Library | `mediated-mailbox-core` | The shared pure library |',
  '| `mediate/` | Deployable | `mediated-mailbox-mediate` | The mediator |',
  '| `packaging/chart/` | Packaging | `mediated-mailbox` | The Helm chart |',
  '| `tests/chainsaw/` | System tests | | The chainsaw suite |',
]))

const cases = [
  ['the table is read up to its end and no further', () =>
    deepStrictEqual(directories, ['core/', 'mediate/', 'packaging/chart/', 'tests/chainsaw/'])],

  ['a second table in the same section is not read as components', () =>
    deepStrictEqual(parseComponents([
      '### Components', '',
      '| Directory | Kind |', '| --- | --- |', '| `core/` | Library |', '',
      'Prose between the tables.', '',
      '| Directory | Note |', '| --- | --- |', '| `other/` | not a component |',
    ].join('\n')), ['core/'])],

  ['a diff spanning two components calls for both labels', () =>
    deepStrictEqual(wantedLabels(['core/mail/port.go', 'mediate/main.go', 'ROADMAP.md'], directories),
      ['component:core', 'component:mediate'])],

  ['a two-segment directory keeps both segments in its label', () =>
    deepStrictEqual(wantedLabels(['packaging/chart/Chart.yaml', 'tests/chainsaw/.chainsaw.yaml'], directories),
      ['component:packaging/chart', 'component:tests/chainsaw'])],

  ['a path under the first segment only of a two-segment directory is no component', () =>
    deepStrictEqual(wantedLabels(['packaging/README.md', 'tests/README.md'], directories), [])],

  ['a directory whose name starts with a component\'s name is no component', () =>
    deepStrictEqual(wantedLabels(['core-extra/x.go', 'mediated/x.go', 'core'], directories), [])],

  ['a diff touching no component calls for no label', () =>
    deepStrictEqual(wantedLabels(['CLAUDE.md', 'docs/adr/README.md', '.github/workflows/lint.yaml', 'go.mod'], directories), [])],

  ['many paths in one component call for its label once', () =>
    deepStrictEqual(wantedLabels(['core/a.go', 'core/b/c.go'], directories), ['component:core'])],

  ['a stale component label is removed and a missing one added', () =>
    deepStrictEqual(labelChanges(['unit:F4', 'component:ui', 'component:core'], ['component:core', 'component:mediate']),
      { add: ['component:mediate'], remove: ['component:ui'] })],

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

  // The repository's own table must parse, or the workflow would fail on every pull request.
  ['CLAUDE.md in this tree parses', () => {
    const own = parseComponents(readFileSync('CLAUDE.md', 'utf8'))
    if (own.length === 0) throw new Error('no components parsed')
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
