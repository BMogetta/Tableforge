#!/usr/bin/env node
/**
 * Generates Zod schemas from JSON Schema definitions.
 * Defs are generated as standalone schemas; endpoint schemas reference them.
 * Output: frontend/src/lib/schema-generated.zod.ts
 */
import { readFileSync, writeFileSync, readdirSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'
import { createRequire } from 'module'
const require = createRequire(join(dirname(fileURLToPath(import.meta.url)), '../frontend/package.json'))
const { jsonSchemaToZod } = require('json-schema-to-zod')

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')
const SCHEMA_DIR = join(ROOT, 'shared/schemas')
const DEFS_DIR = join(SCHEMA_DIR, 'defs')
const OUT_FILE = join(ROOT, 'frontend/src/lib/schema-generated.zod.ts')

// --- Helpers ----------------------------------------------------------------

function readSchema(path) {
  return JSON.parse(readFileSync(path, 'utf-8'))
}

/** "game_session" → "gameSessionSchema" */
function toSchemaName(filename) {
  const base = filename.replace(/\.json$/, '').replace(/\.(request|response)$/, '_$1')
  const camel = base.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
  return camel + 'Schema'
}

/** "game_session" → "GameSession" */
function toTypeName(filename) {
  const base = filename.replace(/\.json$/, '').replace(/\.(request|response)$/, '_$1')
  return base
    .split('_')
    .map(w => w.charAt(0).toUpperCase() + w.slice(1))
    .join('')
}

/** Map $ref paths to their Zod schema variable names. */
const refToSchemaName = {}

function generateZod(schema, name) {
  return jsonSchemaToZod(schema, { name, module: 'none', noImport: true })
}

/**
 * Build a Zod object literal for an endpoint schema, replacing $ref with
 * references to the already-generated def schemas.
 */
function buildEndpointZod(schema, name) {
  const props = schema.properties ?? {}
  const required = new Set(schema.required ?? [])
  const fields = []

  for (const [key, prop] of Object.entries(props)) {
    let zodExpr = propToZod(prop)
    if (!required.has(key)) zodExpr += '.optional()'
    fields.push(`  ${JSON.stringify(key)}: ${zodExpr}`)
  }

  return `const ${name} = z.object({\n${fields.join(',\n')}\n})`
}

/** Convert a single property definition to a Zod expression string. */
function propToZod(prop) {
  // $ref → use the def schema variable
  if (prop['$ref']) {
    const varName = refToSchemaName[prop['$ref']]
    if (varName) return varName
  }

  // array with $ref items
  if (prop.type === 'array' && prop.items) {
    const inner = propToZod(prop.items)
    return `z.array(${inner})`
  }

  // object with additionalProperties (Record)
  if (prop.type === 'object' && prop.additionalProperties) {
    const valZod = propToZod(prop.additionalProperties)
    return `z.record(z.string(), ${valZod})`
  }

  // enum
  if (prop.enum) {
    const vals = prop.enum.map(v => JSON.stringify(v)).join(', ')
    return `z.enum([${vals}])`
  }

  // primitives
  switch (prop.type) {
    case 'string':
      if (prop.format === 'date-time') return 'z.string().datetime({ offset: true })'
      if (prop.minLength) return `z.string().min(${prop.minLength})`
      return 'z.string()'
    case 'integer':
    case 'number': {
      let expr = 'z.number()'
      if (prop.type === 'integer') expr += '.int()'
      if (prop.minimum != null) expr += `.gte(${prop.minimum})`
      return expr
    }
    case 'boolean':
      return 'z.boolean()'
    case 'object':
      return 'z.record(z.string(), z.unknown())'
    default:
      return 'z.unknown()'
  }
}

// --- Main -------------------------------------------------------------------

const lines = [
  `import { z } from 'zod'`,
  '',
  '// ---- Shared types (defs/) -------------------------------------------------',
  '',
]

// 1. Generate defs as standalone exported schemas
const defFiles = readdirSync(DEFS_DIR).filter(f => f.endsWith('.json')).sort()
for (const file of defFiles) {
  const schema = readSchema(join(DEFS_DIR, file))
  delete schema['$schema']
  delete schema['$id']
  const name = toSchemaName(file)
  const typeName = toTypeName(file)

  // Register $ref mapping: "defs/game_session.json" → "gameSessionSchema"
  refToSchemaName[`defs/${file}`] = name

  const code = generateZod(schema, name)
  lines.push(`export ${code}`)
  lines.push(`export type ${typeName} = z.infer<typeof ${name}>`, '')
}

lines.push('// ---- Endpoint schemas ----------------------------------------------------', '')

// 2. Generate endpoint schemas referencing defs
const endpointFiles = readdirSync(SCHEMA_DIR).filter(f => f.endsWith('.json')).sort()
for (const file of endpointFiles) {
  const schema = readSchema(join(SCHEMA_DIR, file))
  const name = toSchemaName(file)
  const typeName = toTypeName(file)

  // If it has properties with $ref, use our builder; otherwise use json-schema-to-zod
  const hasRef = Object.values(schema.properties ?? {}).some(
    p => p['$ref'] || (p.type === 'array' && p.items?.['$ref'])
  )

  let code
  if (hasRef) {
    code = `export ${buildEndpointZod(schema, name)}`
  } else {
    delete schema['$schema']
    delete schema['$id']
    code = `export ${generateZod(schema, name)}`
  }

  lines.push(code)
  lines.push(`export type ${typeName} = z.infer<typeof ${name}>`, '')
}

const header = `/* eslint-disable */
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED FROM JSON SCHEMAS                  ##
 * ## DO NOT MODIFY BY HAND — edit shared/schemas/*.json instead ##
 * ---------------------------------------------------------------
 */
`

writeFileSync(OUT_FILE, header + '\n' + lines.join('\n') + '\n')
console.log(`Generated ${OUT_FILE}`)
