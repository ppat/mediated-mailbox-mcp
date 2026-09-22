#!/usr/bin/env node
// Entry point for the commit-taxonomy checks. One check per invocation so the workflow can run each
// as a named step: the whole set reports a single required status context, and the step name is the
// only thing that says which one went red.
//
//   check-commit-taxonomy.mjs <check> [--offline-presets DIR]
//   check-commit-taxonomy.mjs --list
//   check-commit-taxonomy.mjs --dump-headers
//
// --offline-presets replaces the fetch of every ppat/renovate-presets file with a read from DIR,
// which is how a preset bump is rehearsed before it is taken. --dump-headers prints every emittable
// header with the cell and update type that produces it.

import { CHECKS, internals, repoEnv } from './checks.mjs'

const args = process.argv.slice(2)
if (args.includes('--list')) {
  console.log(Object.keys(CHECKS).join('\n'))
  process.exit(0)
}

const offlineFlag = args.indexOf('--offline-presets')
const env = repoEnv(process.cwd(), { offlinePresets: offlineFlag === -1 ? null : args[offlineFlag + 1] })

if (args.includes('--dump-headers')) {
  const e = await internals.emission(env)
  for (const row of e.rows) console.log(`${row.header.padEnd(40)} ${row.cell.manager} ${row.cell.packageFile} ${row.cell.depName ?? '(lock file)'} ${row.updateType}`)
  for (const refusal of e.refusals) console.error(`refused: ${refusal}`)
  process.exit(e.refusals.length ? 1 : 0)
}

const name = args.find((a, i) => !a.startsWith('--') && !(offlineFlag !== -1 && i === offlineFlag + 1))
if (!CHECKS[name]) {
  console.error(`unknown check '${name ?? ''}'; expected one of: ${Object.keys(CHECKS).join(', ')}`)
  process.exit(2)
}

const { fail, note } = await CHECKS[name](env)
for (const n of note) console.log(`  ${n}`)
if (fail.length) {
  console.error(`\n${name} FAILED:`)
  for (const f of fail) console.error(`  - ${f}`)
  process.exit(1)
}
console.log(`\n${name}: passed`)
