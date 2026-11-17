import { build } from 'esbuild'
import { copyFile, mkdir, readFile, rename, rm, writeFile } from 'node:fs/promises'

await build({ entryPoints: ['src/background.ts'], outfile: 'dist/background.js', bundle: true, format: 'esm', target: 'chrome120' })
await build({ entryPoints: ['src/content.ts'], outfile: 'dist/content.js', bundle: true, format: 'iife', target: 'chrome120' })
await mkdir('dist', { recursive: true })
await copyFile('src/manifest.json', 'dist/manifest.json')
try { await rename('dist/index.html', 'dist/popup.html') } catch {
  const candidates = ['dist/popup.html', 'dist/index.html']
  const existing = await Promise.all(candidates.map(async (file) => readFile(file, 'utf8').then(() => file).catch(() => null)))
  if (!existing.some(Boolean)) throw new Error('Rsbuild did not emit the popup HTML')
}
await rm('dist/.rsbuild', { recursive: true, force: true })
const manifest = JSON.parse(await readFile('dist/manifest.json', 'utf8'))
manifest.version = JSON.parse(await readFile('package.json', 'utf8')).version
await writeFile('dist/manifest.json', `${JSON.stringify(manifest, null, 2)}\n`)
