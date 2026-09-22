// The checks behind the `commit-taxonomy` status context. Each is a named entry in CHECKS and runs
// as its own workflow step, so a red context names which one fired.
//
// WHAT THIS PROVES. That the configuration in this tree, applied to the dependencies this tree
// holds, cannot compile a commit header commitlint.config.js would reject or a header whose claim
// is false, and that the headers already written in a pull request keep the claims the header
// fields make. Emission is derived from config text plus occupancy, never from a Renovate run:
// `internalChecksFilter: strict` withholds update branches whose release age has not elapsed, so
// whole classes of upgrade never reach a run, and a closure built on one is silently incomplete.
//
// WHAT IT DOES NOT PROVE. Which of a grouped branch's headers Renovate emits when the group mixes
// update types. Renovate assigns the first upgrade's config to the branch after sorting by file
// position and dependency name, which is repository state, not configuration. Every candidate
// header of a group is in the closure, and the merge-time gates lint whatever the branch produced.
//
// Every check refuses to guess. A config shape, a matcher, a template construct or a dependency
// file outside the modelled set is a hard failure, never a skip: silently ignoring what it does
// not understand is how a checker reports green over exactly the drift it exists to catch.

import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { join } from 'node:path'
import { parse as parseYaml } from 'yaml'

// The release parser the message-shape patterns were derived against. A major bump can widen the
// body and footer hole without turning anything else red, so the pin is compared, not trusted.
const RELEASE_PLEASE_MAJOR = 17

// Renovate keys are classified rather than listed. HEADER_SHAPED is the shape of any key that could
// reach a commit header or a pull request title. A key of that shape in neither list below is a
// hard failure, so a preset bump introducing `commitMessage` or `prTitle`, either of which replaces
// the whole prefix and bypasses everything modelled here, cannot arrive unnoticed.
const HEADER_SHAPED = /^(commit|semantic|prTitle)/
const HEADER_KEYS = ['semanticCommitType', 'semanticCommitScope', 'commitMessagePrefix']
const HEADER_SWITCHES = ['semanticCommits']
// Each of these decides subject text or throttling, and no value of any of them can produce or
// suppress the type, the scope or the marker. commitBody and commitTrailers are absent on purpose:
// either can put a breaking-change footer into the message, which is release-operative and
// invisible to a header lint, so they are refused rather than ignored.
const SUBJECT_ONLY_KEYS = [
  'commitMessageAction', 'commitMessageExtra', 'commitMessageTopic', 'commitMessageSuffix',
  'commitMessageLowerCase', 'commitBodyTable', 'commitHourlyLimit', 'commitConcurrentLimit',
]

// Built-in presets this tree extends that set no header field. Renovate's internal presets are not
// fetched, so one outside this list is refused until a reader has classified it.
const INERT_BUILTINS = [
  'abandonments:recommended', 'docker:pinDigests', 'helpers:pinGitHubActionDigests',
  'mergeConfidence:age-confidence-badges', 'mergeConfidence:all-badges',
]

// The update types Renovate can produce for an upgrade that carries a package name, and the one
// for a lock file refresh, which carries none. Every cell is folded under each type it can take.
const PACKAGE_UPDATE_TYPES = ['major', 'minor', 'patch', 'pin', 'pinDigest', 'digest', 'lockfileUpdate', 'rollback', 'bump', 'replacement']
// The update types that move what a shipped artifact is built from. A pin and a digest pin record the
// version already in use, so a shipped cell may render a hidden type under those two.
const VERSION_MOVING_UPDATE_TYPES = ['major', 'minor', 'patch', 'digest', 'lockfileUpdate', 'rollback', 'bump', 'replacement']
const LOCKFILE_UPDATE_TYPES = ['lockFileMaintenance']

// The matchers the fold can evaluate against a cell. Any other match* key on a rule that carries a
// header field is refused: an unevaluated matcher would either skip a setter or apply one wrongly,
// and both report a clean closure over a rule the fold never resolved.
const MODELLED_MATCHERS = ['matchManagers', 'matchFileNames', 'matchDepNames', 'matchPackageNames', 'matchDepTypes', 'matchUpdateTypes', 'matchDatasources']

// Files a Renovate manager this check does not model would read. Occupancy is re-derived on every
// run so a new dependency file is a new cell. A file only an unmodelled manager reads is instead a
// refusal, because its headers would be emitted and never enumerated here.
const UNMODELLED_OCCUPANCY = [
  [/(^|\/)Chart\.lock$/, 'helmv3'], [/(^|\/)values(-[^/]+)?\.ya?ml$/, 'helm-values'],
  [/(^|\/)(package-lock\.json|npm-shrinkwrap\.json)$/, 'npm'], [/(^|\/)yarn\.lock$/, 'npm'], [/(^|\/)pnpm-lock\.yaml$/, 'npm'],
  [/(^|\/)\.tool-versions$/, 'asdf'], [/(^|\/)go\.work$/, 'gomod'], [/(^|\/)Cargo\.toml$/, 'cargo'],
  [/(^|\/)pyproject\.toml$/, 'pep621'], [/(^|\/)requirements[^/]*\.txt$/, 'pip_requirements'], [/(^|\/)Gemfile$/, 'bundler'],
  [/(^|\/)kustomization\.ya?ml$/, 'kustomize'], [/\.tf$/, 'terraform'], [/(^|\/)docker-compose[^/]*\.ya?ml$/, 'docker-compose'],
  [/(^|\/)compose\.ya?ml$/, 'docker-compose'], [/(^|\/)\.gitlab-ci\.ya?ml$/, 'gitlabci'], [/(^|\/)Jenkinsfile$/, 'jenkins'],
  [/(^|\/)action\.ya?ml$/, 'github-actions'], [/(^|\/)devbox\.json$/, 'devbox'], [/(^|\/)\.nvmrc$/, 'nvm'],
  [/(^|\/)Dockerfile\.[^/]+$/, 'dockerfile'], [/(^|\/)[^/]+\.Dockerfile$/, 'dockerfile'], [/(^|\/)Containerfile$/, 'dockerfile'],
]

