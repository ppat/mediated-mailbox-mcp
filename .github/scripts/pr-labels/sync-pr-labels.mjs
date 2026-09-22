#!/usr/bin/env node
// Sets a pull request's `component:` labels to exactly the components its diff touches, creating a
// missing label in the component color. Run by the `pr-labels` workflow from a checkout of the pull
// request's head, so the component table read is the one the pull request itself carries.
//
//   sync-pr-labels.mjs            reads GITHUB_TOKEN, GITHUB_REPOSITORY, PR_NUMBER, BASE_SHA, HEAD_SHA
//   sync-pr-labels.mjs --dry-run  prints the labels the diff calls for and calls no API
//
// The diff is the merge-base diff GitHub shows as the pull request's changed files, taken with
// renames split into a deletion and an addition, so a file moved out of a component still labels
// the component it left.

import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { LABEL_COLOR, labelChanges, parseComponents, wantedLabels } from './labels.mjs'

const dryRun = process.argv.includes('--dry-run')
const env = (name) => {
  const value = process.env[name]
  if (!value) throw new Error(`${name} is not set`)
  return value
}

const base = dryRun ? (process.env.BASE_SHA ?? 'origin/main') : env('BASE_SHA')
const head = dryRun ? (process.env.HEAD_SHA ?? 'HEAD') : env('HEAD_SHA')
const paths = execFileSync('git', ['diff', '--no-renames', '--name-only', `${base}...${head}`], { encoding: 'utf8' })
  .split('\n').filter(Boolean)
const directories = parseComponents(readFileSync('CLAUDE.md', 'utf8'))
const wanted = wantedLabels(paths, directories)

console.log(`${paths.length} changed paths, ${directories.length} components in CLAUDE.md`)
console.log(`component labels the diff calls for: ${wanted.length ? wanted.join(', ') : '(none)'}`)
if (dryRun) process.exit(0)

const repository = env('GITHUB_REPOSITORY')
const number = env('PR_NUMBER')
const token = env('GITHUB_TOKEN')

async function github(method, path, body) {
  const response = await fetch(`https://api.github.com/repos/${repository}${path}`, {
    method,
    headers: {
      accept: 'application/vnd.github+json',
      authorization: `Bearer ${token}`,
      'x-github-api-version': '2022-11-28',
      ...(body ? { 'content-type': 'application/json' } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  const text = await response.text()
  return { status: response.status, json: text ? JSON.parse(text) : null, text }
}

async function expect(method, path, body, ok) {
  const response = await github(method, path, body)
  if (!ok.includes(response.status)) {
    throw new Error(`${method} ${path} returned ${response.status}: ${response.text}`)
  }
  return response
}

const labelPath = (name) => encodeURIComponent(name)

// A pull request carries far fewer than a hundred labels, so one page is all of them. More than
// that is refused rather than read as complete.
const listed = await expect('GET', `/issues/${number}/labels?per_page=100`, null, [200])
if (listed.json.length === 100) throw new Error('the pull request carries 100 or more labels, more than one page reads')
const current = listed.json.map((l) => l.name)
const { add, remove } = labelChanges(current, wanted)

for (const name of add) {
  const existing = await expect('GET', `/labels/${labelPath(name)}`, null, [200, 404])
  if (existing.status === 404) {
    // A 422 whose error is already_exists is another run creating the same label first. Any other
    // 422 is a refused label and fails the run.
    const created = await expect('POST', '/labels', { name, color: LABEL_COLOR }, [201, 422])
    if (created.status === 422 && !created.json?.errors?.some((e) => e.code === 'already_exists')) {
      throw new Error(`POST /labels refused ${name}: ${created.text}`)
    }
    console.log(created.status === 201 ? `created label ${name}` : `label ${name} was created concurrently`)
  }
}
if (add.length) await expect('POST', `/issues/${number}/labels`, { labels: add }, [200])
// 404 is the label already gone from the pull request, which is the state wanted.
for (const name of remove) await expect('DELETE', `/issues/${number}/labels/${labelPath(name)}`, null, [200, 404])

console.log(`added: ${add.length ? add.join(', ') : '(none)'}`)
console.log(`removed: ${remove.length ? remove.join(', ') : '(none)'}`)
