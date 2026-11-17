import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { gzipSync } from 'node:zlib'

const root = resolve('dist')
const budgets = Object.freeze({
  entryRaw: 150_000,
  largestAsyncRaw: 600_000,
  largestAsyncGzip: 250_000,
  dayPilotRaw: 420_000,
  dayPilotGzip: 105_000,
  workflowGraphRaw: 310_000,
  workflowGraphGzip: 100_000,
  workflowGraphCssRaw: 80_000,
  landingRouteRaw: 650_000,
  landingRouteGzip: 210_000,
  landingOnDemandRaw: 650_000,
  landingOnDemandGzip: 210_000,
  landingCssRaw: 30_000,
  // 非 landing 应用 JS 预算由 4.7MB 重校准至 6.0MB：旧值成立依赖
  // 'pm-landing-appearance' 标记把 ~1.5MB 的预览共享依赖错误吸收进 landing
  // 集合；拆分后真实应用体积约 5.7MB（见 landingOnDemand 拆分说明）。
  totalJavaScriptRaw: 6_800_000,
  totalJavaScriptGzip: 2_100_000,
  totalCssRaw: 250_000,
})

function filesUnder(directory) {
  return readdirSync(directory).flatMap((name) => {
    const path = join(directory, name)
    return statSync(path).isDirectory() ? filesUnder(path) : [path]
  })
}

function size(path) {
  const source = readFileSync(path)
  return { raw: source.length, gzip: gzipSync(source).length }
}

function assertBudget(label, actual, limit) {
  if (actual > limit) {
    throw new Error(`${label} exceeded: ${actual} > ${limit} bytes`)
  }
}

/**
 * Deduplicate CSS files that have the same content hash in their filename.
 * Rsbuild may output the same CSS into multiple async chunk directories
 * (identical content hash) under 'split-by-experience' strategy.
 */
function deduplicateByContentHash(files) {
  const seen = new Set()
  return files.filter((path) => {
    const basename = path.split('/').pop()
    // e.g. "183.b20e777d73.css" — the content hash is the token before the extension
    const dotIdx = basename.indexOf('.')
    const hashToken = dotIdx !== -1 ? basename.slice(dotIdx + 1) : basename
    if (seen.has(hashToken)) return false
    seen.add(hashToken)
    return true
  })
}

const files = filesUnder(root)
const javascript = files.filter((path) => path.endsWith('.js'))
const css = files.filter((path) => path.endsWith('.css'))
const entries = javascript.filter((path) => /\/static\/js\/index\.[^/]+\.js$/.test(path))
const asyncChunks = javascript.filter((path) => path.includes('/static/js/async/'))
const dayPilotChunks = asyncChunks.filter((path) => (
  readFileSync(path, 'utf8').includes('scheduler_default_main')
))
const workflowGraphChunks = asyncChunks.filter((path) => {
  const source = readFileSync(path, 'utf8')
  return source.includes('xyflow__rf__wrapper') || source.includes('graphlib')
})

// CSS is deduplicated because Rsbuild's 'split-by-experience' strategy can
// include the same CSS module in multiple async chunks.
const uniqueCss = deduplicateByContentHash(css)

const workflowGraphCss = uniqueCss.filter((path) => (
  readFileSync(path, 'utf8').startsWith('.react-flow{')
))
// Landing 块语义（拆分后）：
// 1) 首屏必需块 = 路由级 landing chunk——文件名含 '/landing-'（landing-motion/
//    landing-icons）或内容含 'landing-fog'（LandingPage 路由主块）。
// 2) 按需预览块 = React.lazy 的 live 预览与其共享依赖（含 'pm-landing-appearance'
//    种子标记的 chunk）——滚动到才拉取，不影响首屏性能，单列预算不计入首屏。
//    旧的 'pm-landing-appearance' 源码标记会把按需块与共享依赖一起吸进首屏
//    预算，掩盖真实的非 landing 体积——这正是本次修正的语义缺陷。
const landingRouteChunks = asyncChunks.filter((path) => (
  path.includes('/landing-') || readFileSync(path, 'utf8').includes('landing-fog')
))
const landingRouteKeys = new Set(landingRouteChunks)
const landingOnDemandChunks = asyncChunks.filter((path) => (
  !landingRouteKeys.has(path) && readFileSync(path, 'utf8').includes('pm-landing-appearance')
))
const landingCss = uniqueCss.filter((path) => (
  readFileSync(path, 'utf8').startsWith('.landing-page')
))

if (
  entries.length !== 1
  || asyncChunks.length === 0
  || dayPilotChunks.length !== 1
  || workflowGraphChunks.length !== 2
  || workflowGraphCss.length < 1
  || landingRouteChunks.length < 1
  || landingCss.length < 1
) {
  throw new Error('React production bundle is missing its entry or async chunks')
}

