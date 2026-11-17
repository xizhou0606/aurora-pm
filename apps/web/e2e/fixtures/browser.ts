import { mkdirSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'

import { chromium, type Browser, type BrowserContext, type Page } from 'playwright-core'

export const E2E_PORT = Number(process.env.PM_E2E_PORT || 5175)
export const E2E_BASE_URL = process.env.PM_E2E_BASE_URL || `http://127.0.0.1:${E2E_PORT}`

/**
 * Official playwright-core fallback: the library supports pointing
 * `executablePath` at any Chromium-family binary. Used only when the system
 * Google Chrome is absent (channel: 'chrome' launch throws).
 */
const FALLBACK_CHROMIUM_PATH = join(
  homedir(),
  'Library/Caches/ms-playwright/chromium-1228/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
)

let sharedBrowser: Browser | null = null

/**
 * Browser strategy (official playwright-core API only):
 * 1. E2E_CHROMIUM_PATH env -> explicit executablePath (highest priority).
 * 2. channel: 'chrome'      -> system-installed Google Chrome.
 * 3. executablePath         -> cached Playwright "Chrome for Testing" build.
 */
export async function launchBrowser(): Promise<Browser> {
  if (sharedBrowser?.isConnected()) return sharedBrowser
  const overridePath = process.env.E2E_CHROMIUM_PATH
  if (overridePath) {
    sharedBrowser = await chromium.launch({ executablePath: overridePath })
    return sharedBrowser
  }
  try {
    sharedBrowser = await chromium.launch({ channel: 'chrome' })
    return sharedBrowser
  } catch {
    sharedBrowser = await chromium.launch({ executablePath: FALLBACK_CHROMIUM_PATH })
    return sharedBrowser
  }
}

export async function closeBrowser(): Promise<void> {
  const browser = sharedBrowser
  sharedBrowser = null
  await browser?.close()
}

export type BrowserSession = {
  context: BrowserContext
  page: Page
}

const TRACE_DIR = join(import.meta.dir, '..', 'traces')

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80)
}

/**
 * Runs one test body inside a fresh context with tracing armed. On failure the
 * official `context.tracing.stop({ path })` API persists a trace.zip for
 * `bun x playwright show-trace` replay, then the error is rethrown.
 */
export async function withTracedPage(
  name: string,
  body: (page: Page) => Promise<void>,
): Promise<void> {
  const browser = await launchBrowser()
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    deviceScaleFactor: 1,
  })
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true })
  const page = await context.newPage()
  try {
    await body(page)
    await context.tracing.stop()
  } catch (error) {
    mkdirSync(TRACE_DIR, { recursive: true })
    await context.tracing.stop({ path: join(TRACE_DIR, `${slugify(name)}.zip`) }).catch(() => {})
    await context.close().catch(() => {})
    throw error
  }
  await context.close()
}
