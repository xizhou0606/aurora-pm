import { mkdirSync } from 'node:fs'
import { join } from 'node:path'

import type { Browser, Page, Route } from 'playwright-core'

import { PM_ACTION_CODES } from '../../src/generated/access-catalog'
import { E2E_BASE_URL } from './browser'

export type VisualMode = 'standalone' | 'embedded'

export type VisualRoute = {
  slug: string
  path: string
  /**
   * Mode-agnostic readiness marker: the route's empty-state title from shared
   * copy (deterministic under the fixture's empty API mocks). Headings cannot
   * be used because standalone renders the page title in the shell while
   * embedded hides the duplicate shell (and with it, the h1).
   */
  readyText: string
}

export const VISUAL_ROUTES: VisualRoute[] = [
  { slug: 'workspace-projects', path: '/workspace/projects', readyText: '暂无项目集' },
  { slug: 'design-management', path: '/workspace/design-management', readyText: '还没有走查记录' },
]

export const VISUAL_WIDTHS = [1280, 390] as const

const VISUAL_HEIGHTS: Record<number, number> = {
  1280: 800,
  390: 844,
}

const VISUAL_ROOT = join(import.meta.dir, '..', 'visual')
export const VISUAL_ACTUAL_DIR = join(VISUAL_ROOT, 'actual')
export const VISUAL_TRACE_DIR = join(VISUAL_ROOT, 'traces')

const HOST_PAGE_PATH = '/__e2e_host__'
const PARENT_ORIGIN = new URL(E2E_BASE_URL).origin

/**
 * Embedded entry mechanism (verified against apps/web/src/shared/runtime):
 * `embeddedAtStartup` flips on `window.__MICRO_APP_ENVIRONMENT__` /
 * `window.microApp` before app scripts run, and the session is rehydrated from
 * the host data payload. page.addInitScript applies to every frame, so the app
 * runs embedded inside the fixture host page's iframe (docs §5.4 quadrant B).
 */
export function embeddedSessionScript(routePath: string): string {
  const hostData = {
    contractVersion: 1,
    embedded: true,
    parentOrigin: PARENT_ORIGIN,
    auth: {
      token: 'visual-baseline-token',
      user: { id: 7, username: 'reviewer', display_name: '视觉基线用户', role: 'member', status: 'active' },
    },
    workspace: { id: 1 },
    route: { path: routePath, fullPath: routePath },
    timezone: 'Asia/Shanghai',
  }
  return `
    window.__E2E_HOST_DATA__ = ${JSON.stringify(hostData)}
    window.__MICRO_APP_ENVIRONMENT__ = true
    window.microApp = {
      addDataListener(listener, autoTrigger) {
        if (autoTrigger) listener(window.__E2E_HOST_DATA__)
      },
      removeDataListener() {},
      dispatch() {},
    }
  `
}

function standaloneSessionScript(): string {
  return `
    window.localStorage.setItem('pm_standalone_token', 'visual-baseline-token')
    window.localStorage.setItem('pm_workspace_id', '1')
  `
}

export function hostPageHtml(routePath: string): string {
  return `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>E2E Embedded Host</title>
<style>
  html, body { margin: 0; padding: 0; height: 100%; overflow: hidden; }
  iframe { display: block; width: 100vw; height: 100vh; border: 0; }
</style></head>
<body><iframe id="pm-host-frame" src="${routePath}"></iframe></body>
</html>`
}

/**
 * Deterministic API stubs for visual baselines: real auth handshake, empty
 * data pages (stable EmptyState rendering), catch-all for unmapped reads.
 */
export async function fulfillVisualAPI(route: Route): Promise<void> {
  const method = route.request().method()
  const path = new URL(route.request().url()).pathname

  const json = (data: unknown) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ data }),
  })

  if (method === 'GET' && path === '/api/v1/core/auth/me') {
    return json({ id: 7, username: 'reviewer', display_name: '视觉基线用户', role: 'member', status: 'active' })
  }
  if (method === 'GET' && path === '/api/v1/core/auth/me/workspaces') {
    return json([{ id: 1, name: '产品研发', slug: 'product', is_default: true, enabled: true }])
  }
  if (method === 'GET' && path === '/api/v1/core/access/me/action-permissions') {
    return json({ action_codes: [...PM_ACTION_CODES] })
  }
  if (method === 'GET' && path === '/api/v1/platform/design-reviews/capabilities') {
    return json({
      can_view: true,
      can_create: true,
      can_manage_differences: true,
      can_manage_analysis: true,
      can_delete: true,
      can_manage_collectors: true,
      creation_available: true,
      figma_preview_available: true,
      analysis_available: true,
      object_storage_available: true,
      unavailable_reasons: [],
      allowed_actions: [...PM_ACTION_CODES],
    })
  }
  if (method === 'GET' && path === '/api/v1/platform/design-reviews/requirements') {
    return json([])
  }
  if (method === 'GET') {
    return json({ items: [], total: 0, page: 1, page_size: 20 })
  }
  return json({ ok: true })
}

