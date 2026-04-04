#!/usr/bin/env node
// check-i18n.mjs — compare translation keys between locale files.
// Reports missing keys in any language relative to the reference (en.json).
//
// Usage: node scripts/check-i18n.mjs

import { readFileSync, readdirSync } from 'fs'
import { join, basename } from 'path'

const LOCALES_DIR = join(import.meta.dirname, '..', 'frontend', 'src', 'locales')

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

let hasErrors = false

for (const [lang, keys] of Object.entries(locales)) {
  if (lang === reference) continue

  const missing = [...refKeys].filter(k => !keys.has(k))
  const extra = [...keys].filter(k => !refKeys.has(k))

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

// Summary
console.log(`\nReference: ${reference}.json (${refKeys.size} keys)`)
console.log(`Locales: ${files.map(f => basename(f, '.json')).join(', ')}`)

if (hasErrors) {
  process.exit(1)
}
