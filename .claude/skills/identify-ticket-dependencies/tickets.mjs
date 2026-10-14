// Reads and manages the States and dependencies of the project's tickets, and the epics' tables
// that list them. Run it with --help for the commands, options and output.
//
// The data. An epic is the roadmap ticket (#118), an increment issue or a production point issue.
// Each epic's table has one row per ticket, with the ticket's unit and its State. A ticket's blockers
// are named in the Blocked by row of its own body. The map is a local JSON file that holds every
// ticket with its epic, its State in the epic's table, its blockers, and the State those blockers
// give it (stored as `derived`).
//
// Every GitHub read goes through `gh api graphql`, batched with aliases, so reading every epic and
// ticket takes a handful of calls. Every write replaces an issue body through `gh api`, after saving
// the old body to BACKUP_DIR.
//
// The file is in this order: GitHub access, parsing ticket bodies and epic tables, reading
// ROADMAP.md, reading epics and tickets, the State a ticket's blockers give, output, the commands
// that read, the commands that write, the command line.

import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs'

const USAGE = `Usage: bun tickets.mjs COMMAND [ARGUMENTS] [OPTIONS]

Reads the tickets and epics of a repository from GitHub, and keeps their States and the epics'
tables correct. Reading takes a few GitHub calls however many tickets there are.

Words used below
  State    The State cell of a ticket's row in its epic's table: pending, blocked, in-progress,
           hold or built.
  blockers What a ticket's Blocked by row names: tickets, pull requests and units.
  map      A local file with every ticket's epic, State and blockers.

Commands that read
  refresh --map FILE
      Reads GitHub and saves the map to FILE. Prints how many tickets have each State their
      blockers give, and the tickets whose State in the epic's table differs from it.
  diff --map FILE [--save]
      Reads GitHub and prints what changed since FILE was saved: tickets added, removed or changed,
      marking a ticket that became pending UNBLOCKED and one that became blocked BLOCKED. With no
      FILE it prints one line of counts. --save then saves the new map to FILE.
  show --map FILE [TICKET ...]
      Prints the tickets in FILE, or the named ones, without reading GitHub.
  deps TICKET [TICKET ...]
      Reads GitHub and prints each named ticket's blockers and whether each is built or delivered.
  candidates [--exclude TICKET,TICKET]
      Reads GitHub and prints the tickets that may be launched: open, in this repository, pending,
      with no open pull request, not in --exclude, and under an epic the production point gate
      admits. For each it prints its unit, its epic, the tickets waiting on it, and where its
      Blocked by row and ROADMAP.md's dependency table disagree.

Commands that write
  They print what they would change. They change GitHub only with --write.
  update-states [--show-lines] [--write]
      Sets every State that differs from the one the blockers and merged pull requests give. A row
      of an open ticket reading hold or in-progress is left alone. --show-lines also prints each row
      before and after.
  set-state TICKET in-progress|hold [--write]
      Sets one ticket's State to the one named.
  reset-state TICKET [--write]
      Sets one ticket's State to the one its Blocked by row gives, pending or blocked. This lifts a
      hold and clears in-progress.
  add-ticket TICKET --epic EPIC [--row-state STATE] [--unit-title TEXT] [--write]
      Adds the ticket's row to EPIC's table under its unit, and makes the ticket a sub-issue of EPIC
      unless it is one. A unit new to EPIC gets a new bold unit cell, titled from ROADMAP.md or
      --unit-title. The State is the one its blockers give, or --row-state. A ticket in another
      repository is OWNER/NAME#NUMBER, with the repository name as its unit. Refuses a closed EPIC,
      a ticket that has a row there, and a ticket under another epic, which move-ticket handles.
  move-ticket TICKET --epic EPIC [--write]
      Makes the ticket a sub-issue of EPIC in place of its current epic, and moves its row, with its
      words and State, from the old table to EPIC's.
  remove-row TICKET [--epic EPIC] [--unlink] [--write]
      Removes the ticket's row from its epic's table, or from EPIC's, which removes the row left by a
      ticket that is no longer a sub-issue of EPIC. --unlink also removes the sub-issue link.
  update-overview [--write]
      Corrects the roadmap ticket's overview table. For each epic it gives Status (open or closed for
      an increment, open or crossed for a point, not created for a point with no issue yet), Units
      and Tickets as built / total, leaving out tickets closed as a duplicate or as not planned. It
      adds a row for an epic with none and links a point's row to its issue once it exists. Units
      are the increment's own units in the order its ROADMAP.md section lists them, then the units
      of other increments as "and later work under". A point lists its units in table order.

Options
  --map FILE         The map file.
  --write            Change GitHub. Without it a command that writes only prints what it would change.
  --save             diff: save the new map to FILE after printing what changed.
  --json             Print the full data as JSON instead of tables.
  --repo OWNER/NAME  The repository. Default ppat/mediated-mailbox-mcp.
  --roadmap PATH     ROADMAP.md, read for the units marked delivered, the value path and the
                     dependency table. Default ROADMAP.md in the current directory.

Columns and values
  A State is one of pending, blocked, in-progress, hold or built. The State from blockers can also
  be one of these, which the epics' tables never hold:
    closed-not-built  The ticket was closed as a duplicate or as not planned.
    closed-unmerged   The ticket was closed, but not by a merged pull request.
    format-error      The ticket is open and its body breaks the format below.
    unknown-unit      A blocker is a unit and ROADMAP.md could not be read to say if it is delivered.
    -                 No State could be given, for a ticket in another repository.
  Tables of tickets (refresh, show, diff, deps):
    TICKET                The ticket, as #NUMBER or OWNER/NAME#NUMBER.
    EPIC                  The epic the ticket is a sub-issue of.
    UNIT                  The unit of the ticket's row in the epic's table.
    STATE IN EPIC         The State in that row, or (no row).
    STATE FROM BLOCKERS   The State its Blocked by row and merged pull requests give it.
    NOTE                  differs when the two States differ, and what a blocked ticket waits on.
    CHANGE, DETAIL        diff: added, removed, changed or epic, and what changed.
    ISSUE, BLOCKED BY     deps: open or closed, and each blocker with whether it is built.
  candidates:
    PLACE ON VALUE PATH   Where the ticket's epic comes in ROADMAP.md's value path, counting from 1.
    ROW IN EPIC           The ticket's row number in the epic's table.
    TITLE                 The ticket's title, cut to 60 characters.
    WAITING ON IT         The open tickets whose Blocked by row names this ticket.
    ROADMAP DISAGREES     Where the Blocked by row and ROADMAP.md's dependency table differ.
  Commands that write print EPIC, TICKET, OLD STATE and NEW STATE for a State, and EPIC, ACTION,
  TICKET and DETAIL for the other changes. update-overview names the changed cell in DETAIL.

Output
  A table prints a header line and one row per item, and nothing when there are no rows. A STRUCTURE
  table lists epics and tickets that disagree: a sub-issue with no row, a row with no sub-issue, an
  open ticket under no epic, a ticket whose header names another epic. A sub-issue closed as a
  duplicate or as not planned needs no row.

Backups
  Before a write replaces an issue body, the old body is saved to /tmp/tickets/OWNER/NAME/NUMBER
  and its path is printed. An earlier backup there is renamed with a timestamp. If the backup
  fails, nothing is written. To restore one:
    jq -Rs '{body: .}' FILE | gh api -X PATCH repos/OWNER/NAME/issues/NUMBER --input -

Ticket bodies
  An open ticket's body must have the ticket template's **Unit:** line and its Blocked by row. The
  row says none, or lists #N, [#N](url) and unit identifiers, joined by commas and "and", each with
  an optional note in parentheses. A body that breaks this is named on standard error as FORMAT,
  mapped with no blockers, and the command exits 2.

Exit codes
  0  Done, or help.
  1  Wrong arguments, an unknown command, or a failed gh call.
  2  Done, but an open ticket's body broke the format above.
`

