// One-off migration: rewrite lucide-react imports to the Hugeicons icon system
// (@/shared/ui/icons) using apps/web/src/shared/ui/icons/registry.json.
// Value/type identifiers are replaced outside string literals and comments only.
// Run: bun scripts/migrate-lucide-icons.mjs   (from apps/web)
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const webRoot = path.resolve(here, '..')
const srcRoot = path.join(webRoot, 'src')
const registry = JSON.parse(fs.readFileSync(path.join(webRoot, 'src/shared/ui/icons/registry.json'), 'utf8'))

const IMPORT_RE = /import\s+(type\s+)?\{([^}]*)\}\s*from\s*['"]lucide-react['"]/g
const NAMESPACE_RE = /import\s+\*\s+as\s+\w+\s+from\s*['"]lucide-react['"]/

const files = []
;(function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(p)
    else if (/\.(tsx?|mjs)$/.test(entry.name)) files.push(p)
  }
})(srcRoot)

// Mask string literals and comments so identifier replacement never touches copy.
function maskNonCode(source) {
  const mask = [...source].map(() => false)
  let i = 0
  while (i < source.length) {
    const ch = source[i]
    const next = source[i + 1]
    if (ch === '/' && next === '/') {
      let j = i
      while (j < source.length && source[j] !== '\n') j++
      for (let k = i; k < j; k++) mask[k] = true
      i = j
    } else if (ch === '/' && next === '*') {
      let j = source.indexOf('*/', i + 2)
      j = j === -1 ? source.length : j + 2
      for (let k = i; k < j; k++) mask[k] = true
      i = j
    } else if (ch === '"' || ch === "'" || ch === '`') {
      let j = i + 1
      while (j < source.length) {
        if (source[j] === '\\') j += 2
        else if (source[j] === ch) { j++; break }
        else j++
      }
      for (let k = i; k < j; k++) mask[k] = true
      i = j
    } else i++
  }
  return mask
}

function replaceOutsideNonCode(source, pattern, replacement) {
  const mask = maskNonCode(source)
  return source.replace(pattern, (match, ...args) => {
    const offset = args[args.length - 2]
    const masked = mask.slice(offset, offset + match.length).some(Boolean)
    return masked ? match : typeof replacement === 'function' ? replacement(match) : replacement
  })
}

let touchedFiles = 0
let touchedImports = 0
const problems = []

for (const file of files) {
  const source = fs.readFileSync(file, 'utf8')
  if (!source.includes('lucide-react')) continue
  if (NAMESPACE_RE.test(source)) problems.push(`${path.relative(webRoot, file)}: namespace lucide import is not supported`)

  let code = source
  const renames = [] // [localBindingInCode, newIdentifier]
  code = code.replace(IMPORT_RE, (full, typeMarker, specifiers) => {
    touchedImports++
    const items = specifiers.split(',').map((s) => s.trim()).filter(Boolean)
    const out = []
    for (const item of items) {
      const isType = /^(type\s+)/.test(item)
      const bare = item.replace(/^type\s+/, '')
      const [orig, alias] = bare.split(/\s+as\s+/).map((s) => s.trim())
      const local = alias ?? orig
      if (orig === 'LucideIcon') {
        renames.push([local, 'IconComponent'])
        out.push(isType || typeMarker ? `type ${local === 'IconComponent' ? 'IconComponent' : `IconComponent as ${local}`}` : `IconComponent as ${local}`)
        continue
      }
      const target = registry.mappings[orig]
      if (!target) {
        problems.push(`${path.relative(webRoot, file)}: no mapping for lucide icon "${orig}"`)
        return full
      }
      const component = 'Huge' + target.split(':')[1].split('-').map((p) => p.charAt(0).toUpperCase() + p.slice(1)).join('')
      renames.push([local, component])
      out.push(alias ? `${component} as ${alias}` : component)
    }
    return `import { ${out.join(', ')} } from '@/shared/ui/icons'`
  })
  if (renames.length === 0 && code === source) continue

  for (const [local, component] of renames) {
    if (local === component) continue
    code = replaceOutsideNonCode(code, new RegExp(`\\b${local}\\b`, 'g'), component)
  }

  if (code !== source) {
    fs.writeFileSync(file, code)
    touchedFiles++
  }
}

for (const problem of problems) console.error(`migrate-lucide-icons: ${problem}`)
console.log(`migrate-lucide-icons: ${touchedFiles} files rewritten, ${touchedImports} imports migrated`)
if (problems.length) process.exit(1)
