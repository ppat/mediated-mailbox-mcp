// The half of the `pr-labels` workflow that makes no API call. It reads a pull request's diff,
// works out which `component:` labels the diff calls for, and what has to change on the pull
// request to carry exactly those.
//
// The component table in CLAUDE.md is the one home for the list of components, so it is read from
// there on every run rather than copied here. A new component row is all this needs to label a new
// directory. The parse refuses to guess. A table it cannot read fails the run, because a parse that
// quietly found fewer components would strip labels from every pull request it ran on.

import { execFileSync } from 'node:child_process'

export const LABEL_PREFIX = 'component:'
export const LABEL_COLOR = '5319E7'

const SECTION_HEADING = '### Components'
// A component row's first cell is the directory in backticks with its trailing slash, `core/` or
// `packaging/chart/`. The label drops the slash and keeps every other segment.
const DIRECTORY_CELL = /^`((?:[a-z0-9][a-z0-9-]*\/)+)`$/

export function parseComponents(claudeMd) {
  const lines = claudeMd.split('\n')
  const start = lines.findIndex((l) => l.trim() === SECTION_HEADING)
  if (start === -1) throw new Error(`CLAUDE.md has no '${SECTION_HEADING}' heading`)

  const rows = []
  let inTable = false
  for (const line of lines.slice(start + 1)) {
    if (/^#{1,3} /.test(line)) break
    if (line.startsWith('|')) {
      inTable = true
      rows.push(line)
    } else if (inTable) {
      break
    }
  }
  // The header row and the separator row come first.
  if (rows.length < 3) throw new Error(`the table under '${SECTION_HEADING}' in CLAUDE.md has no component rows`)
  const header = cells(rows[0])
  if (header[0] !== 'Directory') throw new Error(`the table under '${SECTION_HEADING}' in CLAUDE.md no longer starts with a Directory column: ${rows[0]}`)

  const directories = rows.slice(2).map((row) => {
    const match = DIRECTORY_CELL.exec(cells(row)[0])
    if (!match) throw new Error(`cannot read a component directory from this row of CLAUDE.md: ${row}`)
    return match[1]
  })
  const duplicate = directories.find((d, i) => directories.indexOf(d) !== i)
  if (duplicate) throw new Error(`CLAUDE.md lists the component directory ${duplicate} twice`)
  return directories
}

function cells(row) {
  return row.split('|').slice(1, -1).map((c) => c.trim())
}

// The paths the merge-base diff from `base` to `head` touches, the set GitHub shows as a pull
// request's changed files when `base` is the base branch as it stands now. An older base commit
// would count the paths of base commits the head has since merged. Renames are split into a
// deletion and an addition, so a file moved out of a component still labels the component it left.
// Paths are read NUL-separated, because git quotes a path holding a quote, a backslash or a
// non-ASCII byte, and a quoted path matches no component directory.
export function changedPaths(base, head, cwd = process.cwd()) {
  return execFileSync('git', ['diff', '-z', '--no-renames', '--name-only', `${base}...${head}`], { cwd, encoding: 'utf8' })
    .split('\0').filter(Boolean)
}

// Every component whose directory holds at least one of the paths, as label names. A path outside
// every component directory, such as a root file or a document under docs/, calls for no label.
export function wantedLabels(paths, directories) {
  const wanted = new Set()
  for (const path of paths) {
    for (const directory of directories) {
      if (path.startsWith(directory)) wanted.add(LABEL_PREFIX + directory.slice(0, -1))
    }
  }
  return [...wanted].sort()
}

// What turns the labels the pull request carries into exactly the wanted component labels. Labels
// outside the component prefix, the unit label among them, are never touched.
export function labelChanges(current, wanted) {
  const have = current.filter((l) => l.startsWith(LABEL_PREFIX))
  return {
    add: wanted.filter((l) => !have.includes(l)),
    remove: have.filter((l) => !wanted.includes(l)).sort(),
  }
}