const BACKUP_DIR = '/tmp/tickets'
const ROADMAP_TICKET = 118
const PAGE_SIZE = 100

const CLOSING_PULL_REQUESTS = `closedByPullRequestsReferences(first: 10, includeClosedPrs: true) { nodes { number merged state } }
timelineItems(itemTypes: [CLOSED_EVENT], last: 1) {
  nodes { ... on ClosedEvent { closer { __typename ... on PullRequest { merged } } } }
}`
const TICKET_FIELDS = `number title state stateReason body repository { nameWithOwner } ${CLOSING_PULL_REQUESTS}`

function runGh(args, input) {
  try {
    return execFileSync('gh', args, { encoding: 'utf8', input, maxBuffer: 64 * 1024 * 1024 })
  } catch (err) {
    throw new Error(`gh ${args[0]} ${args[1]} failed: ${err.stderr || err.message}`)
  }
}

function queryGitHub(repo, fields) {
  const [owner, name] = repo.split('/')
  const query = `query { repository(owner: "${owner}", name: "${name}") { ${fields} } }`
  const data = JSON.parse(runGh(['api', 'graphql', '-f', `query=${query}`]))
  if (data.errors) throw new Error(`gh api graphql errors: ${JSON.stringify(data.errors)}`)
  return data.data.repository
}

// Closed by a merged pull request, or by a commit on the default branch, which a squash merge is.
function closedByMergedPullRequest(node) {
  if ((node.closedByPullRequestsReferences?.nodes ?? []).some((pr) => pr.merged)) return true
  const items = node.timelineItems?.nodes ?? []
  const closer = items.length ? items[items.length - 1].closer : null
  return Boolean(closer && (closer.__typename === 'Commit' || closer.merged))
}

// Parsing ticket bodies and epic tables.

const MARKDOWN_TICKET_LINK = /\[(#\d+)\]\(https:\/\/github\.com\/[\w.-]+\/[\w.-]+\/(?:issues|pull)\/\d+\)/g
const TICKET_REFERENCE = /^#(\d+)$/
const UNIT_ID = /^[A-Z]\d+$/

export function parseBlockedBy(cell) {
  let text = cell.trim()
  if (/^none\.?$/i.test(text)) return []
  text = text.replace(MARKDOWN_TICKET_LINK, '$1')
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
    const ref = TICKET_REFERENCE.exec(p)
    if (ref) return { kind: 'ref', number: Number(ref[1]) }
    if (UNIT_ID.test(p)) return { kind: 'unit', unit: p }
    throw new Error(`Blocked by item is not a reference or a unit: ${JSON.stringify(p)}`)
  })
}

// The increment or point a header line names, as `V4` or `P1`.
function placeNamedInHeader(body) {
  const m = /\*\*Value increment(?: or production point)?:\*\*\s*\[([^\]]+)\]/.exec(body)
  return m ? placeFromTitle(m[1]) : null
}

const placeFromTitle = (text) => {
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
function parseEpicRows(body) {
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
    const ticket = /(?:([\w.-]+\/[\w.-]+))?#(\d+)/.exec(cells[1])
    if (ticket) rows[`${ticket[1] ?? ''}#${ticket[2]}`] = { unit, tableState: cells[3], position: ++position, line }
  })
  return rows
}

// Reading ROADMAP.md.

function readRoadmap(path) {
  if (!path || !existsSync(path)) return null
  const text = readFileSync(path, 'utf8')
  const delivered = new Set([...text.matchAll(/^\s*- \[x\] \*\*([A-Z]\d+)\b/gm)].map((m) => m[1]))
  return { delivered, path: readValuePath(text), edges: readDependencyEdges(text), unitTitles: readUnitTitles(text), homes: readUnitsOfEachIncrement(text) }
}

// Each unit's `ID — title`, the text of the bold cell that opens its group in an epic's table.
function readUnitTitles(text) {
  const titles = {}
  for (const m of text.matchAll(/^\s*- \[[ x]\] \*\*(([A-Z]\d+) — [^*]+)\*\*/gm)) titles[m[2]] = m[1]
  return titles
}

