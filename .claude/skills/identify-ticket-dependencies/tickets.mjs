// Reads the epics and their tickets from GitHub, maps each ticket's State and dependencies, keeps
// that map in a local JSON file, lists the tickets eligible to launch and keeps the epics' State
// cells in step with what the map derives.
//
// No dependencies. Every GitHub read goes through `gh api graphql`, batched with aliases, so a full
// read of every epic and ticket costs a handful of calls. Output is one line per item and nothing
// when there is nothing, and `--json` prints the full data instead.
//
//   bun tickets.mjs refresh --state FILE              read GitHub, write the map, print a summary
//   bun tickets.mjs diff --state FILE [--write]       read GitHub, print what changed against FILE
//   bun tickets.mjs show --state FILE [N ...]         print the map from FILE, no GitHub read
//   bun tickets.mjs deps N [N ...]                    read the named tickets and print their blockers
//   bun tickets.mjs candidates [--busy N,N]           list the tickets eligible to launch, by number
//   bun tickets.mjs sync [--set N=STATE ...] [--apply]  print, and with --apply write, State cells
//
// Options: --repo OWNER/NAME (default ppat/mediated-mailbox-mcp), --roadmap PATH (default
// ROADMAP.md, read for the units it marks delivered, the value path and the dependency table).
//
// A ticket body must carry the ticket template's header line and its `Blocked by` row, whose cell
// is `none` or a list of `#N`, `[#N](url)` or unit identifiers joined by commas and `and`, each
// with an optional parenthesised note. An open ticket that breaks this is named on standard error,
// mapped with no dependencies, and makes the command exit 2. Closed tickets block nothing, so
// their bodies are not checked.

import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, renameSync, writeFileSync } from 'node:fs'

const ROADMAP_ISSUE = 118
const PAGE = 100

const CLOSER = `closedByPullRequestsReferences(first: 10, includeClosedPrs: true) { nodes { number merged state } }
timelineItems(itemTypes: [CLOSED_EVENT], last: 1) {
  nodes { ... on ClosedEvent { closer { __typename ... on PullRequest { merged } } } }
}`
const ISSUE_FIELDS = `number title state stateReason body repository { nameWithOwner } ${CLOSER}`

function gh(args, input) {
  try {
    return execFileSync('gh', args, { encoding: 'utf8', input, maxBuffer: 64 * 1024 * 1024 })
  } catch (err) {
    throw new Error(`gh ${args[0]} ${args[1]} failed: ${err.stderr || err.message}`)
  }
}

function graphql(repo, fields) {
  const [owner, name] = repo.split('/')
  const query = `query { repository(owner: "${owner}", name: "${name}") { ${fields} } }`
  const data = JSON.parse(gh(['api', 'graphql', '-f', `query=${query}`]))
  if (data.errors) throw new Error(`gh api graphql errors: ${JSON.stringify(data.errors)}`)
  return data.data.repository
}

// Closed by a merged pull request, or by a commit on the default branch, which a squash merge is.
function closedByMerge(node) {
  if ((node.closedByPullRequestsReferences?.nodes ?? []).some((pr) => pr.merged)) return true
  const items = node.timelineItems?.nodes ?? []
  const closer = items.length ? items[items.length - 1].closer : null
  return Boolean(closer && (closer.__typename === 'Commit' || closer.merged))
}

// Ticket bodies.