export type CaptureOptions = {
  route: VisualRoute
  mode: VisualMode
  width: number
}

async function configurePage(page: Page, options: CaptureOptions): Promise<void> {
  await page.addInitScript(
    options.mode === 'embedded'
      ? embeddedSessionScript(options.route.path)
      : standaloneSessionScript(),
  )
  await page.route('**/api/v1/**', (route) => fulfillVisualAPI(route))
  if (options.mode === 'embedded') {
    await page.route(`**${HOST_PAGE_PATH}*`, (route) => {
      const requested = new URL(route.request().url())
      const routePath = requested.searchParams.get('route') || options.route.path
      return route.fulfill({
        status: 200,
        contentType: 'text/html; charset=utf-8',
        body: hostPageHtml(routePath),
      })
    })
  }
}

async function openRoute(page: Page, options: CaptureOptions): Promise<string> {
  if (options.mode === 'embedded') {
    const hostURL = new URL(`${HOST_PAGE_PATH}?route=${encodeURIComponent(options.route.path)}`, E2E_BASE_URL)
    await page.goto(hostURL.toString(), { waitUntil: 'load' })
  } else {
    await page.goto(new URL(options.route.path, E2E_BASE_URL).toString(), { waitUntil: 'load' })
  }
  // Readiness marker: page locators do not pierce iframes, so embedded mode
  // enters through the official frameLocator API (docs §5.4).
  if (options.mode === 'embedded') {
    await page.frameLocator('#pm-host-frame').getByText(options.route.readyText).waitFor({ state: 'visible', timeout: 30_000 })
  } else {
    await page.getByText(options.route.readyText).first().waitFor({ state: 'visible', timeout: 30_000 })
  }
  await page.waitForLoadState('networkidle')
  // Fixed settle window so webfonts/icon sprite/initial paint finish deterministically.
  await new Promise((resolve) => setTimeout(resolve, 600))
  await freezeSvgAnimations(page)
  return page.url()
}

/**
 * Freezes SVG SMIL animations (e.g. BrandMark's animateMotion orbit) at their
 * start state using the official SVG DOM API. `screenshot({ animations:
 * 'disabled' })` covers CSS/WAAPI only, and SMIL ignores prefers-reduced-motion.
 */
async function freezeSvgAnimations(page: Page): Promise<void> {
  const freeze = () => {
    for (const svg of document.querySelectorAll('svg')) {
      const el = svg as SVGSVGElement
      if (typeof el.pauseAnimations !== 'function') continue
      try {
        el.setCurrentTime(0)
        el.pauseAnimations()
      } catch { /* detached node */ }
    }
  }
  for (const frame of page.frames()) {
    await frame.evaluate(freeze).catch(() => {})
  }
}

/**
 * One quadrant of the 2x2 visual matrix (mode x width) for one route.
 * Arms context.tracing and persists a trace.zip under visual/traces on failure.
 */
export async function captureQuadrant(
  browser: Browser,
  options: CaptureOptions,
): Promise<{ path: string; url: string }> {
  const context = await browser.newContext({
    viewport: { width: options.width, height: VISUAL_HEIGHTS[options.width] ?? 800 },
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    deviceScaleFactor: 1,
    reducedMotion: 'reduce',
  })
  await context.tracing.start({ screenshots: true, snapshots: true, sources: true })
  const page = await context.newPage()
  try {
    await configurePage(page, options)
    const url = await openRoute(page, options)

    const dir = join(VISUAL_ACTUAL_DIR, options.route.slug)
    mkdirSync(dir, { recursive: true })
    const file = join(dir, `${options.mode}-${options.width}.png`)
    await page.screenshot({
      path: file,
      animations: 'disabled',
      caret: 'hide',
    })
    await context.tracing.stop()
    await context.close()
    return { path: file, url }
  } catch (error) {
    if (process.env.E2E_DEBUG) {
      const frames = page.frames().map((f) => f.url())
      console.log('[debug] frames:', JSON.stringify(frames))
      for (const frame of page.frames()) {
        const text = await frame.evaluate(() => document.body?.innerText.slice(0, 200) || '(empty)').catch((e) => `ERR ${String(e).slice(0, 80)}`)
        console.log(`[debug] frame ${frame.url().slice(0, 60)} => ${JSON.stringify(text)}`)
      }
    }
    mkdirSync(VISUAL_TRACE_DIR, { recursive: true })
    const tracePath = join(VISUAL_TRACE_DIR, `${options.route.slug}-${options.mode}-${options.width}.zip`)
    await context.tracing.stop({ path: tracePath }).catch(() => {})
    await context.close().catch(() => {})
    throw error
  }
}