// The units each value increment's section lists, in the order it lists them.
function readUnitsOfEachIncrement(text) {
  const start = text.indexOf('## The value path')
  const end = text.indexOf('\n## ', start + 1)
  const homes = {}
  for (const sec of text.slice(start, end).split(/^### /m).slice(1)) {
    const id = /^V(\d+)\b/.exec(sec)
    if (!id) continue
    const list = sec.replace(/\s+/g, ' ').split('**Units:**')[1]?.split(/, then\b|, and the later work|\*\*Value shipped/)[0] ?? ''
    homes[`V${id[1]}`] = [...list.matchAll(/\[([A-Z]\d+)\]/g)].map((m) => m[1])
  }
  return homes
}

// The value path in order, each increment followed by the production point its section names.
function readValuePath(text) {
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
function readDependencyEdges(text) {
  const start = text.indexOf('## Dependencies')
  const end = text.indexOf('\n## ', start + 1)
  const edges = []
  for (const line of text.slice(start, end).split('\n')) {
    const cell = /^\|([^|]*)\|/.exec(line)
    if (!cell || !cell[1].includes('→')) continue
    for (const part of cell[1].split(/,\s*(?:and\s+)?/)) {
      const [from, to] = part.split('→').map((s) => s.trim())
      if (from && to) edges.push({ from: parseEdgeEnd(from), to: parseEdgeEnd(to) })
    }
  }
  return edges
}

function parseEdgeEnd(text) {
  if (/^every\b/i.test(text)) return { unit: '*', later: false }
  const m = /^([A-Z]\d+)/.exec(text)
  return { unit: m ? m[1] : null, later: /later work/.test(text) }
}

// The two disagreements between a ticket's Blocked by line and the dependency table that the
// launch-ticket skill's launch step names.
function findRoadmapDisagreements(ticket, byNumber, roadmap) {
  if (!roadmap || !ticket.unit || !ticket.blockedBy) return []
  const blockerUnits = new Set(
    ticket.blockedBy
      // A pull request maps to its unit through the ticket it closes.
      .flatMap((b) => (b.kind === 'unit' ? [b.unit] : [b.number, ...(b.closes ?? [])].map((n) => byNumber.get(n)?.unit)))
      .filter(Boolean),
  )
  const out = []
  for (const e of roadmap.edges) {
    if (e.to.unit !== ticket.unit || e.to.later || e.from.later || !e.from.unit || e.from.unit === '*') continue
    if (!roadmap.delivered.has(e.from.unit) && !blockerUnits.has(e.from.unit)) {
      out.push(`table edge ${e.from.unit} → ${ticket.unit} has no blocker`)
    }
  }
  for (const u of blockerUnits) {
    if (u === ticket.unit) continue
    const edge = roadmap.edges.some((e) => e.from.unit === u && (e.to.unit === ticket.unit || e.to.unit === '*'))
    if (!edge) out.push(`blocker unit ${u} has no table edge into ${ticket.unit}`)
  }
  return out
}

// Reading epics and tickets from GitHub.

function mapTicket(tickets, node, epic, rows, repo, errors) {
  const other = node.repository.nameWithOwner
  const key = other === repo ? `#${node.number}` : `${other}#${node.number}`
  const row = rows[key]
  const ticket = {
    number: node.number,
    repo: other,
    title: node.title,
    epic,
    position: row ? row.position : null,
    issueState: node.state,
    stateReason: node.stateReason ?? null,
    built: node.state === 'CLOSED' && closedByMergedPullRequest(node),
    openPullRequests: (node.closedByPullRequestsReferences?.nodes ?? [])
      .filter((pr) => pr.state === 'OPEN')
      .map((pr) => pr.number),
    tableState: row ? row.tableState : null,
    unit: row ? row.unit : null,
    place: other === repo ? placeNamedInHeader(node.body ?? '') : null,
    blockedBy: null,
    formatError: null,
  }
  if (other === repo && node.state === 'OPEN') {
    try {
      const parsed = parseTicketBody(node.body ?? '')
      ticket.blockedBy = parsed.blockedBy
      ticket.unit = ticket.unit ?? parsed.unit
    } catch (err) {
      ticket.formatError = err.message
      errors.push(`#${node.number}: ${err.message}`)
    }
  }
  tickets[key] = ticket
}

// Marks each blocker built or not, reading references outside the map from GitHub in one call.
function markBlockersBuilt(tickets, repo) {
  const known = new Map(
    Object.entries(tickets)
      .filter(([k]) => k.startsWith('#'))
      .map(([, ticket]) => [ticket.number, ticket]),
  )
  const wanted = [
    ...new Set(
      Object.values(tickets).flatMap((ticket) => (ticket.blockedBy ?? []).filter((b) => b.kind === 'ref').map((b) => b.number)),
    ),
  ].filter((n) => !known.has(n))
  const extra = new Map()
  const closes = new Map()
  if (wanted.length) {
    const d = queryGitHub(
      repo,
      wanted
        .map(
          (n) =>
            `r${n}: issueOrPullRequest(number: ${n}) { __typename ... on PullRequest { merged closingIssuesReferences(first: 5) { nodes { number } } } ... on Issue { state ${CLOSING_PULL_REQUESTS} } }`,
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
      else extra.set(n, node.state === 'CLOSED' && closedByMergedPullRequest(node))
    }
  }
  for (const ticket of Object.values(tickets)) {
    for (const b of ticket.blockedBy ?? []) {
      if (b.kind !== 'ref') continue
      b.built = known.has(b.number) ? known.get(b.number).built : extra.get(b.number)
      if (closes.has(b.number)) b.closes = closes.get(b.number)
    }
  }
}

// Open issues that look like tickets, a header line or a `unit:` label, wherever they sit.
function findTicketLikeIssues(repo) {
  const found = []
  for (let cur = null; ; ) {
    const d = queryGitHub(
      repo,
      `issues(states: OPEN, first: ${PAGE_SIZE}${cur ? `, after: "${cur}"` : ''}) { pageInfo { hasNextPage endCursor } nodes { number body labels(first: 20) { nodes { name } } parent { number } } }`,
    )
    for (const n of d.issues.nodes) {
      const looks = /^\*\*Unit:\*\*/m.test(n.body ?? '') || n.labels.nodes.some((l) => l.name.startsWith('unit:'))
      if (looks) found.push({ number: n.number, parent: n.parent?.number ?? null })
    }
    if (!d.issues.pageInfo.hasNextPage) return found
    cur = d.issues.pageInfo.endCursor
  }
}

function readAllEpicsAndTickets(repo) {
  const errors = []
  const epics = {}
  const tickets = {}
  const top = queryGitHub(
    repo,
    `issue(number: ${ROADMAP_TICKET}) { subIssues(first: ${PAGE_SIZE}) { nodes { number title state body } } }`,
  )
  for (const e of top.issue.subIssues.nodes) {
    epics[e.number] = { title: e.title, state: e.state, place: placeFromTitle(e.title), body: e.body ?? '', rows: parseEpicRows(e.body ?? '') }
  }
  const seen = Object.fromEntries(Object.keys(epics).map((n) => [n, new Set()]))
  let cursors = Object.fromEntries(Object.keys(epics).map((n) => [n, null]))
  while (Object.keys(cursors).length) {
    const fields = Object.entries(cursors)
      .map(
        ([n, cur]) =>
          `e${n}: issue(number: ${n}) { subIssues(first: ${PAGE_SIZE}${cur ? `, after: "${cur}"` : ''}) { pageInfo { hasNextPage endCursor } nodes { ${TICKET_FIELDS} } } }`,
      )
      .join(' ')
    const d = queryGitHub(repo, fields)
    const next = {}
    for (const n of Object.keys(cursors)) {
      const si = d[`e${n}`].subIssues
      for (const node of si.nodes) {
        mapTicket(tickets, node, Number(n), epics[n].rows, repo, errors)
        seen[n].add(node.repository.nameWithOwner === repo ? `#${node.number}` : `${node.repository.nameWithOwner}#${node.number}`)
      }
      if (si.pageInfo.hasNextPage) next[n] = si.pageInfo.endCursor
    }
    cursors = next
  }
  markBlockersBuilt(tickets, repo)

  const structure = []
  for (const [n, e] of Object.entries(epics)) {
    for (const k of seen[n]) {
      const ticket = tickets[k]
      // Closed as a duplicate or as not planned is a deliberate close that needs no row.
      if (ticket.issueState === 'CLOSED' && !ticket.built && ['DUPLICATE', 'NOT_PLANNED'].includes(ticket.stateReason)) continue
      if (!e.rows[k]) structure.push(`no row: ${k} is a sub-issue of epic #${n} with no row in its table`)
    }
    for (const k of Object.keys(e.rows)) if (!seen[n].has(k)) structure.push(`no sub-issue: epic #${n} has a row for ${k}, which is not its sub-issue`)
  }
  const epicNumbers = new Set([ROADMAP_TICKET, ...Object.keys(epics).map(Number)])
  for (const i of findTicketLikeIssues(repo)) {
    if (epicNumbers.has(i.number)) continue
    if (!epicNumbers.has(i.parent)) structure.push(`under no epic: open ticket #${i.number}`)
  }
  for (const [k, ticket] of Object.entries(tickets)) {
    const place = epics[ticket.epic]?.place
    if (ticket.issueState === 'OPEN' && ticket.place && place && ticket.place !== place) {
      structure.push(`misplaced: ${k}'s header names ${ticket.place}, its epic #${ticket.epic} is ${place}`)
    }
  }
  return { epics, tickets, errors, structure }
}

function readTickets(repo, numbers) {
  const errors = []
  const tickets = {}
  const d = queryGitHub(repo, numbers.map((n) => `ticket${n}: issue(number: ${n}) { ${TICKET_FIELDS} parent { number } }`).join(' '))
  for (const n of numbers) {
    const node = d[`ticket${n}`]
    if (!node) {
      errors.push(`#${n}: no such issue`)
      continue
    }
    mapTicket(tickets, node, node.parent?.number ?? null, {}, repo, errors)
  }
  markBlockersBuilt(tickets, repo)
  return { tickets, errors }
}

// The State a ticket's blockers and pull request give it, by the reconcile-epics skill's rules.
// In-progress and hold are not derived, because they need the roster.
function deriveState(ticket, delivered) {
  if (ticket.issueState === 'CLOSED') {
    if (ticket.built) return 'built'
    // Closed as a duplicate or as not planned is a deliberate close, not an anomaly to report.
    return ['DUPLICATE', 'NOT_PLANNED'].includes(ticket.stateReason) ? 'closed-not-built' : 'closed-unmerged'
  }
  if (ticket.formatError) return 'format-error'
  if (ticket.blockedBy === null) return null
  for (const b of ticket.blockedBy) {
    if (b.kind === 'unit') {
      if (delivered === null) return 'unknown-unit'
      if (!delivered.has(b.unit)) return 'blocked'
    } else if (!b.built) return 'blocked'
  }
  return 'pending'
}

function readCurrentMap(options) {
  const { epics, tickets, errors, structure } = readAllEpicsAndTickets(options.repo)
  const roadmap = readRoadmap(options.roadmap)
  for (const ticket of Object.values(tickets)) ticket.derived = deriveState(ticket, roadmap?.delivered ?? null)
  const epicBodies = {}
  for (const [n, e] of Object.entries(epics)) {
    epicBodies[n] = { body: e.body, rows: e.rows }
    delete e.body
    delete e.rows
  }
  return {
    ticketMap: { fetchedAt: new Date().toISOString(), repo: options.repo, roadmapIssue: ROADMAP_TICKET, epics, tickets },
    epicBodies,
    roadmap,
    errors,
    structure,
  }
}

// Printing.

// The State in the epic's table differs from the one the blockers give. A row reading in-progress
// or hold never differs, because those States are not derived.
const stateInEpicDiffers = (ticket) =>
  ticket.derived !== null && ticket.derived !== 'closed-not-built' && ![ticket.derived, 'in-progress', 'hold'].includes(ticket.tableState)

const unbuiltBlockers = (ticket) =>
  (ticket.blockedBy ?? [])
    .filter((b) => b.kind === 'unit' || !b.built)
    .map((b) => (b.kind === 'ref' ? `#${b.number}` : b.unit))
    .join(', ')

// Prints a table: a header line, then the rows, columns padded to their widest cell. Nothing at all
// when there are no rows.
function printTable(headers, rows) {
  if (!rows.length) return
  const widths = headers.map((h, i) => Math.max(h.length, ...rows.map((r) => String(r[i] ?? '').length)))
  const fmt = (r) =>
    r
      .map((c, i) => (i === r.length - 1 ? String(c ?? '') : String(c ?? '').padEnd(widths[i])))
      .join('  ')
      .trimEnd()
  console.log(fmt(headers))
  for (const r of rows) console.log(fmt(r))
}

const TICKET_HEADERS = ['TICKET', 'EPIC', 'UNIT', 'STATE IN EPIC', 'STATE FROM BLOCKERS', 'NOTE']
const ticketTableRow = (k, ticket) => [
  k,
  `#${ticket.epic}`,
  ticket.unit ?? '-',
  ticket.tableState ?? '(no row)',
  ticket.derived ?? '-',
  [stateInEpicDiffers(ticket) ? 'differs' : '', ticket.derived === 'blocked' ? `waits on ${unbuiltBlockers(ticket)}` : ''].filter(Boolean).join('; '),
]

// The one-line form of a ticket that --json's diff changes carry.
const ticketLine = (k, ticket) =>
  `${k.padEnd(8)} epic #${ticket.epic}  ${(ticket.unit ?? '-').padEnd(4)} ${(ticket.tableState ?? '(no row)').padEnd(11)} derived ${(ticket.derived ?? '-').padEnd(14)}${stateInEpicDiffers(ticket) ? ' !' : ''}${ticket.derived === 'blocked' ? `  waits on ${unbuiltBlockers(ticket)}` : ''}`

const byTicketKey = (a, b) => a.length - b.length || a.localeCompare(b)

function writeMapFile(path, ticketMap) {
  writeFileSync(`${path}.tmp`, JSON.stringify(ticketMap, null, 1))
  renameSync(`${path}.tmp`, path)
}

function printFindingsAndExitCode(options, errors, structure, json) {
  if (options.json) console.log(JSON.stringify(json, null, 1))
  if (!options.json) {
    printTable(
      ['STRUCTURE', 'FINDING'],
      (structure ?? []).map((x) => {
        const i = x.indexOf(': ')
        return [x.slice(0, i), x.slice(i + 2)]
      }),
    )
  }
  for (const e of errors) console.error(`FORMAT ${e}`)
  return errors.length ? 2 : 0
}

// Commands that read.

function cmdRefresh(options) {
  const { ticketMap, errors, structure } = readCurrentMap(options)
  writeMapFile(options.map, ticketMap)
  if (!options.json) {
    const counts = {}
    for (const ticket of Object.values(ticketMap.tickets)) counts[ticket.derived ?? '-'] = (counts[ticket.derived ?? '-'] ?? 0) + 1
    printTable(['STATE FROM BLOCKERS', 'TICKETS'], Object.entries(counts))
    const mism = Object.keys(ticketMap.tickets).filter((k) => stateInEpicDiffers(ticketMap.tickets[k]))
    printTable(TICKET_HEADERS, mism.map((k) => ticketTableRow(k, ticketMap.tickets[k])))
  }
  return printFindingsAndExitCode(options, errors, structure, { ticketMap, structure, errors })
}

// The names `diff` prints for the fields it compares. The saved map keeps the field names.
const FIELD_LABELS = { tableState: 'state in epic', derived: 'state from blockers', issueState: 'issue state', stateReason: 'close reason', blockedBy: 'blocked by', formatError: 'format error', openPullRequests: 'open pull requests' }
const COMPARED_FIELDS = ['epic', 'issueState', 'stateReason', 'built', 'tableState', 'blockedBy', 'derived', 'formatError', 'openPullRequests']

function countsSummary(ticketMap) {
  const counts = {}
  for (const ticket of Object.values(ticketMap.tickets)) counts[ticket.derived ?? '-'] = (counts[ticket.derived ?? '-'] ?? 0) + 1
  return `${Object.keys(ticketMap.tickets).length} tickets in ${Object.keys(ticketMap.epics).length} epics: ${Object.entries(counts).map(([k, v]) => `${k} ${v}`).join(', ')}`
}

function cmdDiff(options) {
  // A first run has no map yet, so everything on GitHub reads as added.
  const old = existsSync(options.map)
    ? JSON.parse(readFileSync(options.map, 'utf8'))
    : { fetchedAt: 'no earlier map', epics: {}, tickets: {} }
  const { ticketMap: now, errors, structure } = readCurrentMap(options)
  if (!existsSync(options.map) && !options.json) {
    // With no earlier map every ticket would read as added, so print the counts alone.
    if (options.save) writeMapFile(options.map, now)
    console.log(`no earlier map${options.save ? ', saved' : ''}: ${countsSummary(now)}`)
    return printFindingsAndExitCode(options, errors, structure, null)
  }
  const changes = []
  const rows = []
  for (const k of [...new Set([...Object.keys(old.tickets), ...Object.keys(now.tickets)])].sort(byTicketKey)) {
    const a = old.tickets[k]
    const b = now.tickets[k]
    if (!a) {
      changes.push(`+ ${ticketLine(k, b)}`)
      rows.push(['added', k, `#${b.epic}`, b.unit ?? '-', b.tableState ?? '(no row)', b.derived ?? '-', ''])
    } else if (!b) {
      changes.push(`- ${k} no longer under an epic`)
      rows.push(['removed', k, '', '', '', '', 'no longer under an epic'])
    }
    else {
      const ds = COMPARED_FIELDS.filter((f) => JSON.stringify(a[f]) !== JSON.stringify(b[f])).map(
        (f) => `${FIELD_LABELS[f] ?? f} ${JSON.stringify(a[f])} -> ${JSON.stringify(b[f])}`,
      )
      if (!ds.length) continue
      const tag =
        a.derived === 'blocked' && b.derived === 'pending'
          ? 'UNBLOCKED '
          : a.derived === 'pending' && b.derived === 'blocked'
            ? 'BLOCKED '
            : ''
      changes.push(`~ ${k} ${tag}${ds.join('; ')}`)
      rows.push(['changed', k, `#${b.epic}`, b.unit ?? '-', b.tableState ?? '(no row)', b.derived ?? '-', `${tag}${ds.join('; ')}`])
    }
  }
  for (const k of new Set([...Object.keys(old.epics), ...Object.keys(now.epics)])) {
    if (JSON.stringify(old.epics[k]) !== JSON.stringify(now.epics[k])) {
      changes.push(`epic #${k}: ${JSON.stringify(old.epics[k])} -> ${JSON.stringify(now.epics[k])}`)
      rows.push(['epic', `#${k}`, '', '', '', '', `${JSON.stringify(old.epics[k])} -> ${JSON.stringify(now.epics[k])}`])
    }
  }
  if (!options.json) printTable(['CHANGE', 'TICKET', 'EPIC', 'UNIT', 'STATE IN EPIC', 'STATE FROM BLOCKERS', 'DETAIL'], rows)
  if (options.save) writeMapFile(options.map, now)
  return printFindingsAndExitCode(options, errors, structure, { since: old.fetchedAt, changes, structure, errors })
}

function cmdShow(options) {
  const ticketMap = JSON.parse(readFileSync(options.map, 'utf8'))
  const keys = options.numbers.length ? options.numbers.map((n) => `#${n.replace(/^#/, '')}`) : Object.keys(ticketMap.tickets).sort(byTicketKey)
  if (options.json) console.log(JSON.stringify(keys.map((k) => ticketMap.tickets[k] ?? null), null, 1))
  else {
    printTable(
      TICKET_HEADERS,
      keys.map((k) => (ticketMap.tickets[k] ? ticketTableRow(k, ticketMap.tickets[k]) : [k, '', '', '', '', 'not in the map'])),
    )
  }
  return 0
}

function cmdDeps(options) {
  const { tickets, errors } = readTickets(options.repo, options.numbers.map((n) => Number(n.replace(/^#/, ''))))
  const delivered = readRoadmap(options.roadmap)?.delivered ?? null
  for (const ticket of Object.values(tickets)) ticket.derived = deriveState(ticket, delivered)
  if (!options.json) {
    printTable(
      ['TICKET', 'ISSUE', 'EPIC', 'STATE FROM BLOCKERS', 'BLOCKED BY'],
      Object.entries(tickets).map(([k, ticket]) => {
        const blockers =
          (ticket.blockedBy ?? [])
            .map((b) =>
              b.kind === 'ref'
                ? `#${b.number} ${b.built ? 'built' : 'not built'}`
                : `${b.unit} ${delivered?.has(b.unit) ? 'delivered' : 'not delivered'}`,
            )
            .join(', ') || (ticket.blockedBy === null ? 'not read, the ticket is closed or in another repository' : 'none')
        return [k, ticket.issueState.toLowerCase(), ticket.epic === null ? '-' : `#${ticket.epic}`, ticket.derived ?? '-', blockers]
      }),
    )
  }
  return printFindingsAndExitCode(options, errors, [], { tickets, errors })
}

// The epics the production point gate admits. The value path in ROADMAP.md gives the order, and a
// point is crossed when its issue, found by its title among the roadmap ticket's sub-issues, is
// closed. Admitted are the increments before the next point not yet crossed, and that point's own
// issue once every one of its increments' issues is closed.
function epicsAdmittedByGate(ticketMap, roadmap) {
  const byPlace = new Map(Object.entries(ticketMap.epics).map(([n, e]) => [e.place, { number: Number(n), ...e }]))
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

function cmdCandidates(options) {
  const { ticketMap, roadmap, errors } = readCurrentMap(options)
  if (!roadmap) throw new Error(`candidates needs ROADMAP.md at ${options.roadmap}`)
  const admitted = epicsAdmittedByGate(ticketMap, roadmap)
  const excluded = new Set(options.excluded)
  const byNumber = new Map(
    Object.entries(ticketMap.tickets)
      .filter(([k]) => k.startsWith('#'))
      .map(([, ticket]) => [ticket.number, ticket]),
  )
  const dependents = new Map()
  for (const ticket of byNumber.values()) {
    if (ticket.issueState !== 'OPEN') continue
    for (const b of ticket.blockedBy ?? []) if (b.kind === 'ref') dependents.set(b.number, [...(dependents.get(b.number) ?? []), ticket.number])
  }
  const out = []
  for (const ticket of byNumber.values()) {
    if (!admitted.has(ticket.epic) || ticket.repo !== options.repo || ticket.issueState !== 'OPEN') continue
    if (ticket.tableState !== 'pending' || ticket.derived !== 'pending' || ticket.openPullRequests.length || excluded.has(ticket.number)) continue
    out.push({
      number: ticket.number,
      title: ticket.title,
      unit: ticket.unit,
      epic: ticket.epic,
      place: ticketMap.epics[ticket.epic].place,
      epicPosition: admitted.get(ticket.epic),
      row: ticket.position,
      blockedBy: (ticket.blockedBy ?? []).map((b) => (b.kind === 'ref' ? `#${b.number}` : b.unit)),
      dependents: (dependents.get(ticket.number) ?? []).map((n) => `#${n}`),
      disagreements: findRoadmapDisagreements(ticket, byNumber, roadmap),
    })
  }
  // By number, so the order the tickets launch in is decided only by the launch-ticket skill.
  out.sort((a, b) => a.number - b.number)
  if (!options.json) {
    printTable(
      ['TICKET', 'UNIT', 'EPIC', 'PLACE ON VALUE PATH', 'ROW IN EPIC', 'TITLE', 'BLOCKED BY', 'WAITING ON IT', 'ROADMAP DISAGREES'],
      out.map((c) => [
        `#${c.number}`,
        c.unit ?? '-',
        `${c.place}#${c.epic}`,
        c.epicPosition,
        c.row,
        c.title.length > 60 ? `${c.title.slice(0, 57)}...` : c.title,
        c.blockedBy.join(' ') || 'none',
        c.dependents.join(' ') || 'none',
        c.disagreements.join('; '),
      ]),
    )
  }
  return printFindingsAndExitCode(options, errors, [], { candidates: out, errors })
}

// Commands that write. Each prints what it would change and changes GitHub only with --write.

// Corrects the State cells of the epics' rows to what the tickets' blockers and merged pull requests
// give. A row of an open ticket that reads hold or in-progress stays unless `named` names its
// ticket. With `onlyNamed` no other row changes. A named ticket gets the State given for it, or when that is BY_BLOCKERS the
// one its Blocked by line gives.
const BY_BLOCKERS = 'by-blockers'

function cmdUpdateStates(options) {
  return updateStates(options, new Map(), false)
}

function cmdSetState(options) {
  const match = /^#?(\d+)$/.exec(options.numbers[0] ?? '')
  const state = options.numbers[1]
  if (!match || options.numbers.length !== 2 || !['in-progress', 'hold'].includes(state)) {
    throw new Error('set-state takes a ticket number and in-progress or hold')
  }
  return updateStates(options, new Map([[`#${match[1]}`, state]]), true)
}

function cmdResetState(options) {
  const match = /^#?(\d+)$/.exec(options.numbers[0] ?? '')
  if (!match || options.numbers.length !== 1) throw new Error('reset-state takes a ticket number')
  return updateStates(options, new Map([[`#${match[1]}`, BY_BLOCKERS]]), true)
}

function updateStates(options, named, onlyNamed) {
  const { ticketMap, epicBodies, errors } = readCurrentMap(options)
  const changes = []
  for (const [k, ticket] of Object.entries(ticketMap.tickets)) {
    const row = epicBodies[ticket.epic]?.rows[k]
    if (!row) continue
    let want = null
    const set = named.get(k)
    if (onlyNamed && !set) continue
    if (set === BY_BLOCKERS) {
      if (!['pending', 'blocked'].includes(ticket.derived)) throw new Error(`reset-state ${k}: its blockers give ${ticket.derived}, not pending or blocked`)
      want = ticket.derived
    }
    else if (set) want = set
    else if (ticket.derived === 'built') want = 'built'
    else if (['pending', 'blocked'].includes(ticket.derived) && !['hold', 'in-progress'].includes(row.tableState)) want = ticket.derived
    if (want && want !== row.tableState) changes.push({ key: k, epic: ticket.epic, line: row.line, from: row.tableState, to: want })
  }
  for (const k of named.keys()) {
    if (!changes.some((c) => c.key === k) && !Object.values(epicBodies).some((b) => b.rows[k])) throw new Error(`${k} has no row in any epic`)
  }
  const edited = {}
  for (const c of changes) {
    const lines = (edited[c.epic] ??= epicBodies[c.epic].body.split('\n'))
    const cells = lines[c.line].split('|')
    // A row is | unit | ticket | what lands | state |, so the State cell is the fifth piece.
    cells[4] = ` ${c.to} `
    lines[c.line] = cells.join('|')
  }
  if (!options.json) {
    printTable(['EPIC', 'TICKET', 'OLD STATE', 'NEW STATE'], changes.map((c) => [`#${c.epic}`, c.key, c.from, c.to]))
    if (options.showLines) {
      for (const c of changes) {
        console.log(`--- epic #${c.epic} line ${c.line + 1}\n- ${epicBodies[c.epic].body.split('\n')[c.line]}\n+ ${edited[c.epic][c.line]}`)
      }
    }
  }
  if (options.write) {
    const [owner, name] = options.repo.split('/')
    for (const n of Object.keys(edited)) backupBody(options, n, epicBodies[n].body)
    for (const [n, lines] of Object.entries(edited)) {
      // The body travels as JSON on standard input, never as a shell argument.
      runGh(['api', '-X', 'PATCH', `repos/${owner}/${name}/issues/${n}`, '--input', '-'], JSON.stringify({ body: lines.join('\n') }))
    }
  }
  return printFindingsAndExitCode(options, errors, [], { changes, written: Boolean(options.write), errors })
}



const OWNER_REPO = /^(?:([\w.-]+\/[\w.-]+))?#?(\d+)$/

function parseTicketReference(text, repo) {
  const m = OWNER_REPO.exec(text)
  if (!m) throw new Error(`not a ticket reference: ${text}`)
  return { repo: m[1] ?? repo, number: Number(m[2]), key: `${m[1] && m[1] !== repo ? m[1] : ''}#${m[2]}` }
}

function readOneTicket(options, ref) {
  const node = queryGitHub(ref.repo, `issue(number: ${ref.number}) { ${TICKET_FIELDS} databaseId parent { number } }`).issue
  if (!node) throw new Error(`${ref.key}: no such issue`)
  const tickets = {}
  const errors = []
  mapTicket(tickets, node, node.parent?.number ?? null, {}, options.repo, errors)
  markBlockersBuilt(tickets, options.repo)
  const ticket = tickets[ref.key]
  ticket.derived = deriveState(ticket, readRoadmap(options.roadmap)?.delivered ?? null)
  return { ticket, node }
}

function readEpicIssue(options, number) {
  const e = queryGitHub(options.repo, `issue(number: ${number}) { state title body }`).issue
  if (!e) throw new Error(`epic #${number}: no such issue`)
  return { ...e, lines: (e.body ?? '').split('\n') }
}

const cell0 = (line) => line.split('|')[1] ?? ''

// Removes a ticket's row. A row that opened its unit's group hands the unit cell to the next row of
// the group, so the group keeps its name.
function removeRowFromLines(lines, key) {
  const rows = parseEpicRows(lines.join('\n'))
  const r = rows[key]
  if (!r) return null
  const next = Object.values(rows).find((x) => x.line === r.line + 1)
  if (cell0(lines[r.line]).trim() && next && next.unit === r.unit && !cell0(lines[next.line]).trim()) {
    const cells = lines[next.line].split('|')
    cells[1] = cell0(lines[r.line])
    lines[next.line] = cells.join('|')
  }
  return lines.splice(r.line, 1)[0]
}

// Adds a row under its unit's group, or at the end of the table under a new bold unit cell.
function addRowToLines(lines, key, unit, groupTitle, what, rowState) {
  const rows = parseEpicRows(lines.join('\n'))
  if (rows[key]) throw new Error(`${key} already has a row in the epic's table`)
  const group = Object.values(rows).filter((r) => r.unit === unit)
  const all = Object.values(rows)
  let at
  if (all.length) at = Math.max(...(group.length ? group : all).map((r) => r.line)) + 1
  else {
    const head = lines.findIndex((l) => /^\|\s*Unit\s*\|\s*Ticket/.test(l))
    if (head < 0) throw new Error("the epic has no table with a Unit and Ticket column")
    at = head + 2
  }
  const row = `| ${group.length ? '' : `**${groupTitle}**`} | ${key} | ${what} | ${rowState} |`
  lines.splice(at, 0, row)
  return row
}

function rowContentFor(options, ref, ticket, node, epicLines, roadmap) {
  const header = /^\*\*Unit:\*\*\s*\[([A-Z]\d+)/m.exec(node.body ?? '')
  const unit = ticket.repo !== options.repo ? ticket.repo.split('/')[1] : header?.[1]
  if (!unit) throw new Error(`${ref.key}: no unit in its header line`)
  const inEpic = Object.values(parseEpicRows(epicLines.join('\n'))).some((r) => r.unit === unit)
  const groupTitle = options.unitTitle ?? roadmap?.unitTitles[unit] ?? (ticket.repo !== options.repo ? unit : null)
  if (!inEpic && !groupTitle) throw new Error(`unit ${unit} is new to the epic and has no title in ROADMAP.md, give --unit-title`)
  const rowState = options.rowState ?? (['pending', 'blocked', 'built'].includes(ticket.derived) ? ticket.derived : null)
  if (!rowState) throw new Error(`${ref.key}: no State derivable (${ticket.derived}), give --row-state`)
  return { unit, groupTitle, what: ticket.title.replace(/ \(unit [A-Z]\d+\)$/, ''), rowState }
}

function linkSubIssue(options, epic, id) {
  const [owner, name] = options.repo.split('/')
  runGh(['api', '-X', 'POST', `repos/${owner}/${name}/issues/${epic}/sub_issues`, '--input', '-'], JSON.stringify({ sub_issue_id: id, replace_parent: true }))
}

function unlinkSubIssue(options, epic, id) {
  const [owner, name] = options.repo.split('/')
  runGh(['api', '-X', 'DELETE', `repos/${owner}/${name}/issues/${epic}/sub_issue`, '--input', '-'], JSON.stringify({ sub_issue_id: id }))
}

// Saves the body about to be replaced to BACKUP_DIR/OWNER/NAME/NUMBER and prints the path. A backup
// that earlier write left there is renamed with a timestamp suffix, so each version survives. A
// failure throws, so the write that follows it does not happen.
function backupBody(options, number, body) {
  const dir = `${BACKUP_DIR}/${options.repo}`
  mkdirSync(dir, { recursive: true })
  const path = `${dir}/${number}`
  if (existsSync(path)) renameSync(path, `${path}.${new Date().toISOString().replace(/[-:]/g, '').replace('.', '')}`)
  writeFileSync(path, body ?? '', { flag: 'wx' })
  ;(options.json ? console.error : console.log)(`backup #${number} ${path}`)
}

function writeBody(options, number, lines) {
  const [owner, name] = options.repo.split('/')
  runGh(['api', '-X', 'PATCH', `repos/${owner}/${name}/issues/${number}`, '--input', '-'], JSON.stringify({ body: lines.join('\n') }))
}

function printPlan(options, plan, extra = {}) {
  if (options.json) console.log(JSON.stringify({ plan, written: Boolean(options.write), ...extra }, null, 1))
  else printTable(['EPIC', 'ACTION', 'TICKET', 'DETAIL'], plan)
  return 0
}

function cmdAddTicket(options) {
  const epicNo = Number(options.epic.replace(/^#/, ''))
  const ref = parseTicketReference(options.numbers[0], options.repo)
  const epic = readEpicIssue(options, epicNo)
  if (epic.state === 'CLOSED') throw new Error(`epic #${epicNo} is closed, and no ticket is added to a closed epic`)
  const { ticket, node } = readOneTicket(options, ref)
  const parent = node.parent?.number ?? null
  if (parent && parent !== epicNo) throw new Error(`${ref.key} is a sub-issue of epic #${parent}, use move-ticket to put it under #${epicNo}`)
  const r = rowContentFor(options, ref, ticket, node, epic.lines, readRoadmap(options.roadmap))
  const row = addRowToLines(epic.lines, ref.key, r.unit, r.groupTitle, r.what, r.rowState)
  // A session links its own ticket first, so the link is usually there and only the row is added.
  const linked = parent === epicNo
  const plan = [
    [`#${epicNo}`, 'link', ref.key, linked ? `already linked as a sub-issue of #${epicNo}` : `sub-issue of #${epicNo}`],
    [`#${epicNo}`, 'add row', ref.key, row],
  ]
  if (options.write) {
    backupBody(options, epicNo, epic.body)
    if (!linked) linkSubIssue(options, epicNo, node.databaseId)
    writeBody(options, epicNo, epic.lines)
  }
  return printPlan(options, plan)
}

function cmdMoveTicket(options) {
  const ref = parseTicketReference(options.numbers[0], options.repo)
  const to = Number(options.epic.replace(/^#/, ''))
  const { ticket, node } = readOneTicket(options, ref)
  const from = node.parent?.number ?? null
  if (from === to) throw new Error(`${ref.key} is already under epic #${to}`)
  const target = readEpicIssue(options, to)
  if (target.state === 'CLOSED') throw new Error(`epic #${to} is closed, and no ticket is added to a closed epic`)
  const plan = [[`#${to}`, 'link', ref.key, `sub-issue of #${to}${from ? `, replacing parent #${from}` : ''}`]]
  const source = from ? readEpicIssue(options, from) : null
  const unit = source ? parseEpicRows(source.lines.join('\n'))[ref.key]?.unit : null
  const old = source ? removeRowFromLines(source.lines, ref.key) : null
  if (old) plan.push([`#${from}`, 'remove row', ref.key, old])
  let row
  if (old) {
    // The row keeps its words and State, and goes under its unit's group in the new epic.
    const cells = old.split('|').map((c) => c.trim())
    const title = cell0(old).replace(/\*\*/g, '').trim() || readRoadmap(options.roadmap)?.unitTitles[unit] || unit
    row = addRowToLines(target.lines, ref.key, unit, title, cells[3], cells[4])
  } else {
    const r = rowContentFor(options, ref, ticket, node, target.lines, readRoadmap(options.roadmap))
    row = addRowToLines(target.lines, ref.key, r.unit, r.groupTitle, r.what, r.rowState)
  }
  plan.push([`#${to}`, 'add row', ref.key, row])
  if (options.write) {
    backupBody(options, to, target.body)
    if (old) backupBody(options, from, source.body)
    linkSubIssue(options, to, node.databaseId)
    writeBody(options, to, target.lines)
    if (old) writeBody(options, from, source.lines)
  }
  return printPlan(options, plan)
}

function cmdRemoveRow(options) {
  const ref = parseTicketReference(options.numbers[0], options.repo)
  const { node } = readOneTicket(options, ref)
  const parent = node.parent?.number ?? null
  // A row left behind by a ticket that is no longer the epic's sub-issue is removed with --epic.
  const from = options.epic ? Number(options.epic.replace(/^#/, '')) : parent
  if (options.unlink && from !== parent) throw new Error(`${ref.key} is not a sub-issue of epic #${from}, so --unlink has nothing to remove there`)
  const plan = []
  const source = from ? readEpicIssue(options, from) : null
  const old = source ? removeRowFromLines(source.lines, ref.key) : null
  if (old) plan.push([`#${from}`, 'remove row', ref.key, old])
  if (options.unlink) {
    if (!from) throw new Error(`${ref.key} is not a sub-issue of any epic`)
    plan.push([`#${from}`, 'unlink', ref.key, `no longer a sub-issue of #${from}`])
  }
  if (!plan.length) throw new Error(`${ref.key} has no row${from ? ` in epic #${from}` : ' and no parent epic'}`)
  if (options.write) {
    if (old) backupBody(options, from, source.body)
    if (old) writeBody(options, from, source.lines)
    if (options.unlink) unlinkSubIssue(options, from, node.databaseId)
  }
  return printPlan(options, plan)
}

// Recomputes each overview row of the roadmap ticket from the epics: its Status, its Units and its
// Tickets. Status is the epic's issue state, open or closed for an increment and open or crossed for
// a point, and `not created` for a point row that links ROADMAP.md because it has no issue yet.
// Tickets is `built / total` over the epic's tickets, leaving out those closed as a duplicate or as
// not planned. Units are the increment's own units in the order its section of ROADMAP.md lists
// them (a unit the section omits keeps its table place), then the units another increment's section
// lists as later work, in the epic's table order. A production point lists its units in table order.
function cmdUpdateOverview(options) {
  const { ticketMap, epicBodies, roadmap, errors } = readCurrentMap(options)
  const top = readEpicIssue(options, ROADMAP_TICKET)
  const plan = []
  const head = top.lines.findIndex((l) => /^\|\s*Value increment or production point\s*\|/.test(l))
  if (head < 0) throw new Error('the roadmap ticket has no overview table')
  let end = head
  while (/^\|/.test(top.lines[end + 1] ?? '')) end++
  if (!/\|\s*Status\s*\|/.test(top.lines[head])) {
    plan.push([`#${ROADMAP_TICKET}`, 'column', 'Status', 'added after the first column'])
    for (let i = head; i <= end; i++) {
      const cells = top.lines[i].split('|')
      cells.splice(2, 0, i === head ? ' Status ' : i === head + 1 ? ' --- ' : ' ')
      top.lines[i] = cells.join('|')
    }
  }
  // An epic that is a sub-issue of the roadmap ticket and has no row gets one, in value-path order.
  const placeOfRow = (line) => placeFromTitle(/\[([^\]]*)\]/.exec(line.split('|')[1])?.[1] ?? '')
  const order = roadmap?.path ?? []
  const added = new Set()
  const missing = Object.entries(ticketMap.epics).filter(([, e]) => !top.lines.slice(head + 2, end + 1).some((l) => placeOfRow(l) === e.place))
  for (const [n, e] of missing.sort((x, y) => order.indexOf(x[1].place) - order.indexOf(y[1].place))) {
    let at = head + 2
    for (let i = head + 2; i <= end; i++) if (order.indexOf(placeOfRow(top.lines[i])) < order.indexOf(e.place)) at = i + 1
    top.lines.splice(at, 0, `| [${e.title}](https://github.com/${options.repo}/issues/${n}) |  |  |  |`)
    end++
    // Rows already added that sit at or below the insertion move down one line.
    for (const a of [...added].sort((x, y) => y - x)) if (a >= at) { added.delete(a); added.add(a + 1) }
    added.add(at)
    plan.push([`#${ROADMAP_TICKET}`, 'add row', `#${n}`, e.title])
  }
  for (let i = head + 2; i <= end; i++) {
    const cells = top.lines[i].split('|')
    const title = /\[([^\]]*)\]/.exec(cells[1])?.[1] ?? cells[1].trim()
    // The row's epic is the one whose place its title names, whatever its link says.
    const place = placeFromTitle(title)
    const found = Object.entries(ticketMap.epics).find(([, e]) => e.place === place)
    const epicNo = found ? Number(found[0]) : NaN
    const epic = found?.[1]
    if (epic && !cells[1].includes(`/issues/${epicNo})`)) {
      plan.push([`#${ROADMAP_TICKET}`, 'link', `#${epicNo}`, `${title}: ${/\(([^)]*)\)/.exec(cells[1])?.[1] ?? '(none)'} -> issue #${epicNo}`])
      cells[1] = cells[1].replace(/\([^)]*\)/, `(https://github.com/${options.repo}/issues/${epicNo})`)
    }
    const set = (col, name, to) => {
      if (cells[col].trim() !== to && !added.has(i)) plan.push([`#${ROADMAP_TICKET}`, name, epic ? `#${epicNo}` : title, `${title}: ${cells[col].trim() || '(empty)'} -> ${to}`])
      cells[col] = ` ${to} `
    }
    if (!epic) {
      if (/ROADMAP\.md/.test(cells[1])) set(2, 'status', 'not created')
      top.lines[i] = cells.join('|')
      continue
    }
    const tableUnits = [...new Set(Object.values(epicBodies[epicNo].rows).sort((x, y) => x.position - y.position).map((r) => r.unit))]
    let units
    if (epic.place.startsWith('V')) {
      // A unit that another increment's section lists is that increment's, so here it is later work.
      const elsewhere = (u) => Object.entries(roadmap?.homes ?? {}).some(([place, list]) => place !== epic.place && list.includes(u))
      // The increment's own units follow its section's order. A unit the section does not list keeps
      // its place in the table, and the listed units fill the other places in the section's order.
      const own = tableUnits.filter((u) => !elsewhere(u))
      const listed = (roadmap?.homes[epic.place] ?? []).filter((u) => own.includes(u))
      const fill = [...listed]
      const home = own.map((u) => (listed.includes(u) ? fill.shift() : u))
      const later = tableUnits.filter(elsewhere)
      units = home.join(' · ') + (later.length ? `, and later work under ${later.join(' · ')}` : '')
    } else units = tableUnits.join(' · ')
    const counted = Object.values(ticketMap.tickets).filter((ticket) => ticket.epic === epicNo && ticket.derived !== 'closed-not-built')
    set(2, 'status', epic.state === 'CLOSED' ? (epic.place.startsWith('V') ? 'closed' : 'crossed') : 'open')
    set(3, 'units', units)
    set(4, 'tickets', `${counted.filter((ticket) => ticket.derived === 'built').length} / ${counted.length}`)
    top.lines[i] = cells.join('|')
  }
  if (options.write && plan.length) {
    backupBody(options, ROADMAP_TICKET, top.body)
    writeBody(options, ROADMAP_TICKET, top.lines)
  }
  printPlan(options, plan)
  return errors.length ? (errors.forEach((e) => console.error(`FORMAT ${e}`)), 2) : 0
}

function parseArgs(argv) {
  const options = {
    repo: 'ppat/mediated-mailbox-mcp',
    roadmap: 'ROADMAP.md',
    map: null,
    write: false,
    save: false,
    json: false,
    showLines: false,
    epic: null,
    unlink: false,
    rowState: null,
    unitTitle: null,
    excluded: [],
    numbers: [],
  }
  const [command, ...rest] = argv
  if (!command || ['help', '-h', '--help'].includes(command) || rest.some((a) => a === '-h' || a === '--help')) {
    return { help: true, bad: !command }
  }
  for (let i = 0; i < rest.length; i++) {
    const a = rest[i]
    if (a === '--repo') options.repo = rest[++i]
    else if (a === '--roadmap') options.roadmap = rest[++i]
    else if (a === '--map') options.map = rest[++i]
    else if (a === '--write') options.write = true
    else if (a === '--save') options.save = true
    else if (a === '--json') options.json = true
    else if (a === '--show-lines') options.showLines = true
    else if (a === '--epic') options.epic = rest[++i]
    else if (a === '--unlink') options.unlink = true
    else if (a === '--row-state') options.rowState = rest[++i]
    else if (a === '--unit-title') options.unitTitle = rest[++i]
    else if (a === '--exclude') options.excluded = rest[++i].split(',').map((n) => Number(n.replace(/^#/, '')))
    else if (a.startsWith('--')) throw new Error(`unknown option: ${a}`)
    else options.numbers.push(a)
  }
  const commands = {
    refresh: cmdRefresh,
    diff: cmdDiff,
    show: cmdShow,
    deps: cmdDeps,
    candidates: cmdCandidates,
    'update-states': cmdUpdateStates,
    'set-state': cmdSetState,
    'reset-state': cmdResetState,
    'add-ticket': cmdAddTicket,
    'move-ticket': cmdMoveTicket,
    'remove-row': cmdRemoveRow,
    'update-overview': cmdUpdateOverview,
  }
  if (!commands[command]) return { help: true, bad: true, message: `unknown command: ${command}` }
  if (options.write && ['refresh', 'diff', 'show', 'deps', 'candidates'].includes(command)) {
    throw new Error(`${command} does not change GitHub, so --write does not apply${command === 'diff' ? '. Use --save to save the map' : ''}`)
  }
  if (['refresh', 'diff', 'show'].includes(command) && !options.map) throw new Error(`${command} needs --map FILE`)
  if (['add-ticket', 'move-ticket'].includes(command) && (options.numbers.length !== 1 || !options.epic)) throw new Error(`${command} takes TICKET --epic EPIC`)
  if (command === 'remove-row' && options.numbers.length !== 1) throw new Error('remove-row takes TICKET')
  if (command === 'deps' && !options.numbers.length) throw new Error('deps needs at least one ticket number')
  return { run: commands[command], options }
}

if (import.meta.main ?? process.argv[1]?.endsWith('tickets.mjs')) {
  try {
    const parsed = parseArgs(process.argv.slice(2))
    if (parsed.help) {
      if (parsed.message) console.error(parsed.message)
      ;(parsed.bad ? console.error : console.log)(USAGE)
      process.exit(parsed.bad ? 1 : 0)
    }
    process.exit(parsed.run(parsed.options))
  } catch (err) {
    console.error(err.message)
    process.exit(1)
  }
}
