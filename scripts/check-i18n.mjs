#!/usr/bin/env node
// check-i18n.mjs — compare translation keys between locale files.
// Reports missing keys in any language relative to the reference (en.json).
// Updates the <!-- i18n:start --> section in README.md with a status table.
//
// Usage:
//   node scripts/check-i18n.mjs          # exits 1 on missing keys
//   node scripts/check-i18n.mjs --warn   # always exits 0 (CI warning mode)

import { readFileSync, writeFileSync, readdirSync, existsSync } from 'fs'
import { join, basename } from 'path'

const ROOT = join(import.meta.dirname, '..')
const LOCALES_DIR = join(ROOT, 'frontend', 'src', 'locales')
const README = join(ROOT, 'README.md')
const warnOnly = process.argv.includes('--warn')

// ── Flatten keys ────────────────────────────────────────────────────────────

function flattenKeys(obj, prefix = '') {
  const keys = []
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flattenKeys(v, path))
    } else {
      keys.push(path)
    }
  }
  return keys
}

// ── Load locales ────────────────────────────────────────────────────────────

const files = readdirSync(LOCALES_DIR).filter(f => f.endsWith('.json')).sort()
if (files.length < 2) {
  console.log('Need at least 2 locale files to compare.')
  process.exit(0)
}

const locales = {}
for (const file of files) {
  const lang = basename(file, '.json')
  const data = JSON.parse(readFileSync(join(LOCALES_DIR, file), 'utf8'))
  locales[lang] = new Set(flattenKeys(data))
}

const reference = 'en'
const refKeys = locales[reference]
if (!refKeys) {
  console.error(`Reference locale "${reference}" not found.`)
  process.exit(1)
}

// ── Compare ─────────────────────────────────────────────────────────────────

let hasErrors = false
const results = []

for (const [lang, keys] of Object.entries(locales)) {
  const missing = [...refKeys].filter(k => !keys.has(k))
  const extra = [...keys].filter(k => !refKeys.has(k))
  const translated = [...refKeys].filter(k => keys.has(k)).length
  const pct = refKeys.size > 0 ? Math.round((translated / refKeys.size) * 100) : 100

  results.push({ lang, total: keys.size, missing: missing.length, extra: extra.length, pct })

  if (lang === reference) continue

  if (missing.length > 0) {
    hasErrors = true
    console.log(`\n${lang}.json — missing ${missing.length} key(s):`)
    for (const k of missing) console.log(`  - ${k}`)
  }

  if (extra.length > 0) {
    console.log(`\n${lang}.json — ${extra.length} extra key(s) not in ${reference}:`)
    for (const k of extra) console.log(`  + ${k}`)
  }

  if (missing.length === 0 && extra.length === 0) {
    console.log(`${lang}.json — all ${keys.size} keys match ${reference}.json`)
  }
}

console.log(`\nReference: ${reference}.json (${refKeys.size} keys)`)
console.log(`Locales: ${files.map(f => basename(f, '.json')).join(', ')}`)

// ── Update README ───────────────────────────────────────────────────────────

if (existsSync(README)) {
  const readme = readFileSync(README, 'utf8')
  const startMarker = '<!-- i18n:start -->'
  const endMarker = '<!-- i18n:end -->'
  const start = readme.indexOf(startMarker)
  const end = readme.indexOf(endMarker)

  if (start !== -1 && end !== -1) {
    const date = new Date().toISOString().slice(0, 10)

    const langNames = { en: 'English', es: 'Español' }

    let table = '## Translations\n\n'
    table += '| Language | Keys | Coverage |\n'
    table += '|----------|------|----------|\n'

    for (const r of results) {
      const name = langNames[r.lang] || r.lang
      let color
      if (r.pct >= 100) color = 'brightgreen'
      else if (r.pct >= 80) color = 'green'
      else if (r.pct >= 50) color = 'yellow'
      else color = 'red'

      const badge = `![${r.pct}%](https://img.shields.io/badge/translated-${r.pct}%25-${color})`
      const status = r.missing > 0 ? `${r.total} (${r.missing} missing)` : `${r.total}`
      table += `| ${name} | ${status} | ${badge} |\n`
    }

    table += `\n_Last updated: ${date}_\n`

    const updated =
      readme.slice(0, start + startMarker.length) +
      '\n' + table +
      readme.slice(end)

    writeFileSync(README, updated)
    console.log('\nREADME.md i18n section updated.')
  }
}

// ── Exit ────────────────────────────────────────────────────────────────────

if (hasErrors && !warnOnly) {
  process.exit(1)
}
