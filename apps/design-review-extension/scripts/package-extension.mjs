import { createHash } from 'node:crypto'
import { cp, mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { resolve } from 'node:path'

const version = JSON.parse(await readFile('package.json', 'utf8')).version
const filename = `pm-design-review-extension-${version}.zip`
const downloads = resolve('../web/public/downloads')
const output = resolve(downloads, filename)
const extracted = resolve(downloads, `pm-design-review-extension-${version}`)
await mkdir(downloads, { recursive: true })
await rm(output, { force: true })
execFileSync('zip', ['-q', '-r', '-X', output, '.'], { cwd: 'dist' })
await rm(extracted, { recursive: true, force: true })
await cp('dist', extracted, { recursive: true })
const bytes = await readFile(output)
const sha256 = createHash('sha256').update(bytes).digest('hex')
await writeFile(`${output}.sha256`, `${sha256}  ${filename}\n`)

const clientsManifest = resolve(downloads, `pm-design-review-clients-${version}.json`)
try {
  const manifest = JSON.parse(await readFile(clientsManifest, 'utf8'))
  manifest.generatedAt = new Date().toISOString()
  manifest.artifacts.extension = { file: filename, sha256, byteSize: bytes.byteLength }
  const manifestBody = `${JSON.stringify(manifest, null, 2)}\n`
  await writeFile(clientsManifest, manifestBody)
  const manifestSha256 = createHash('sha256').update(manifestBody).digest('hex')
  await writeFile(`${clientsManifest}.sha256`, `${manifestSha256}  pm-design-review-clients-${version}.json\n`)
} catch (error) {
  if (error?.code !== 'ENOENT') throw error
}