// The named scopes as path predicates, first match wins. Every other tracked file is the empty
// scope: the shipped surface, or the repository-level residue. Rows here are the rows of the scope
// table in .claude/rules/commits.md whose footprint a path can witness.
const FOOTPRINTS = [
  ['renovate', [/^\.github\/renovate\.json$/]],
  ['internal-workflows', [
    /^\.github\/(workflows|scripts)\//,
    /^(commitlint\.config\.js|package\.json|bun\.lock)$/,
    /^(release-please-config\.json|\.release-please-manifest\.json)$/,
    /^(\.golangci\.yaml|\.sqlfluff|\.sqlfluffignore|\.yamllint|\.markdownlint-cli2\.yaml|\.shellcheckrc|lychee\.toml)$/,
    /^(\.pre-commit-config\.yaml|mise\.toml|mise\.lock)$/,
    /^ui\/browser\/(\.oxlintrc\.json|\.oxfmtrc\.json|sgconfig\.yml|rules\/)/,
  ]],
  ['agents', [/^\.claude\//, /^CLAUDE\.md$/]],
]

// Necessary path conditions for the named scopes a diff can witness. The line-level scopes (a
// version moved and nothing else did) state only the half a path can testify to.
const SCOPE_PATH_CONDITIONS = {
  ...Object.fromEntries(FOOTPRINTS.map(([name, patterns]) => [name, patterns])),
  release: [/^(CHANGELOG\.md|\.release-please-manifest\.json)$/],
  'github-actions': [/^\.github\/workflows\//],
  'internal-dependencies': [
    ...FOOTPRINTS.find(([name]) => name === 'internal-workflows')[1],
    /^testsupport\/cmd\/pgrun\/image\.go$/, /^ui\/browser\/(codegen\/)?(package\.json|bun\.lock)$/,
  ],
}

const CLAIM_TYPES = ['feat', 'fix', 'perf', 'refactor', 'revert']
const HEADER_RE = /^(?<type>[a-zA-Z]+)(?:\((?<scope>[^)]*)\))?(?<bang>!)?: /

const memo = (fn) => { let v; let done = false; return (...a) => { if (!done) { v = fn(...a); done = true } return v } }
const FETCH_ATTEMPTS = 4
async function fetchWithRetry(url) {
  let lastError
  for (let attempt = 1; attempt <= FETCH_ATTEMPTS; attempt++) {
    try {
      const res = await fetch(url, { signal: AbortSignal.timeout(15000) })
      if (res.ok) return await res.text()
      if (res.status === 404) throw Object.assign(new Error(`${url} -> HTTP 404 (the pinned ref or the path does not exist)`), { terminal: true })
      lastError = new Error(`${url} -> HTTP ${res.status}`)
      if (res.status < 500 && res.status !== 429) throw Object.assign(lastError, { terminal: true })
    } catch (e) {
      if (e.terminal) throw e
      lastError = e
    }
    if (attempt < FETCH_ATTEMPTS) await new Promise((r) => setTimeout(r, 1000 * 2 ** (attempt - 1)))
  }
  throw new Error(`${lastError.message} after ${FETCH_ATTEMPTS} attempts`)
}

function memoBy(fn) {
  const cache = new Map()
  return (key) => { if (!cache.has(key)) cache.set(key, fn(key)); return cache.get(key) }
}

// Shared across every env in the process so the self-test's repeated runs fetch each preset once.
const FETCHED = new Map()

export const repoEnv = (root, opts = {}) => {
  const require = createRequire(join(root, 'package.json'))
  const text = (rel) => readFileSync(join(root, rel), 'utf8')
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' })
  const lintCache = new Map()
  const env = {
    root,
    offlinePresets: opts.offlinePresets ?? null,
    text,
    json: (rel) => JSON.parse(env.text(rel)),
    yaml: memoBy((rel) => parseYaml(env.text(rel))),
    commitlint: () => require(join(root, 'commitlint.config.js')),
    commitlintSource: () => env.text('commitlint.config.js'),
    // The gate's own binary, not a re-implementation of it: a checker that models acceptance instead
    // of invoking it can only ever agree with itself.
    lint: (message, configPath = join(root, 'commitlint.config.js')) => {
      const key = `${configPath}\n${message}`
      if (!lintCache.has(key)) {
        try {
          execFileSync(join(root, 'node_modules/.bin/commitlint'), ['--config', configPath],
            { cwd: root, input: message, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
          lintCache.set(key, { accepted: true, output: '' })
        } catch (e) {
          lintCache.set(key, { accepted: false, output: `${e.stdout ?? ''}${e.stderr ?? ''}`.trim() })
        }
      }
      return lintCache.get(key)
    },
    tracked: memo(() => git('ls-files').trim().split('\n').filter(Boolean)),
    commits: memo(() => readCommits(git, opts)),
    prBody: () => opts.prBody ?? process.env.COMMIT_TAXONOMY_PR_BODY ?? '',
    // A transient failure (a network error, a timeout, a 5xx or a 429) is retried with backoff, so a
    // raw-content blip costs seconds rather than a red required check. A 404 is terminal on the first
    // attempt: at a pinned ref it means the pin or the path is wrong, which is a real finding.
    fetchText: async (url) => {
      if (!FETCHED.has(url)) FETCHED.set(url, fetchWithRetry(url))
      return FETCHED.get(url)
    },
    ...(opts.overrides ?? {}),
  }
  return env
}

// The pull request's own commits, with headers and changed paths. Empty on any event that carries
// no pull request, which makes the pull-request-state checks no-ops on the schedule run rather than
// failures.
function readCommits(git, opts) {
  const count = Number(opts.prCommits ?? process.env.COMMIT_TAXONOMY_PR_COMMITS ?? 0)
  if (!count) return []
  const shas = git('log', '--no-merges', '--format=%H', `HEAD~${count}..HEAD`).trim().split('\n').filter(Boolean)
  return shas.map((sha) => ({
    sha,
    message: git('log', '-1', '--format=%B', sha),
    paths: git('show', '--name-only', '--format=', sha).trim().split('\n').filter(Boolean),
  }))
}

const parseHeader = (message) => {
  const header = message.split('\n', 1)[0]
  const m = header.match(HEADER_RE)
  if (!m) return null
  return {
    header,
    type: m.groups.type,
    scope: m.groups.scope ?? '',
    breaking: Boolean(m.groups.bang) || /^BREAKING[ -]CHANGE:/m.test(message),
  }
}

const namedFootprints = (path) => FOOTPRINTS.filter(([, pats]) => pats.some((p) => p.test(path))).map(([name]) => name)
const footprintOf = (path) => namedFootprints(path)[0] ?? ''

// ---------------------------------------------------------------------------------------------
// Occupancy: every dependency cell a modelled manager would extract from the tracked tree.

const DOCKERFILE = /(^|\/)Dockerfile$/
const WORKFLOW = /^\.github\/workflows\/[^/]+\.ya?ml$/

const extractGomod = (env, file) => {
  const cells = []
  const lines = env.text(file).split('\n')
  let block = null
  for (const raw of lines) {
    const line = raw.trim()
    if (/^(require|tool|ignore|exclude|replace|retract)\s*\($/.test(line)) { block = line.split(/\s/)[0]; continue }
    if (line === ')') { block = null; continue }
    const goDirective = line.match(/^go\s+(\S+)$/)
    if (goDirective && !block) { cells.push({ manager: 'gomod', packageFile: file, depName: 'go', depType: 'golang', datasource: 'golang-version' }); continue }
    const single = line.match(/^require\s+(\S+)\s+(\S+)/)
    const inBlock = block === 'require' ? line.match(/^(\S+)\s+(\S+)/) : null
    const m = single ?? inBlock
    if (m) cells.push({ manager: 'gomod', packageFile: file, depName: m[1], depType: line.includes('// indirect') ? 'indirect' : 'require', datasource: 'go' })
  }
  return cells
}

const extractDockerfile = (env, file) => {
  const cells = []
  const stages = new Set()
  for (const raw of env.text(file).split('\n')) {
    const m = raw.match(/^FROM\s+(?:--platform=\S+\s+)?(\S+)(?:\s+AS\s+(\S+))?/i)
    if (!m) continue
    if (stages.has(m[1])) continue
    if (m[2]) stages.add(m[2])
    const image = m[1].split('@')[0].split(':')[0]
    cells.push({ manager: 'dockerfile', packageFile: file, depName: image, depType: 'stage', datasource: 'docker' })
  }
  return cells
}

const extractMise = (env, file) => {
  const cells = []
  let inTools = false
  for (const raw of env.text(file).split('\n')) {
    const line = raw.trim()
    if (/^\[.*\]$/.test(line)) { inTools = line === '[tools]'; continue }
    if (!inTools || !line || line.startsWith('#')) continue
    const m = line.match(/^("?)([^"=\s]+)\1\s*=/)
    if (m) cells.push({ manager: 'mise', packageFile: file, depName: m[2], depType: 'tool', datasource: null })
  }
  return cells
}

const extractBun = (env, file, tracked) => {
  const dir = file.replace(/package\.json$/, '')
  if (!tracked.includes(`${dir}bun.lock`)) return []
  const manifest = env.json(file)
  const cells = []
  for (const section of ['dependencies', 'devDependencies', 'peerDependencies', 'optionalDependencies']) {
    for (const depName of Object.keys(manifest[section] ?? {})) cells.push({ manager: 'bun', packageFile: file, depName, depType: section, datasource: 'npm' })
  }
  cells.push({ manager: 'bun', packageFile: file, depName: null, depType: null, datasource: null, lockfile: true })
  return cells
}

const extractPreCommit = (env, file) => {
  const cells = []
  for (const repo of env.yaml(file).repos ?? []) {
    const m = String(repo.repo ?? '').match(/^https:\/\/github\.com\/([^/]+\/[^/]+?)(?:\.git)?$/)
    if (m) cells.push({ manager: 'pre-commit', packageFile: file, depName: m[1], depType: 'repository', datasource: 'github-tags' })
  }
  return cells
}

const extractGithubActions = (env, file) => {
  const cells = []
  for (const raw of env.text(file).split('\n')) {
    const uses = raw.match(/^\s*-?\s*uses:\s*["']?([^@\s"']+)@/)
    if (uses && !uses[1].startsWith('./') && !uses[1].startsWith('docker://')) {
      const [owner, repo] = uses[1].split('/')
      cells.push({ manager: 'github-actions', packageFile: file, depName: `${owner}/${repo}`, depType: 'action', datasource: 'github-tags' })
    }
    const runner = raw.match(/^\s*runs-on:\s*["']?([a-z]+)-([0-9.]+|latest)/)
    if (runner) cells.push({ manager: 'github-actions', packageFile: file, depName: runner[1], depType: 'github-runner', datasource: 'github-runners' })
  }
  return cells
}

const extractRegex = (env, manager, tracked) => {
  const cells = []
  const filePatterns = (manager.managerFilePatterns ?? []).map((p) => {
    const m = p.match(/^\/(.*)\/$/)
    if (!m) throw new Error(`custom manager file pattern '${p}' is not in the /regex/ form this check models`)
    return new RegExp(m[1])
  })
  for (const file of tracked.filter((f) => filePatterns.some((re) => re.test(f)))) {
    const content = env.text(file)
    for (const matchString of manager.matchStrings ?? []) {
      for (const m of content.matchAll(new RegExp(matchString, 'g'))) {
        const depName = m.groups?.depName ?? manager.depNameTemplate
        if (!depName || /\{\{/.test(depName)) throw new Error(`custom manager match in ${file} yields no literal depName; depNameTemplate is not modelled`)
        const datasource = m.groups?.datasource ?? (manager.datasourceTemplate && !/\{\{/.test(manager.datasourceTemplate) ? manager.datasourceTemplate : null)
        cells.push({ manager: 'custom.regex', packageFile: file, depName, depType: 'regex', datasource })
      }
    }
  }
  return cells
}

const extractRenovateConfig = (env, file) => {
  const cells = []
  for (const ref of env.json(file).extends ?? []) {
    const m = ref.match(/^github>([^/]+\/[^/:#]+)/)
    if (m) cells.push({ manager: 'renovate-config', packageFile: file, depName: m[1], depType: 'preset', datasource: 'github-tags' })
  }
  return cells
}

const occupancy = (env) => {
  const tracked = env.tracked()
  const cells = []
  const unmodelled = []
  for (const file of tracked) {
    if (/(^|\/)node_modules\//.test(file)) continue
    for (const [re, manager] of UNMODELLED_OCCUPANCY) if (re.test(file)) unmodelled.push(`${file} (${manager})`)
    if (/(^|\/)go\.mod$/.test(file)) cells.push(...extractGomod(env, file))
    if (DOCKERFILE.test(file)) cells.push(...extractDockerfile(env, file))
    if (/(^|\/)\.?mise\.toml$/.test(file)) cells.push(...extractMise(env, file))
    if (/(^|\/)package\.json$/.test(file)) cells.push(...extractBun(env, file, tracked))
    if (/^\.pre-commit-config\.ya?ml$/.test(file)) cells.push(...extractPreCommit(env, file))
    if (WORKFLOW.test(file)) cells.push(...extractGithubActions(env, file))
    if (file === '.github/renovate.json') cells.push(...extractRenovateConfig(env, file))
  }
  if (tracked.includes('mise.lock')) cells.push({ manager: 'mise', packageFile: 'mise.toml', depName: null, depType: null, datasource: null, lockfile: true })
  for (const manager of env.json('.github/renovate.json').customManagers ?? []) {
    if (manager.customType !== 'regex') throw new Error(`custom manager of type '${manager.customType}' is not modelled`)
    cells.push(...extractRegex(env, manager, tracked))
  }
  return { cells, unmodelled }
}

// ---------------------------------------------------------------------------------------------
// Config: the resolution chain, the sites that can reach a header, and the fold per cell.

const loadSources = async (env) => {
  const root = env.json('.github/renovate.json')
  const presets = []
  const builtins = []
  const unknown = []
  for (const ref of root.extends ?? []) {
    const shared = ref.match(/^github>ppat\/renovate-presets(?::([\w-]+))?#(.+)$/)
    if (shared) {
      const [, name = 'default', tag] = shared
      const body = env.offlinePresets
        ? readFileSync(join(env.offlinePresets, `${name}.json`), 'utf8')
        : await env.fetchText(`https://raw.githubusercontent.com/ppat/renovate-presets/${tag}/${name}.json`)
      presets.push({ name: ref, config: JSON.parse(body), local: false })
    } else if (INERT_BUILTINS.includes(ref)) {
      builtins.push(ref)
    } else {
      unknown.push(ref)
    }
  }
  // Renovate merges the extends entries left to right, then the extending config on top, so the
  // root config resolves last and its packageRules accumulate last and win per field.
  return { sources: [...presets, { name: '.github/renovate.json', config: root, local: true }], builtins, unknown }
}

const nestedExtends = (node, path) => {
  if (Array.isArray(node)) return node.flatMap((v, i) => nestedExtends(v, `${path}[${i}]`))
  if (!node || typeof node !== 'object') return []
  return Object.entries(node).flatMap(([k, v]) => (k === 'extends' ? [`${path}.${k}`] : nestedExtends(v, `${path}.${k}`)))
}

const matcherKeys = (rule) => Object.keys(rule).filter((k) => k.startsWith('match'))

const walkSites = (sources) => {
  const sites = []
  const walk = (node, ctx, path) => {
    if (Array.isArray(node)) return node.forEach((v, i) => walk(v, ctx, `${path}[${i}]`))
    if (!node || typeof node !== 'object') return
    for (const [key, value] of Object.entries(node)) {
      if (HEADER_SHAPED.test(key)) sites.push({ ...ctx, path: `${path}.${key}`, key, value })
      walk(value, ctx, `${path}.${key}`)
    }
  }
  for (const { name, config, local } of sources) walk(config, { source: name, local }, name)
  return sites
}

// Renovate's package-name matching: a `/regex/` entry, a `*` that matches anything, or a glob,
// compared without case. Entries prefixed `!` exclude, and when no positive entry exists the
// negatives alone decide.
const globToRegex = (glob) => new RegExp(`^${glob.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*\*/g, '\u0000').replace(/\*/g, '[^/]*').replace(/\u0000/g, '.*').replace(/\?/g, '[^/]')}$`, 'i')
const matchesPattern = (value, pattern) => {
  const re = pattern.match(/^\/(.*)\/([gimsuy]*)$/)
  if (re) return new RegExp(re[1], re[2].replace('g', '')).test(value)
  if (pattern === '*') return true
  return globToRegex(pattern).test(value)
}
const matchesList = (value, patterns) => {
  if (value === null || value === undefined) return false
  const positives = patterns.filter((p) => !p.startsWith('!'))
  const negatives = patterns.filter((p) => p.startsWith('!')).map((p) => p.slice(1))
  const positive = positives.length === 0 || positives.some((p) => matchesPattern(value, p))
  return positive && !negatives.some((p) => matchesPattern(value, p))
}
const matchesFile = (file, patterns) => patterns.some((p) => {
  const re = p.match(/^\/(.*)\/$/)
  if (re) return new RegExp(re[1]).test(file)
  if (/[{}]/.test(p)) throw new Error(`matchFileNames pattern '${p}' uses braces, which this check does not model`)
  return globToRegex(p.replace(/^\*\*\//, '')).test(file) || (p.startsWith('**/') && globToRegex(p).test(file))
})

// Whether a rule matches a cell under an update type: true, false, or 'unknown' when a matcher the
// cell cannot answer (a datasource this extractor does not assign) is the only thing undecided.
const ruleMatches = (rule, cell, updateType) => {
  let unknown = false
  for (const key of matcherKeys(rule)) {
    if (!MODELLED_MATCHERS.includes(key)) throw new Error(`matcher ${key} is not modelled`)
    const list = rule[key]
    let verdict
    if (key === 'matchManagers') verdict = list.includes(cell.manager)
    else if (key === 'matchFileNames') verdict = matchesFile(cell.packageFile, list)
    else if (key === 'matchDepNames' || key === 'matchPackageNames') verdict = matchesList(cell.depName, list)
    else if (key === 'matchDepTypes') verdict = cell.depType !== null && list.includes(cell.depType)
    else if (key === 'matchUpdateTypes') verdict = list.includes(updateType)
    else if (key === 'matchDatasources') verdict = cell.datasource === null ? 'unknown' : list.includes(cell.datasource)
    if (verdict === false) return false
    if (verdict === 'unknown') unknown = true
  }
  return unknown ? 'unknown' : true
}

const HANDLEBARS = /\{\{#if [^}]*\}\}|\{\{\/if\}\}|\{\{\{?semanticCommit(?:Type|Scope)\}?\}\}/g
const LITERAL_HEADER = /^([a-z]+)(?:\(([^)]*)\))?(!)?:$/

const classifyPrefix = (value) => {
  if (value === '') return { kind: 'unset' }
  if (/^[()!:\s]*$/.test(value.replace(HANDLEBARS, ''))) return { kind: 'template', breaking: value.includes('!') }
  const m = value.match(LITERAL_HEADER)
  if (m) return { kind: 'literal', type: m[1], scope: m[2] ?? '', breaking: Boolean(m[3]) }
  return { kind: 'unmodelled' }
}

const renderPrefix = (template, type, scope) => template
  .replace(/\{\{#if semanticCommitScope\}\}(.*?)\{\{\/if\}\}/g, (_, body) => (scope ? body : ''))
  .replace(/\{\{\{?semanticCommitType\}?\}\}/g, type)
  .replace(/\{\{\{?semanticCommitScope\}?\}\}/g, scope)

const renderGroup = (template, cell) => {
  if (!template) return null
  const rendered = template.replace(/\{\{\{?manager\}?\}\}/g, cell.manager).replace(/\{\{\{?depType\}?\}\}/g, cell.depType ?? '')
  if (/\{\{/.test(rendered)) throw new Error(`groupName template '${template}' uses a construct this check does not render`)
  return rendered
}

// Fold every header-carrying field down to what one cell emits under one update type, applying
// top-level values then packageRules in resolution order, later winning per field.
const fold = (sources, cell, updateType) => {
  const out = { type: 'chore', scope: '', prefix: '', semanticCommits: undefined, group: null, scopeClaim: null, unknown: [] }
  for (const { name, config, local } of sources) {
    if (config.semanticCommitType !== undefined) out.type = config.semanticCommitType
    if (config.semanticCommitScope !== undefined) { out.scope = config.semanticCommitScope; out.scopeClaim = { where: `${name} (top level)`, local } }
    if (config.commitMessagePrefix !== undefined) out.prefix = config.commitMessagePrefix
    if (config.semanticCommits !== undefined) out.semanticCommits = config.semanticCommits
  }
  for (const { name, config, local } of sources) {
    for (const [i, rule] of (config.packageRules ?? []).entries()) {
      const verdict = ruleMatches(rule, cell, updateType)
      if (verdict === false) continue
      if (verdict === 'unknown') { out.unknown.push(`${name} packageRules[${i}]`); continue }
      if (rule.semanticCommitType !== undefined) out.type = rule.semanticCommitType
      if (rule.semanticCommitScope !== undefined) { out.scope = rule.semanticCommitScope; out.scopeClaim = { where: `${name} packageRules[${i}]`, local } }
      if (rule.commitMessagePrefix !== undefined) out.prefix = rule.commitMessagePrefix
      if (rule.semanticCommits !== undefined) out.semanticCommits = rule.semanticCommits
      if (rule.groupName !== undefined) out.group = renderGroup(rule.groupName, cell)
    }
  }
  return out
}

const headerOf = (folded) => {
  const prefix = classifyPrefix(folded.prefix)
  if (prefix.kind === 'unmodelled') throw new Error(`commitMessagePrefix '${folded.prefix}' is neither a semanticCommit* template nor a literal header`)
  if (prefix.kind === 'literal') return folded.prefix
  if (prefix.kind === 'unset') return `${folded.type}${folded.scope ? `(${folded.scope})` : ''}:`
  return renderPrefix(folded.prefix, folded.type, folded.scope)
}

const updateTypesOf = (cell) => (cell.lockfile ? LOCKFILE_UPDATE_TYPES : PACKAGE_UPDATE_TYPES)

// The whole emittable set: every cell under every update type it can take, with provenance.
const emission = async (env) => {
  const { sources, builtins, unknown } = await loadSources(env)
  const { cells, unmodelled } = occupancy(env)
  const rows = []
  const refusals = []
  for (const cell of cells) {
    for (const updateType of updateTypesOf(cell)) {
      let folded
      try { folded = fold(sources, cell, updateType) } catch (e) { refusals.push(`${cell.manager} ${cell.packageFile} ${cell.depName ?? 'lockfile'} (${updateType}): ${e.message}`); continue }
      if (folded.unknown.length) { refusals.push(`${cell.manager} ${cell.packageFile} ${cell.depName ?? 'lockfile'} (${updateType}): ${folded.unknown.join(', ')} cannot be decided for this cell, because its datasource is not modelled`); continue }
      let header
      try { header = headerOf(folded) } catch (e) { refusals.push(`${cell.manager} ${cell.packageFile} ${cell.depName ?? 'lockfile'} (${updateType}): ${e.message}`); continue }
      rows.push({ cell, updateType, folded, header })
    }
  }
  return { sources, builtins, unknown, cells, unmodelled, rows, refusals }
}

const describeCell = (row) => `${row.cell.manager} ${row.cell.packageFile}${row.cell.depName ? ` ${row.cell.depName}` : ' (lock file)'} ${row.updateType}`

// ---------------------------------------------------------------------------------------------
// The shipped boundary, read from the release workflow: what each image's Dockerfile copies in,
// the Dockerfile itself, and the directory the chart is packaged from.

const shippedPrefixes = (env) => {
  const release = env.yaml('.github/workflows/release.yaml')
  const images = release.jobs?.images?.strategy?.matrix?.image
  if (!Array.isArray(images) || !images.length) throw new Error('release.yaml names no image matrix, so the shipped boundary cannot be read')
  const prefixes = new Set()
  for (const image of images) {
    const dockerfile = `${image}/Dockerfile`
    prefixes.add(dockerfile)
    for (const raw of env.text(dockerfile).split('\n')) {
      const m = raw.match(/^COPY\s+(.*)$/)
      if (!m || /--from=/.test(m[1])) continue
      const parts = m[1].trim().split(/\s+/).filter((p) => !p.startsWith('--'))
      for (const src of parts.slice(0, -1)) prefixes.add(src.replace(/\/$/, ''))
    }
  }
  const chart = JSON.stringify(release).match(/helm package (\S+)/)
  if (!chart) throw new Error('release.yaml packages no chart, so the shipped boundary cannot be read')
  prefixes.add(chart[1])
  return [...prefixes]
}

const isShippedPath = (path, prefixes) => footprintOf(path) === '' && prefixes.some((p) => path === p || path.startsWith(`${p}/`))

// A cell ships when its file ships and it is not a build-time dependency of a shipped file. A bun
// devDependency is installed for the build and never enters the bundle.
const cellShips = (cell, prefixes) => isShippedPath(cell.packageFile, prefixes) && !(cell.manager === 'bun' && cell.depType === 'devDependencies')

// ---------------------------------------------------------------------------------------------

const closure = async (env) => {
  const fail = []
  const note = []
  const e = await emission(env)
  const rules = env.commitlint().rules
  const types = rules['type-enum'][2]
  const scopes = rules['scope-enum'][2]

  for (const ref of e.unknown) fail.push(`extends entry '${ref}' is neither a pinned ppat/renovate-presets file nor a built-in preset known to set no header field. classify it before trusting anything below`)
  for (const { name, config } of e.sources) {
    for (const [key, value] of Object.entries(config)) {
      if (key === 'extends') continue
      for (const at of nestedExtends(value, `${name}.${key}`)) fail.push(`a preset is extended at ${at}. only a source's top-level extends is composed here, so the rules it pulls in would go unread`)
    }
    if (!name.startsWith('.github/') && config.extends?.length) fail.push(`${name} extends ${config.extends.join(', ')}. a preset's own extends chain is not composed here`)
  }

  const sites = walkSites(e.sources)
  for (const site of sites) {
    const where = site.local ? fail : note
    const suffix = site.local ? '' : ' (upstream: emitted unless a local rule overrides it, which the fold below decides)'
    if (!HEADER_KEYS.includes(site.key) && !HEADER_SWITCHES.includes(site.key)) {
      if (!SUBJECT_ONLY_KEYS.includes(site.key)) fail.push(`'${site.key}' at ${site.path} can reach a commit header or a pull request title and this check does not model it. classify it before trusting anything below`)
      continue
    }
    if (site.key === 'semanticCommits' || typeof site.value !== 'string') continue
    if (site.key === 'semanticCommitType' && !types.includes(site.value)) where.push(`type '${site.value}' at ${site.path} is not in type-enum${suffix}`)
    if (site.key === 'semanticCommitScope' && !scopes.includes(site.value)) where.push(`scope '${site.value}' at ${site.path} is not in scope-enum${suffix}`)
    if (site.key === 'commitMessagePrefix' && classifyPrefix(site.value).kind === 'unmodelled') fail.push(`commitMessagePrefix '${site.value}' at ${site.path} is neither a semanticCommit* template nor a literal header, so it cannot be checked`)
  }

  for (const { name, config } of e.sources.filter((s) => s.local)) {
    for (const [i, rule] of (config.packageRules ?? []).entries()) {
      const unmodelled = matcherKeys(rule).filter((k) => !MODELLED_MATCHERS.includes(k))
      if (unmodelled.length) fail.push(`${name} packageRules[${i}] uses ${unmodelled.join(', ')}, which the fold does not model`)
    }
  }
  for (const item of e.unmodelled) fail.push(`${item} is read by a Renovate manager this check does not model, so its headers would be emitted and never enumerated here`)
  for (const refusal of e.refusals) fail.push(refusal)

  const seen = new Map()
  for (const row of e.rows) {
    if (row.folded.semanticCommits !== 'enabled') { fail.push(`${describeCell(row)}: semanticCommits resolves to '${row.folded.semanticCommits ?? 'unset'}', so Renovate emits no type(scope) prefix and the header fails the gate`). continue }
    if (!seen.has(row.header)) seen.set(row.header, [])
    seen.get(row.header).push(describeCell(row))
  }
  // Both a normal subject and a degenerate one: an update carrying no version variables renders
  // commitMessageExtra's placeholders as empty.
  for (const [prefix, provenance] of seen) {
    for (const subject of ['update something (1.2.3 -> 1.2.4)', 'update lockfile bun ( -> )']) {
      const verdict = env.lint(`${prefix} ${subject}`)
      if (!verdict.accepted) fail.push(`'${prefix} ${subject}' is emittable (${provenance[0]}${provenance.length > 1 ? ` and ${provenance.length - 1} more` : ''}), and commitlint rejects it:\n${verdict.output}`)
    }
  }

  // Scope-carrying sites outside Renovate: release-please composes its own pull request titles, and
  // with squash merges the title is what lands on main.
  const releasePlease = env.json('release-please-config.json')
  const placeholders = { component: '', version: '1.2.3', scope: '', branch: 'main' }
  for (const key of Object.keys(releasePlease).filter((k) => k.endsWith('title-pattern'))) {
    const rendered = releasePlease[key].replace(/\$\{([a-zA-Z]+)\}/g, (_, name) => placeholders[name] ?? 'x')
    const verdict = env.lint(rendered)
    if (!verdict.accepted) fail.push(`release-please's ${key} renders '${rendered}', which commitlint rejects:\n${verdict.output}`)
    else note.push(`release-please ${key} renders: ${rendered}`)
  }

  note.push(`${e.cells.length} dependency cells across ${new Set(e.cells.map((c) => c.manager)).size} managers, ${e.rows.length} cell-by-update-type rows, ${e.sources.length} config sources. built-ins assumed inert: ${e.builtins.join(', ') || 'none'}`)
  note.push(`emittable prefixes: ${[...seen.keys()].join(' | ')}`)
  return { fail, note }
}

// ---------------------------------------------------------------------------------------------

// Emission truth: the header a cell renders must be a true claim about what the cell's file does.
const truth = async (env) => {
  const fail = []
  const note = []
  const e = await emission(env)
  for (const refusal of e.refusals) fail.push(refusal)
  const prefixes = shippedPrefixes(env)

  // A cell rides a group, and the group's header is one header for the whole branch, so a cell in an
  // unshipped file may carry a shipped claim when a shipped cell shares its group. That is the
  // grouped-branch reality made explicit, not a loophole: the go pin in mise.toml rides the go group.
  const groupShips = new Map()
  for (const row of e.rows) {
    if (row.folded.group === null) continue
    if (cellShips(row.cell, prefixes)) groupShips.set(row.folded.group, true)
    else if (!groupShips.has(row.folded.group)) groupShips.set(row.folded.group, false)
  }

  let shippedCells = 0
  let internalCells = 0
  for (const row of e.rows) {
    const parsed = parseHeader(`${row.header} x`)
    if (!parsed) { fail.push(`${describeCell(row)}: rendered header '${row.header}' does not parse`). continue }
    const ships = cellShips(row.cell, prefixes) || (row.folded.group !== null && groupShips.get(row.folded.group) === true)
    if (ships) shippedCells++; else internalCells++
    if (!ships && (CLAIM_TYPES.includes(parsed.type) || parsed.breaking)) {
      fail.push(`${describeCell(row)} renders '${row.header}', a claim that a shipped artifact changed, but ${row.cell.packageFile} reaches no consumer${row.cell.depType === 'devDependencies' ? ' (a devDependency)' : ''}`)
    }
    if (ships && VERSION_MOVING_UPDATE_TYPES.includes(row.updateType) && !CLAIM_TYPES.includes(parsed.type)) {
      fail.push(`${describeCell(row)} renders '${row.header}', a hidden type, but ${row.cell.packageFile} ships and a ${row.updateType} update moves what it builds: the release would be silent`)
    }
    if (ships && row.updateType === 'major' && !parsed.breaking) {
      fail.push(`${describeCell(row)} renders '${row.header}' without a breaking marker. a major of a shipped dependency carries one, as the read-before-deploying signal`)
    }
    if (ships && row.updateType !== 'major' && parsed.breaking) {
      fail.push(`${describeCell(row)} renders '${row.header}' with a breaking marker on a ${row.updateType} update`)
    }
    if (!row.folded.scopeClaim || !row.folded.scopeClaim.local) {
      fail.push(`${describeCell(row)}: its scope '${row.folded.scope}' is ${row.folded.scopeClaim ? `claimed by ${row.folded.scopeClaim.where}, a shared preset` : 'claimed by nothing'}. a preset bump could then move this header with no local change, so the claim must be restated in this repository's own config`)
    }
  }
  note.push(`shipped: ${[...prefixes].join(', ')}`)
  note.push(`${shippedCells} shipped rows, ${internalCells} internal rows. groups: ${[...groupShips].map(([g, s]) => `${g}=${s ? 'ships' : 'internal'}`).join(', ') || 'none'}`)
  return { fail, note }
}

// ---------------------------------------------------------------------------------------------

const selfConsistency = async (env) => {
  const fail = []
  const note = []
  const rules = env.commitlint().rules
  const types = rules['type-enum'][2]
  const scopes = rules['scope-enum'][2]

  const sections = env.json('release-please-config.json')['changelog-sections'].map((s) => s.type)
  for (const t of types) if (!sections.includes(t)) fail.push(`type '${t}' is legal but has no changelog-sections entry: a release window holding only that type renders nothing and cuts no release, silently and green`)
  for (const t of sections) if (!types.includes(t)) fail.push(`changelog-sections has an entry for type '${t}', which type-enum rejects: dead config`)

  // The literals commitlint.config.js's local rules reason over, read from source because they are
  // not exported. The naming convention is the seam, so a rename fails here rather than quietly
  // narrowing what gets checked.
  const source = env.commitlintSource()
  const consts = [...source.matchAll(/const (\w+_(SCOPES|TYPES|BY_TYPE))\s*=\s*([[{][\s\S]*?[\]}])\n/g)]
  if (!consts.length) fail.push('commitlint.config.js declares no *_SCOPES, *_TYPES or *_BY_TYPE constant. this cross-check has nothing to read and would pass vacuously')
  for (const [, name, kind, body] of consts) {
    const literals = [...body.matchAll(/'([^']*)'/g)].map((m) => m[1])
    const enumerated = kind === 'TYPES' ? types : kind === 'SCOPES' ? scopes : [...types, ...scopes]
    for (const literal of literals) {
      if (!enumerated.includes(literal)) fail.push(`commitlint.config.js's ${name} names '${literal}', which the enums reject: a predicate that can never fire`)
    }
    note.push(`${name}: ${literals.map((l) => l || "''").join(', ')}`)
  }

  const overlaps = []
  for (const path of env.tracked()) {
    const hits = namedFootprints(path)
    if (hits.length > 1) overlaps.push(`${path} -> ${hits.join(', ')}`)
  }
  if (overlaps.length) fail.push(`tracked file(s) fall in more than one named footprint, so the derived scope is ambiguous: ${overlaps.slice(0, 10).join('; ')}`)
  else note.push(`the named footprints do not overlap across ${env.tracked().length} tracked files`)

  // The two gates' own wiring. A skipped job reports `skipped`, and `skipped` SATISFIES a required
  // status check, and a job that never triggers never reports at all. Both make a required context
  // enforce nothing while looking enforced, so the shape is asserted from inside the job.
  const lint = env.yaml('.github/workflows/lint.yaml')
  const lintOn = lint.on ?? lint[true]
  const job = lint.jobs?.['commit-taxonomy']
  if (!job) fail.push('lint.yaml has no commit-taxonomy job, but branch protection requires the context it reports')
  else {
    if (job.needs) fail.push('the commit-taxonomy job has a needs:, so an upstream failure makes it report skipped, which satisfies the required context instead of blocking on it')
    if (job.if) fail.push('the commit-taxonomy job has an if:, so it can report skipped, which satisfies the required context instead of blocking on it')
  }
  for (const trigger of ['pull_request', 'workflow_dispatch', 'schedule']) {
    if (!(trigger in (lintOn ?? {}))) fail.push(`lint.yaml does not trigger on ${trigger}. the repository-state checks in this job go stale without it`)
  }
  if (lintOn?.pull_request?.paths || lintOn?.pull_request?.['paths-ignore']) fail.push('lint.yaml filters pull_request by paths, so this job does not report on every pull request and a required context waiting on it can never clear')
  const messages = lint.jobs?.['commit-messages']
  if (!messages) fail.push('lint.yaml has no commit-messages job, so branch commits are not linted')
  else {
    if (messages.needs) fail.push('the commit-messages job has a needs:, so an upstream failure makes it report skipped, which satisfies the required context')
    if (messages.if && messages.if.replace(/\s/g, '') !== "${{github.event_name=='pull_request'}}") fail.push(`the commit-messages job has the condition '${messages.if}'. the only condition it may carry is one true on every pull_request event, since that is the event a required context is judged on`)
  }

  const title = env.yaml('.github/workflows/pr-title.yaml')
  const titleOn = title.on ?? title[true]
  const titleJob = title.jobs?.['pr-title']
  if (!titleJob) fail.push('pr-title.yaml has no pr-title job, but branch protection requires the context it reports')
  else {
    if (titleJob.needs) fail.push('the pr-title job has a needs:, so it can report skipped, which satisfies the required context')
    if (titleJob.if) fail.push('the pr-title job has an if:, so it can report skipped, which satisfies the required context')
  }
  const titleTypes = titleOn?.pull_request?.types ?? []
  for (const type of ['opened', 'edited', 'synchronize', 'reopened']) {
    if (!titleTypes.includes(type)) fail.push(`pr-title.yaml does not run on pull_request ${type}. a title changed after the last push would go unlinted`)
  }
  if (titleOn?.pull_request?.paths || titleOn?.pull_request?.['paths-ignore']) fail.push('pr-title.yaml filters pull_request by paths, so the required context can never clear on a pull request that misses the filter')

  // One commitlint engine behind every door: the pre-commit hook pins the same version as the root
  // manifest the two gates and this check install.
  const manifest = env.json('package.json')
  const cli = manifest.devDependencies?.['@commitlint/cli']
  if (!cli) fail.push('package.json pins no @commitlint/cli, so the gates and this check resolve no engine of their own')
  const hookPin = env.text('.pre-commit-config.yaml').match(/@commitlint\/cli@([^'"\s]+)/)
  if (!hookPin) fail.push('.pre-commit-config.yaml pins no @commitlint/cli for the commit-msg hook')
  else if (hookPin[1] !== cli) fail.push(`the pre-commit commit-msg hook runs @commitlint/cli ${hookPin[1]} and the gates run ${cli}. a local commit and CI judge with different engines`)
  return { fail, note }
}

// ---------------------------------------------------------------------------------------------

const messageShape = async (env) => {
  const fail = []
  const note = []
  const types = env.commitlint().rules['type-enum'][2]

  // release-please parses the whole message and splits it on blank lines: a paragraph shaped like a
  // conventional commit becomes its own release entry, so a chore-headed commit can cut a minor.
  const paragraph = new RegExp(`(?:^|\\n)[ \\t]*\\n(?:[a-z]+(?:\\([^)]*\\))?!:|(?:${types.join('|')})(?:\\([^)]*\\))?:)[ \\t]`)
  for (const commit of env.commits()) {
    const short = commit.sha.slice(0, 8)
    if (paragraph.test(commit.message)) fail.push(`${short}: a body paragraph is shaped like a conventional commit header, so the release parser reads it as a second commit and can size the release off it`)
    if (/^[ \t]*Release-As:[ \t]*\S/im.test(commit.message)) fail.push(`${short}: a Release-As: footer overrides the computed version outright`)
    if (commit.message.includes('BEGIN_COMMIT_OVERRIDE')) fail.push(`${short}: a BEGIN_COMMIT_OVERRIDE block replaces the release-facing message wholesale`)
  }
  if (env.prBody().includes('BEGIN_COMMIT_OVERRIDE')) fail.push('the pull request body carries a BEGIN_COMMIT_OVERRIDE block, which replaces the release-facing message wholesale')

  const pinned = env.text('.github/workflows/release.yaml').match(/ppat\/github-workflows\/[^@]+@([0-9a-f]{40})/)
  if (!pinned) {
    fail.push('cannot find the pinned release workflow in release.yaml, so the parser behind the patterns above is unknown')
  } else {
    const remote = await env.fetchText(`https://raw.githubusercontent.com/ppat/github-workflows/${pinned[1]}/.github/workflows/release-please.yaml`)
    const version = remote.match(/RELEASE_PLEASE_VERSION:\s*"([^"]+)"/)
    if (!version) fail.push('the pinned release workflow declares no RELEASE_PLEASE_VERSION, so which parser the patterns above describe is unknown')
    else if (Number(version[1].split('.')[0]) !== RELEASE_PLEASE_MAJOR) fail.push(`the release parser is now ${version[1]}, past the ${RELEASE_PLEASE_MAJOR}.x the patterns above were derived from: re-derive them, then move RELEASE_PLEASE_MAJOR`)
    else note.push(`release parser ${version[1]} is within the ${RELEASE_PLEASE_MAJOR}.x the patterns were derived from`)
  }
  note.push(`${env.commits().length} commit(s) inspected`)
  return { fail, note }
}

// ---------------------------------------------------------------------------------------------

// The empty scope claims the change ships or is repository-level. commitlint sees the header and
// never the diff, so the half it cannot check is here: a claim type or a breaking marker on the
// empty scope must touch at least one path that ships.
const emptyScope = async (env) => {
  const fail = []
  const note = []
  const prefixes = shippedPrefixes(env)
  for (const commit of env.commits()) {
    const parsed = parseHeader(commit.message)
    if (!parsed || parsed.scope !== '') continue
    const short = commit.sha.slice(0, 8)
    const shipped = commit.paths.filter((p) => isShippedPath(p, prefixes))
    if ((CLAIM_TYPES.includes(parsed.type) || parsed.breaking) && !shipped.length) {
      fail.push(`${short}: type '${parsed.type}'${parsed.breaking ? ' with a breaking marker' : ''} asserts a shipped artifact changed, but no changed path ships (what ships is what an image's Dockerfile copies in, the Dockerfiles, and the chart). commitlint accepts this because it reads the header and never the diff. Type it chore, ci, docs or test.`)
    }
    note.push(`${short}: empty scope, ${shipped.length} of ${commit.paths.length} changed path(s) ship`)
  }
  return { fail, note }
}

// ---------------------------------------------------------------------------------------------

// ADVISORY, with a sunset: review the fire log after 30 merged pull requests carrying named scopes.
// Promote to required if it has produced at least one true positive and no false positives, and
// delete it if it is noise-only. A check left advisory indefinitely spends the audit attention it
// asks for and buys nothing.
//
// Two strengths, following the scope table. The line-level scopes say a version moved and nothing
// else did, so every changed path must sit inside their footprint. The path scopes take a diff
// scoped to what motivated it, which may carry paths outside the footprint (a rule document beside
// the workflow that enforces it), so requiring every path inside would fire on the rule document's
// own worked example. What no named scope survives is a diff touching nothing in its footprint.
const STRICT_SCOPES = ['release', 'renovate', 'github-actions', 'internal-dependencies']
const namedScope = async (env) => {
  const fail = []
  const note = []
  for (const commit of env.commits()) {
    const parsed = parseHeader(commit.message)
    if (!parsed || !parsed.scope) continue
    const short = commit.sha.slice(0, 8)
    const condition = SCOPE_PATH_CONDITIONS[parsed.scope]
    if (!condition) { note.push(`${short}: scope '${parsed.scope}' states no condition a diff can witness`). continue }
    const inside = commit.paths.filter((p) => condition.some((re) => re.test(p)))
    const stray = commit.paths.filter((p) => !condition.some((re) => re.test(p)))
    if (!inside.length) fail.push(`${short}: scope '${parsed.scope}' does not cover ${commit.paths.slice(0, 5).join(', ')}${commit.paths.length > 5 ? ` and ${commit.paths.length - 5} more` : ''}, and no changed path is in its footprint: scope it to what motivated the change`)
    else if (STRICT_SCOPES.includes(parsed.scope) && stray.length) fail.push(`${short}: scope '${parsed.scope}' means a version moved and nothing else did, but the diff also changes ${stray.slice(0, 5).join(', ')}${stray.length > 5 ? ` and ${stray.length - 5} more` : ''}: a hand edit beside a pin bump takes the surface's own scope`)
    else note.push(`${short}: scope '${parsed.scope}' covers ${inside.length} of ${commit.paths.length} changed path(s)`)
  }
  return { fail, note }
}

export const CHECKS = {
  closure,
  truth,
  'self-consistency': selfConsistency,
  'message-shape': messageShape,
  'empty-scope': emptyScope,
  'named-scope': namedScope,
}

export const internals = { FOOTPRINTS, classifyPrefix, emission, fold, footprintOf, occupancy, parseHeader, renderPrefix, shippedPrefixes, ruleMatches }
