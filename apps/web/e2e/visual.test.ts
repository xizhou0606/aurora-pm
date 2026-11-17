import { afterAll, describe, test } from 'bun:test'

import { closeBrowser, launchBrowser } from './fixtures/browser'
import { captureQuadrant, VISUAL_ROUTES, VISUAL_WIDTHS, type VisualMode } from './fixtures/visual-helpers'

/**
 * Four-quadrant visual matrix (docs/bun-fullstack-adoption.md §5.4):
 * standalone/embedded x 1280/390. Each test captures one quadrant screenshot
 * to e2e/visual/actual/<route-slug>/<mode>-<width>.png; comparison vs baseline
 * runs via `bun x reg-cli` (see package.json "test:visual").
 */
describe('visual matrix', () => {
  const modes: VisualMode[] = ['standalone', 'embedded']
  // bun test default per-test timeout is 5s; embedded iframe boot needs more.
  const testTimeout = 60_000

  for (const mode of modes) {
    for (const width of VISUAL_WIDTHS) {
      for (const route of VISUAL_ROUTES) {
        test(`${mode} ${width}px ${route.slug}`, async () => {
          const browser = await launchBrowser()
          const result = await captureQuadrant(browser, { route, mode, width })
          console.log(`captured ${result.path}`)
        }, testTimeout)
      }
    }
  }
})

afterAll(async () => {
  await closeBrowser()
})