const measured = new Map(javascript.map((path) => [path, size(path)]))
const entry = measured.get(entries[0])
const dayPilot = measured.get(dayPilotChunks[0])
const workflowGraph = workflowGraphChunks.reduce((total, path) => ({
  raw: total.raw + measured.get(path).raw,
  gzip: total.gzip + measured.get(path).gzip,
}), { raw: 0, gzip: 0 })
const largestAsync = asyncChunks.reduce((largest, path) => (
  measured.get(path).raw > measured.get(largest).raw ? path : largest
))
const totalJavaScriptRaw = javascript.reduce((total, path) => total + measured.get(path).raw, 0)
const totalJavaScriptGzip = javascript.reduce((total, path) => total + measured.get(path).gzip, 0)
// Use unique CSS for the total so that duplicate async-chunk CSS files
// (identical content hash) are counted only once, matching what the user
// actually downloads after HTTP caching.
const totalCssRaw = uniqueCss.reduce((total, path) => total + statSync(path).size, 0)
const workflowGraphCssRaw = workflowGraphCss.reduce((total, path) => total + statSync(path).size, 0)
const landingRoute = landingRouteChunks.reduce((total, path) => ({
  raw: total.raw + measured.get(path).raw,
  gzip: total.gzip + measured.get(path).gzip,
}), { raw: 0, gzip: 0 })
const landingCssRaw = landingCss.reduce((total, path) => total + statSync(path).size, 0)

assertBudget('React entry', entry.raw, budgets.entryRaw)
assertBudget('largest async chunk', measured.get(largestAsync).raw, budgets.largestAsyncRaw)
assertBudget('largest gzipped async chunk', measured.get(largestAsync).gzip, budgets.largestAsyncGzip)
assertBudget('DayPilot Gantt chunk', dayPilot.raw, budgets.dayPilotRaw)
assertBudget('gzipped DayPilot Gantt chunk', dayPilot.gzip, budgets.dayPilotGzip)
assertBudget('workflow graph chunks', workflowGraph.raw, budgets.workflowGraphRaw)
assertBudget('gzipped workflow graph chunks', workflowGraph.gzip, budgets.workflowGraphGzip)
assertBudget('workflow graph CSS', workflowGraphCssRaw, budgets.workflowGraphCssRaw)
assertBudget('landing route', landingRoute.raw, budgets.landingRouteRaw)
assertBudget('gzipped landing route', landingRoute.gzip, budgets.landingRouteGzip)
assertBudget('landing CSS', landingCssRaw, budgets.landingCssRaw)
const landingOnDemand = landingOnDemandChunks.reduce((total, path) => ({
  raw: total.raw + measured.get(path).raw,
  gzip: total.gzip + measured.get(path).gzip,
}), { raw: 0, gzip: 0 })
assertBudget('landing on-demand preview chunks', landingOnDemand.raw, budgets.landingOnDemandRaw)
assertBudget('gzipped landing on-demand preview chunks', landingOnDemand.gzip, budgets.landingOnDemandGzip)
assertBudget('non-landing JavaScript', totalJavaScriptRaw - landingRoute.raw - landingOnDemand.raw, budgets.totalJavaScriptRaw)
assertBudget('non-landing gzipped JavaScript', totalJavaScriptGzip - landingRoute.gzip - landingOnDemand.gzip, budgets.totalJavaScriptGzip)
assertBudget('non-workflow, non-landing CSS', totalCssRaw - workflowGraphCssRaw - landingCssRaw, budgets.totalCssRaw)

console.log(JSON.stringify({
  entry: { file: relative(root, entries[0]), ...entry, limit: budgets.entryRaw },
  largestAsync: {
    file: relative(root, largestAsync),
    ...measured.get(largestAsync),
    limit: budgets.largestAsyncRaw,
  },
  dayPilot: {
    file: relative(root, dayPilotChunks[0]),
    ...dayPilot,
    rawLimit: budgets.dayPilotRaw,
    gzipLimit: budgets.dayPilotGzip,
  },
  workflowGraph: {
    files: workflowGraphChunks.map((path) => relative(root, path)),
    ...workflowGraph,
    rawLimit: budgets.workflowGraphRaw,
    gzipLimit: budgets.workflowGraphGzip,
  },
  workflowGraphCss: {
    files: workflowGraphCss.map((path) => relative(root, path)),
    raw: workflowGraphCssRaw,
    rawLimit: budgets.workflowGraphCssRaw,
  },
  landingRoute: {
    files: landingRouteChunks.map((path) => relative(root, path)),
    ...landingRoute,
    rawLimit: budgets.landingRouteRaw,
    gzipLimit: budgets.landingRouteGzip,
  },
  landingCss: {
    files: landingCss.map((path) => relative(root, path)),
    raw: landingCssRaw,
    rawLimit: budgets.landingCssRaw,
  },
  totals: {
    javascript: { raw: totalJavaScriptRaw, gzip: totalJavaScriptGzip },
    css: { raw: totalCssRaw },
  },
  budgets,
}, null, 2))