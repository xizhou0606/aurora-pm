import { expect } from 'bun:test'

import { type Locator, type Page } from 'playwright-core'

const DEFAULT_TIMEOUT = 15_000

/**
 * Replacements for @playwright/test auto-retrying assertions, built only on
 * the official playwright-core Locator API plus bun:test expect.
 */

export async function waitForVisible(locator: Locator, timeout = DEFAULT_TIMEOUT): Promise<void> {
  await locator.waitFor({ state: 'visible', timeout })
}

export async function waitForCount(locator: Locator, count: number, timeout = DEFAULT_TIMEOUT): Promise<void> {
  const deadline = Date.now() + timeout
  let last = -1
  while (Date.now() < deadline) {
    last = await locator.count()
    if (last === count) return
    await Bun.sleep(100)
  }
  expect(last).toBe(count)
}

export async function waitForUrl(
  page: Page,
  expected: string | RegExp,
  timeout = DEFAULT_TIMEOUT,
): Promise<void> {
  const deadline = Date.now() + timeout
  let url = ''
  // Mirror @playwright/test toHaveURL semantics: a string expectation is
  // resolved against the current page origin before comparison.
  const matches = (current: string) => {
    if (typeof expected !== 'string') return expected.test(current)
    try {
      return current === new URL(expected, current).toString()
    } catch {
      return current === expected
    }
  }
  while (Date.now() < deadline) {
    url = page.url()
    if (matches(url)) return
    await Bun.sleep(100)
  }
  if (typeof expected === 'string') expect(url).toBe(expected)
  else expect(url).toMatch(expected)
}

export async function waitForAttribute(
  locator: Locator,
  name: string,
  value: string,
  timeout = DEFAULT_TIMEOUT,
): Promise<void> {
  const deadline = Date.now() + timeout
  let last: string | null = null
  while (Date.now() < deadline) {
    last = await locator.getAttribute(name)
    if (last === value) return
    await Bun.sleep(100)
  }
  expect(last).toBe(value)
}

export async function waitForCondition(
  condition: () => boolean | Promise<boolean>,
  timeout = DEFAULT_TIMEOUT,
): Promise<void> {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    if (await condition()) return
    await Bun.sleep(100)
  }
  expect(await condition()).toBe(true)
}
