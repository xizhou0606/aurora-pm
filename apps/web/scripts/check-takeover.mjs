import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'

const root = resolve('.')
const sourceRoot = join(root, 'src')
const packageJson = JSON.parse(readFileSync(join(root, 'package.json'), 'utf8'))
const bannedDirectDependencies = new Set([
  '@element-plus/icons-vue',
  '@module-federation/enhanced',
  '@module-federation/rsbuild-plugin',
  '@vitejs/plugin-vue',
  '@vue/test-utils',
  'element-plus',
  'lucide-vue-next',
  'pinia',
  'vite',
  'vue',
  'vue-router',
  'vue-tsc',
])

function filesUnder(directory) {
  if (!existsSync(directory)) return []
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    return entry.isDirectory() ? filesUnder(path) : [path]
  })
}

function fail(message) {
  throw new Error(message)
}

const directDependencies = {
  ...packageJson.dependencies,
  ...packageJson.devDependencies,
}
for (const dependency of bannedDirectDependencies) {
  if (dependency in directDependencies) fail(`legacy direct dependency remains: ${dependency}`)
}
for (const scriptName of Object.keys(packageJson.scripts)) {
  if (scriptName.endsWith(':react') || scriptName.includes('legacy')) {
    fail(`dual-build script remains: ${scriptName}`)
  }
}

const forbiddenPaths = [
  'dist-react',
  'index.react.html',
  'tsconfig.react.json',
  'vite.config.ts',
  'vitest.react.config.ts',
  'src/App.vue',
  'src/features/ui-harness',
]
for (const path of forbiddenPaths) {
  if (existsSync(join(root, path))) fail(`legacy path remains: ${path}`)
}

const sourceFiles = filesUnder(sourceRoot)
const vueFiles = sourceFiles.filter((path) => extname(path) === '.vue')
if (vueFiles.length) fail(`Vue source remains: ${vueFiles.map((path) => relative(root, path)).join(', ')}`)

const sourceText = sourceFiles
  .filter((path) => /\.(css|ts|tsx)$/.test(path) && !/\.(?:react-)?test\./.test(path))
  .map((path) => readFileSync(path, 'utf8'))
  .join('\n')
for (const token of [
  '@module-federation',
  "from 'pinia'",
  "from 'vue'",
  "from 'vue-router'",
  "from 'element-plus'",
  '--el-',
  '/ui-harness',
]) {
  if (sourceText.includes(token)) fail(`legacy source token remains: ${token}`)
}

const rsbuild = readFileSync(join(root, 'rsbuild.config.ts'), 'utf8')
if (!rsbuild.includes("root: 'dist'")) fail('Rsbuild must own the canonical dist directory')
if (!rsbuild.includes("index: './src/main.tsx'")) fail('Rsbuild must own the canonical React entry')
if (/VITE_PM_NATIVE_(?:INBOX|WORK_ITEMS|BI_SCREEN|QUALITY)_MODE/.test(rsbuild)) {
  fail('retired native module migration flags remain in the build')
}
if (rsbuild.includes('@module-federation')) fail('Module Federation plugin remains in production')
const catchAllApiProxyIndex = rsbuild.indexOf("'/api': {")
for (const route of [
  '/api/v1/core/auth/login',
  '/api/v1/core/auth/logout',
  '/api/v1/core/auth/me',
  '/api/v1/core/workspaces',
]) {
  const routeIndex = rsbuild.indexOf(`'${route}': {`)
  if (routeIndex < 0 || routeIndex > catchAllApiProxyIndex) {
    fail(`host identity proxy must precede the catch-all API proxy: ${route}`)
  }
  if (!rsbuild.slice(routeIndex, routeIndex + 220).includes('target: hostAuthProxyTarget')) {
    fail(`host identity proxy must use VITE_DEV_HOST_AUTH_PROXY_TARGET: ${route}`)
  }
}

const html = readFileSync(join(root, 'index.html'), 'utf8')
if (!html.includes('id="react-root"') || html.includes('id="app"')) {
  fail('canonical HTML is not React-owned')
}

const dist = join(root, 'dist')
if (!existsSync(join(dist, 'index.html'))) fail('canonical React dist is missing; run pnpm build first')
if (existsSync(join(dist, 'mf-manifest.json')) || existsSync(join(dist, 'mf-stats.json'))) {
  fail('Module Federation artifacts remain in the canonical dist')
}

console.log(JSON.stringify({
  entry: 'src/main.tsx',
  output: 'dist',
  vueFiles: 0,
  moduleFederationArtifacts: 0,
  moduleRuntime: 'native-only',
}, null, 2))