const LINK = /\[(#\d+)\]\(https:\/\/github\.com\/[\w.-]+\/[\w.-]+\/(?:issues|pull)\/\d+\)/g
const REF = /^#(\d+)$/
const UNIT = /^[A-Z]\d+$/

export function parseBlockedBy(cell) {
  let text = cell.trim()
  if (/^none\.?$/i.test(text)) return []
  text = text.replace(LINK, '$1')
  for (let prev = null; prev !== text; ) {
    prev = text
    text = text.replace(/\s*\([^()]*\)/g, '')
  }
  const parts = text
    .split(/,\s*(?:and\s+)?|\s+and\s+/)
    .map((p) => p.trim())
    .filter(Boolean)
  if (!parts.length) throw new Error('empty Blocked by cell')
  return parts.map((p) => {
    const ref = REF.exec(p)
    if (ref) return { kind: 'ref', number: Number(ref[1]) }
    if (UNIT.test(p)) return { kind: 'unit', unit: p }
    throw new Error(`Blocked by item is not a reference or a unit: ${JSON.stringify(p)}`)
  })
}

// The increment or point a header line names, as `V4` or `P1`.
function headerPlace(body) {
  const m = /\*\*Value increment(?: or production point)?:\*\*\s*\[([^\]]+)\]/.exec(body)
  return m ? placeOf(m[1]) : null
}

const placeOf = (text) => {
  const v = /^V(\d+)\b/.exec(text)
  if (v) return `V${v[1]}`
  const p = /^Production point (\d+)\b/i.exec(text)
  return p ? `P${p[1]}` : null
}

function parseTicketBody(body) {
  if (!/^\*\*Unit:\*\*/m.test(body)) throw new Error('no header line starting **Unit:**')
  const unit = /^\*\*Unit:\*\*\s*\[([A-Z]\d+)/m.exec(body)
  const row = /^\|\s*Blocked by\s*\|(.*)\|\s*$/m.exec(body)
  if (!row) throw new Error('no Blocked by row')
  return { unit: unit ? unit[1] : null, blockedBy: parseBlockedBy(row[1]) }
}

// An epic's table, | Unit | Ticket | What lands | State |, keyed by `#N` or `owner/repo#N`, with
// each row's position and the body line it sits on, so a State cell can be rewritten alone.
function parseEpicTable(body) {
  const rows = {}
  let unit = null
  let inTable = false
  let position = 0
  body.split('\n').forEach((raw, line) => {
    const m = /^\|(.*)\|\s*$/.exec(raw.trim())
    if (!m) {
      inTable = false
      return
    }
    const cells = m[1].split('|').map((c) => c.trim())
    if (cells[0] === 'Unit' && cells[1] === 'Ticket') {
      inTable = true
      return
    }
    if (!inTable || cells.length < 4 || /^[-:\s]*$/.test(cells.join(''))) return
    if (cells[0]) unit = cells[0].replace(/\*\*/g, '').split(' — ')[0].trim()
    const t = /(?:([\w.-]+\/[\w.-]+))?#(\d+)/.exec(cells[1])
    if (t) rows[`${t[1] ?? ''}#${t[2]}`] = { unit, tableState: cells[3], position: ++position, line }
  })
  return rows
}

// ROADMAP.md.

function readRoadmap(path) {
  if (!path || !existsSync(path)) return null
  const text = readFileSync(path, 'utf8')
  const delivered = new Set([...text.matchAll(/^\s*- \[x\] \*\*([A-Z]\d+)\b/gm)].map((m) => m[1]))
  return { delivered, path: valuePath(text), edges: dependencyEdges(text) }
}

// The value path in order, each increment followed by the production point its section names.
function valuePath(text) {
  const start = text.indexOf('## The value path')
  const end = text.indexOf('\n## ', start + 1)
  const sections = text.slice(start, end).split(/^### /m).slice(1)
  const path = []
  for (const s of sections) {
    const id = /^V(\d+)\b/.exec(s)
    if (!id) continue
    path.push(`V${id[1]}`)
    const point = /then\s+\[production point (\d+)\]/i.exec(s.replace(/\s+/g, ' '))
    if (point) path.push(`P${point[1]}`)
  }
  return path
}

// The dependency table's edges. An end is a unit, a unit's later work, or every unit.
function dependencyEdges(text) {
  const start = text.indexOf('## Dependencies')
  const end = text.indexOf('\n## ', start + 1)
  const edges = []
  for (const line of text.slice(start, end).split('\n')) {
    const cell = /^\|([^|]*)\|/.exec(line)
    if (!cell || !cell[1].includes('→')) continue
    for (const part of cell[1].split(/,\s*(?:and\s+)?/)) {
      const [from, to] = part.split('→').map((s) => s.trim())
      if (from && to) edges.push({ from: edgeEnd(from), to: edgeEnd(to) })
    }
  }
  return edges
}

function edgeEnd(text) {
  if (/^every\b/i.test(text)) return { unit: '*', later: false }
  const m = /^([A-Z]\d+)/.exec(text)
  return { unit: m ? m[1] : null, later: /later work/.test(text) }
}

// The two disagreements between a ticket's Blocked by line and the dependency table that the
// launch-ticket skill's launch step names.
function tableDisagreements(t, byNumber, roadmap) {
  if (!roadmap || !t.unit || !t.blockedBy) return []
  const blockerUnits = new Set(
    t.blockedBy
      // A pull request maps to its unit through the ticket it closes.
      .flatMap((b) => (b.kind === 'unit' ? [b.unit] : [b.number, ...(b.closes ?? [])].map((n) => byNumber.get(n)?.unit)))
      .filter(Boolean),
  )
  const out = []
  for (const e of roadmap.edges) {
    if (e.to.unit !== t.unit || e.to.later || e.from.later || !e.from.unit || e.from.unit === '*') continue
    if (!roadmap.delivered.has(e.from.unit) && !blockerUnits.has(e.from.unit)) {
      out.push(`table edge ${e.from.unit} → ${t.unit} has no blocker`)
    }
  }
  for (const u of blockerUnits) {
    if (u === t.unit) continue
    const edge = roadmap.edges.some((e) => e.from.unit === u && (e.to.unit === t.unit || e.to.unit === '*'))
    if (!edge) out.push(`blocker unit ${u} has no table edge into ${t.unit}`)
  }
  return out
}

// Reading GitHub.

function addTicket(tickets, node, epic, rows, repo, errors) {
  const other = node.repository.nameWithOwner
  const key = other === repo ? `#${node.number}` : `${other}#${node.number}`
  const row = rows[key]
  const t = {
    number: node.number,
    repo: other,
    title: node.title,
    epic,
    position: row ? row.position : null,
    issueState: node.state,
    stateReason: node.stateReason ?? null,
    built: node.state === 'CLOSED' && closedByMerge(node),
    openPullRequests: (node.closedByPullRequestsReferences?.nodes ?? [])
      .filter((pr) => pr.state === 'OPEN')
      .map((pr) => pr.number),
    tableState: row ? row.tableState : null,
    unit: row ? row.unit : null,
    place: other === repo ? headerPlace(node.body ?? '') : null,
    blockedBy: null,
    formatError: null,
  }
  if (other === repo && node.state === 'OPEN') {
    try {
      const parsed = parseTicketBody(node.body ?? '')
      t.blockedBy = parsed.blockedBy
      t.unit = t.unit ?? parsed.unit
    } catch (err) {
      t.formatError = err.message
      errors.push(`#${node.number}: ${err.message}`)
    }
  }
  tickets[key] = t
}

// Marks each blocker built or not, reading references outside the map from GitHub in one call.
function resolveBlockers(tickets, repo) {
  const known = new Map(
    Object.entries(tickets)
      .filter(([k]) => k.startsWith('#'))
      .map(([, t]) => [t.number, t]),
  )
  const wanted = [
    ...new Set(
      Object.values(tickets).flatMap((t) => (t.blockedBy ?? []).filter((b) => b.kind === 'ref').map((b) => b.number)),
    ),
  ].filter((n) => !known.has(n))
  const extra = new Map()
  const closes = new Map()
  if (wanted.length) {
    const d = graphql(
      repo,
      wanted
        .map(
          (n) =>
            `r${n}: issueOrPullRequest(number: ${n}) { __typename ... on PullRequest { merged closingIssuesReferences(first: 5) { nodes { number } } } ... on Issue { state ${CLOSER} } }`,
        )
        .join(' '),
    )
    for (const n of wanted) {
      const node = d[`r${n}`]
      if (!node) extra.set(n, false)
      else if (node.__typename === 'PullRequest') {
        extra.set(n, Boolean(node.merged))
        closes.set(n, node.closingIssuesReferences.nodes.map((i) => i.number))
      }
      else extra.set(n, node.state === 'CLOSED' && closedByMerge(node))
    }
  }
  for (const t of Object.values(tickets)) {
    for (const b of t.blockedBy ?? []) {
      if (b.kind !== 'ref') continue
      b.built = known.has(b.number) ? known.get(b.number).built : extra.get(b.number)
      if (closes.has(b.number)) b.closes = closes.get(b.number)
    }
  }
}

// Open issues that look like tickets, a header line or a `unit:` label, wherever they sit.
function openTicketLikeIssues(repo) {
  const found = []
  for (let cur = null; ; ) {
    const d = graphql(
      repo,
      `issues(states: OPEN, first: ${PAGE}${cur ? `, after: "${cur}"` : ''}) { pageInfo { hasNextPage endCursor } nodes { number body labels(first: 20) { nodes { name } } parent { number } } }`,
    )
    for (const n of d.issues.nodes) {
      const looks = /^\*\*Unit:\*\*/m.test(n.body ?? '') || n.labels.nodes.some((l) => l.name.startsWith('unit:'))
      if (looks) found.push({ number: n.number, parent: n.parent?.number ?? null })
    }
    if (!d.issues.pageInfo.hasNextPage) return found
    cur = d.issues.pageInfo.endCursor
  }
}

function fetchAll(repo) {
  const errors = []
  const epics = {}
  const tickets = {}
  const top = graphql(
    repo,
    `issue(number: ${ROADMAP_ISSUE}) { subIssues(first: ${PAGE}) { nodes { number title state body } } }`,
  )
  for (const e of top.issue.subIssues.nodes) {
    epics[e.number] = { title: e.title, state: e.state, place: placeOf(e.title), body: e.body ?? '', rows: parseEpicTable(e.body ?? '') }
  }
  const seen = Object.fromEntries(Object.keys(epics).map((n) => [n, new Set()]))
  let cursors = Object.fromEntries(Object.keys(epics).map((n) => [n, null]))
  while (Object.keys(cursors).length) {
    const fields = Object.entries(cursors)
      .map(
        ([n, cur]) =>
          `e${n}: issue(number: ${n}) { subIssues(first: ${PAGE}${cur ? `, after: "${cur}"` : ''}) { pageInfo { hasNextPage endCursor } nodes { ${ISSUE_FIELDS} } } }`,
      )
      .join(' ')
    const d = graphql(repo, fields)
    const next = {}
    for (const n of Object.keys(cursors)) {
      const si = d[`e${n}`].subIssues
      for (const node of si.nodes) {
        addTicket(tickets, node, Number(n), epics[n].rows, repo, errors)
        seen[n].add(node.repository.nameWithOwner === repo ? `#${node.number}` : `${node.repository.nameWithOwner}#${node.number}`)
      }
      if (si.pageInfo.hasNextPage) next[n] = si.pageInfo.endCursor
    }
    cursors = next
  }
  resolveBlockers(tickets, repo)

  const structure = []
  for (const [n, e] of Object.entries(epics)) {
    for (const k of seen[n]) if (!e.rows[k]) structure.push(`no row: ${k} is a sub-issue of epic #${n} with no row in its table`)
    for (const k of Object.keys(e.rows)) if (!seen[n].has(k)) structure.push(`no sub-issue: epic #${n} has a row for ${k}, which is not its sub-issue`)
  }
  const epicNumbers = new Set([ROADMAP_ISSUE, ...Object.keys(epics).map(Number)])
  for (const i of openTicketLikeIssues(repo)) {
    if (epicNumbers.has(i.number)) continue
    if (!epicNumbers.has(i.parent)) structure.push(`under no epic: open ticket #${i.number}`)
  }
  for (const [k, t] of Object.entries(tickets)) {
    const place = epics[t.epic]?.place
    if (t.issueState === 'OPEN' && t.place && place && t.place !== place) {
      structure.push(`misplaced: ${k}'s header names ${t.place}, its epic #${t.epic} is ${place}`)
    }
  }
  return { epics, tickets, errors, structure }
}

function fetchSome(repo, numbers) {
  const errors = []
  const tickets = {}
  const d = graphql(repo, numbers.map((n) => `t${n}: issue(number: ${n}) { ${ISSUE_FIELDS} parent { number } }`).join(' '))
  for (const n of numbers) {
    const node = d[`t${n}`]
    if (!node) {
      errors.push(`#${n}: no such issue`)
      continue
    }
    addTicket(tickets, node, node.parent?.number ?? null, {}, repo, errors)
  }
  resolveBlockers(tickets, repo)
  return { tickets, errors }
}

// The State control's rules give a ticket, apart from in-progress and hold, which need the roster.
function derive(t, delivered) {
  if (t.issueState === 'CLOSED') {
    if (t.built) return 'built'
    // Closed as a duplicate or as not planned is a deliberate close, not an anomaly to report.
    return ['DUPLICATE', 'NOT_PLANNED'].includes(t.stateReason) ? 'closed-not-built' : 'closed-unmerged'
  }
  if (t.formatError) return 'format-error'
  if (t.blockedBy === null) return null
  for (const b of t.blockedBy) {
    if (b.kind === 'unit') {
      if (delivered === null) return 'unknown-unit'
      if (!delivered.has(b.unit)) return 'blocked'
    } else if (!b.built) return 'blocked'
  }
  return 'pending'
}

function snapshot(o) {
  const { epics, tickets, errors, structure } = fetchAll(o.repo)
  const roadmap = readRoadmap(o.roadmap)
  for (const t of Object.values(tickets)) t.derived = derive(t, roadmap?.delivered ?? null)
  const bodies = {}
  for (const [n, e] of Object.entries(epics)) {
    bodies[n] = { body: e.body, rows: e.rows }
    delete e.body
    delete e.rows
  }
  return {
    state: { fetchedAt: new Date().toISOString(), repo: o.repo, roadmapIssue: ROADMAP_ISSUE, epics, tickets },
    bodies,
    roadmap,
    errors,
    structure,
  }
}

// Output.

// The issue table's State disagrees with the derived one. In-progress and hold need the roster.
const differs = (t) =>
  t.derived !== null && t.derived !== 'closed-not-built' && ![t.derived, 'in-progress', 'hold'].includes(t.tableState)

const waitsOn = (t) =>
  (t.blockedBy ?? [])
    .filter((b) => b.kind === 'unit' || !b.built)
    .map((b) => (b.kind === 'ref' ? `#${b.number}` : b.unit))
    .join(', ')

const line = (k, t) =>
  `${k.padEnd(8)} epic #${t.epic}  ${(t.unit ?? '-').padEnd(4)} ${(t.tableState ?? '(no row)').padEnd(11)} derived ${(t.derived ?? '-').padEnd(14)}${differs(t) ? ' !' : ''}${t.derived === 'blocked' ? `  waits on ${waitsOn(t)}` : ''}`

const byKey = (a, b) => a.length - b.length || a.localeCompare(b)

function write(path, state) {
  writeFileSync(`${path}.tmp`, JSON.stringify(state, null, 1))
  renameSync(`${path}.tmp`, path)
}

function finish(o, errors, structure, json) {
  if (o.json) console.log(JSON.stringify(json, null, 1))
  for (const s of structure ?? []) if (!o.json) console.log(`STRUCTURE ${s}`)
  for (const e of errors) console.error(`FORMAT ${e}`)
  return errors.length ? 2 : 0
}

// Commands.

function refresh(o) {
  const { state, errors, structure } = snapshot(o)
  write(o.state, state)
  if (!o.json) {
    const counts = {}
    for (const t of Object.values(state.tickets)) counts[t.derived] = (counts[t.derived] ?? 0) + 1
    console.log(`${Object.keys(state.tickets).length} tickets in ${Object.keys(state.epics).length} epics: ${Object.entries(counts).map(([k, v]) => `${k} ${v}`).join(', ')}`)
    const mism = Object.keys(state.tickets).filter((k) => differs(state.tickets[k]))
    if (mism.length) console.log(`table State differs from derived: ${mism.join(' ')}`)
  }
  return finish(o, errors, structure, { state, structure, errors })
}

const FIELDS = ['epic', 'issueState', 'stateReason', 'built', 'tableState', 'blockedBy', 'derived', 'formatError', 'openPullRequests']

function diff(o) {
  // A first run has no map yet, so everything on GitHub reads as added.
  const old = existsSync(o.state)
    ? JSON.parse(readFileSync(o.state, 'utf8'))
    : { fetchedAt: 'no earlier map', epics: {}, tickets: {} }
  const { state: now, errors, structure } = snapshot(o)
  const changes = []
  for (const k of [...new Set([...Object.keys(old.tickets), ...Object.keys(now.tickets)])].sort(byKey)) {
    const a = old.tickets[k]
    const b = now.tickets[k]
    if (!a) changes.push(`+ ${line(k, b)}`)
    else if (!b) changes.push(`- ${k} no longer under an epic`)
    else {
      const ds = FIELDS.filter((f) => JSON.stringify(a[f]) !== JSON.stringify(b[f])).map(
        (f) => `${f} ${JSON.stringify(a[f])} -> ${JSON.stringify(b[f])}`,
      )
      if (!ds.length) continue
      const tag =
        a.derived === 'blocked' && b.derived === 'pending'
          ? 'UNBLOCKED '
          : a.derived === 'pending' && b.derived === 'blocked'
            ? 'BLOCKED '
            : ''
      changes.push(`~ ${k} ${tag}${ds.join('; ')}`)
    }
  }
  for (const k of new Set([...Object.keys(old.epics), ...Object.keys(now.epics)])) {
    if (JSON.stringify(old.epics[k]) !== JSON.stringify(now.epics[k])) {
      changes.push(`epic #${k}: ${JSON.stringify(old.epics[k])} -> ${JSON.stringify(now.epics[k])}`)
    }
  }
  if (!o.json) for (const c of changes) console.log(c)
  if (o.write) write(o.state, now)
  return finish(o, errors, structure, { since: old.fetchedAt, changes, structure, errors })
}

function show(o) {
  const state = JSON.parse(readFileSync(o.state, 'utf8'))
  const keys = o.numbers.length ? o.numbers.map((n) => `#${n.replace(/^#/, '')}`) : Object.keys(state.tickets).sort(byKey)
  if (o.json) console.log(JSON.stringify(keys.map((k) => state.tickets[k] ?? null), null, 1))
  else for (const k of keys) console.log(state.tickets[k] ? line(k, state.tickets[k]) : `${k} not in the map`)
  return 0
}

function deps(o) {
  const { tickets, errors } = fetchSome(o.repo, o.numbers.map((n) => Number(n.replace(/^#/, ''))))
  const delivered = readRoadmap(o.roadmap)?.delivered ?? null
  for (const t of Object.values(tickets)) t.derived = derive(t, delivered)
  if (!o.json) {
    for (const [k, t] of Object.entries(tickets)) {
      const blockers =
        (t.blockedBy ?? [])
          .map((b) =>
            b.kind === 'ref'
              ? `#${b.number} ${b.built ? 'built' : 'not built'}`
              : `${b.unit} ${delivered?.has(b.unit) ? 'delivered' : 'not delivered'}`,
          )
          .join(', ') || (t.blockedBy === null ? 'not read, the ticket is closed or in another repository' : 'none')
      console.log(`${k} ${t.issueState.toLowerCase()}, epic #${t.epic}, derived ${t.derived}, blocked by: ${blockers}`)
    }
  }
  return finish(o, errors, [], { tickets, errors })
}

// The epics the production point gate admits. The value path in ROADMAP.md gives the order, and a
// point is crossed when its issue, found by its title among the roadmap ticket's sub-issues, is
// closed. Admitted are the increments before the next point not yet crossed, and that point's own
// issue once every one of its increments' issues is closed.
function admittedEpics(state, roadmap) {
  const byPlace = new Map(Object.entries(state.epics).map(([n, e]) => [e.place, { number: Number(n), ...e }]))
  const admitted = new Map()
  let sincePoint = []
  for (const [i, place] of roadmap.path.entries()) {
    const e = byPlace.get(place)
    if (place.startsWith('V')) {
      if (e) admitted.set(e.number, i + 1)
      sincePoint.push(e)
      continue
    }
    if (e && e.state === 'CLOSED') {
      sincePoint = []
      continue
    }
    if (e && sincePoint.every((x) => x && x.state === 'CLOSED')) admitted.set(e.number, i + 1)
    return admitted
  }
  return admitted
}

function candidates(o) {
  const { state, roadmap, errors } = snapshot(o)
  if (!roadmap) throw new Error(`candidates needs ROADMAP.md at ${o.roadmap}`)
  const admitted = admittedEpics(state, roadmap)
  const busy = new Set(o.busy)
  const byNumber = new Map(
    Object.entries(state.tickets)
      .filter(([k]) => k.startsWith('#'))
      .map(([, t]) => [t.number, t]),
  )
  const dependents = new Map()
  for (const t of byNumber.values()) {
    if (t.issueState !== 'OPEN') continue
    for (const b of t.blockedBy ?? []) if (b.kind === 'ref') dependents.set(b.number, [...(dependents.get(b.number) ?? []), t.number])
  }
  const out = []
  for (const t of byNumber.values()) {
    if (!admitted.has(t.epic) || t.repo !== o.repo || t.issueState !== 'OPEN') continue
    if (t.tableState !== 'pending' || t.derived !== 'pending' || t.openPullRequests.length || busy.has(t.number)) continue
    out.push({
      number: t.number,
      title: t.title,
      unit: t.unit,
      epic: t.epic,
      place: state.epics[t.epic].place,
      epicPosition: admitted.get(t.epic),
      row: t.position,
      blockedBy: (t.blockedBy ?? []).map((b) => (b.kind === 'ref' ? `#${b.number}` : b.unit)),
      dependents: (dependents.get(t.number) ?? []).map((n) => `#${n}`),
      disagreements: tableDisagreements(t, byNumber, roadmap),
    })
  }
  // By number, so the order the tickets launch in is decided only by the launch-ticket skill.
  out.sort((a, b) => a.number - b.number)
  if (!o.json) {
    for (const c of out) {
      const title = c.title.length > 60 ? `${c.title.slice(0, 57)}...` : c.title
      console.log(
        `#${c.number} ${c.unit ?? '-'} ${c.place}#${c.epic} path ${c.epicPosition} row ${c.row} | ${title} | blocked by ${c.blockedBy.join(' ') || 'none'} | dependents ${c.dependents.join(' ') || 'none'}${c.disagreements.length ? ` | ${c.disagreements.join('; ')}` : ''}`,
      )
    }
  }
  return finish(o, errors, [], { candidates: out, errors })
}

// The State cells that differ from the derived State, and the ones --set names. A hold or an
// in-progress row stays as it is while its ticket is open, unless --set names it. `pending` in
// --set lifts a hold, writing the derived State.
function sync(o) {
  const { state, bodies, errors } = snapshot(o)
  const sets = new Map(
    o.set.map((s) => {
      const m = /^#?(\d+)=(in-progress|hold|pending)$/.exec(s)
      if (!m) throw new Error(`--set takes N=in-progress|hold|pending, not ${s}`)
      return [`#${m[1]}`, m[2]]
    }),
  )
  const changes = []
  for (const [k, t] of Object.entries(state.tickets)) {
    const row = bodies[t.epic]?.rows[k]
    if (!row) continue
    let want = null
    const set = sets.get(k)
    if (set === 'pending') want = t.derived
    else if (set) want = set
    else if (t.derived === 'built') want = 'built'
    else if (['pending', 'blocked'].includes(t.derived) && !['hold', 'in-progress'].includes(row.tableState)) want = t.derived
    if (want && want !== row.tableState) changes.push({ key: k, epic: t.epic, line: row.line, from: row.tableState, to: want })
  }
  for (const k of sets.keys()) {
    if (!changes.some((c) => c.key === k) && !Object.values(bodies).some((b) => b.rows[k])) throw new Error(`--set ${k}: no row in any epic`)
  }
  const edited = {}
  for (const c of changes) {
    const lines = (edited[c.epic] ??= bodies[c.epic].body.split('\n'))
    const cells = lines[c.line].split('|')
    // A row is | unit | ticket | what lands | state |, so the State cell is the fifth piece.
    cells[4] = ` ${c.to} `
    lines[c.line] = cells.join('|')
  }
  if (!o.json) {
    for (const c of changes) console.log(`epic #${c.epic} ${c.key} ${c.from} -> ${c.to}`)
    if (o.diff) {
      for (const c of changes) {
        console.log(`--- epic #${c.epic} line ${c.line + 1}\n- ${bodies[c.epic].body.split('\n')[c.line]}\n+ ${edited[c.epic][c.line]}`)
      }
    }
  }
  if (o.apply) {
    const [owner, name] = o.repo.split('/')
    for (const [n, lines] of Object.entries(edited)) {
      // The body travels as JSON on standard input, never as a shell argument.
      gh(['api', '-X', 'PATCH', `repos/${owner}/${name}/issues/${n}`, '--input', '-'], JSON.stringify({ body: lines.join('\n') }))
    }
  }
  return finish(o, errors, [], { changes, applied: Boolean(o.apply), errors })
}

function parseArgs(argv) {
  const o = {
    repo: 'ppat/mediated-mailbox-mcp',
    roadmap: 'ROADMAP.md',
    state: null,
    write: false,
    json: false,
    apply: false,
    diff: false,
    busy: [],
    set: [],
    numbers: [],
  }
  const [cmd, ...rest] = argv
  for (let i = 0; i < rest.length; i++) {
    const a = rest[i]
    if (a === '--repo') o.repo = rest[++i]
    else if (a === '--roadmap') o.roadmap = rest[++i]
    else if (a === '--state') o.state = rest[++i]
    else if (a === '--write') o.write = true
    else if (a === '--json') o.json = true
    else if (a === '--apply') o.apply = true
    else if (a === '--diff') o.diff = true
    else if (a === '--busy') o.busy = rest[++i].split(',').map((n) => Number(n.replace(/^#/, '')))
    else if (a === '--set') o.set.push(rest[++i])
    else o.numbers.push(a)
  }
  const commands = { refresh, diff, show, deps, candidates, sync }
  if (!commands[cmd]) throw new Error('usage: tickets.mjs refresh|diff|show|deps|candidates|sync (see the header comment)')
  if (['refresh', 'diff', 'show'].includes(cmd) && !o.state) throw new Error(`${cmd} needs --state FILE`)
  if (cmd === 'deps' && !o.numbers.length) throw new Error('deps needs at least one ticket number')
  return { run: commands[cmd], o }
}

if (import.meta.main ?? process.argv[1]?.endsWith('tickets.mjs')) {
  try {
    const { run, o } = parseArgs(process.argv.slice(2))
    process.exit(run(o))
  } catch (err) {
    console.error(err.message)
    process.exit(1)
  }
}
