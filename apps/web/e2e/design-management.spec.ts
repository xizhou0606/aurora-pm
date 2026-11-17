import { afterAll, expect, test } from 'bun:test'

import type { Page } from 'playwright-core'

import { closeBrowser, E2E_BASE_URL, withTracedPage } from './fixtures/browser'
import {
  waitForAttribute,
  waitForCondition,
  waitForCount,
  waitForUrl,
  waitForVisible,
} from './fixtures/assert'
import {
  COLLECTOR_ID,
  DIFFERENCE_ID,
  REVIEW_ID,
  ROOT_REVIEW_ID,
  expectWorkspaceHeaders,
  installDesignReviewAPI,
} from './fixtures/design-review'

const listPath = '/workspace/design-management'
const detailPath = `${listPath}/reviews/${REVIEW_ID}`

/** bun test default per-test timeout is 5s; browser tests need more. */
const TEST_TIMEOUT = 60_000

async function goto(page: Page, path: string): Promise<void> {
  await page.goto(new URL(path, E2E_BASE_URL).toString())
}

/** bun:test wrapper around the shared traced-page harness (fresh context per test). */
function e2e(name: string, body: (page: Page) => Promise<void>): void {
  test(name, () => withTracedPage(name, body), TEST_TIMEOUT)
}

afterAll(async () => {
  await closeBrowser()
})

e2e('keeps list filters and pagination in the URL and sends PM scope headers', async (page) => {
  const api = await installDesignReviewAPI(page)
  await goto(page, listPath)

  await waitForVisible(page.getByRole('heading', { name: '设计管理' }))
  await page.getByRole('searchbox', { name: '搜索走查记录' }).fill('工作台')
  await page.getByRole('combobox', { name: '执行状态' }).click()
  await page.getByRole('option', { name: '已完成' }).click()
  await page.getByRole('button', { name: '下一页' }).click()

  await waitForUrl(page, /keyword=%E5%B7%A5%E4%BD%9C%E5%8F%B0/)
  await waitForUrl(page, /status=completed/)
  await waitForUrl(page, /page=2/)
  await expectWorkspaceHeaders(api.findRequest('GET', '/api/v1/platform/design-reviews'))
})

e2e('switches canvases, filters both markers and keeps evidence visibly rendered', async (page) => {
  await installDesignReviewAPI(page)
  await goto(page, detailPath)

  await waitForVisible(page.getByRole('heading', { name: '工作台视觉走查' }))
  await waitForVisible(page.getByAltText('Figma 设计').first())
  await page.getByRole('tab', { name: '叠加' }).click()
  await waitForCondition(async () => (await page.getByRole('slider').inputValue()) === '50')
  await page.getByRole('tab', { name: '差异热图' }).click()
  await waitForVisible(page.getByAltText('差异热图'))

  await page.getByRole('tab', { name: '并排' }).click()
  await waitForVisible(page.getByRole('button', { name: '差异 #1' }).first())
  await page.getByRole('combobox', { name: '分类' }).click()
  await page.getByRole('option', { name: '颜色' }).click()
  await waitForCount(page.getByRole('heading', { name: '布局间距不一致' }), 0)
  await waitForVisible(page.getByRole('heading', { name: '按钮颜色偏差' }))
})

e2e('creates a manual difference through the public mutation contract', async (page) => {
  const api = await installDesignReviewAPI(page)
  await goto(page, detailPath)

  await page.getByRole('button', { name: '新增人工差异' }).click()
  const dialog = page.getByRole('dialog', { name: '新增人工差异' })
  await dialog.getByLabel('差异标题').fill('导航栏高度不一致')
  await dialog.getByLabel('问题描述').fill('实现侧导航栏高出 8px。')
  await dialog.getByRole('button', { name: '保存' }).click()

  await waitForCondition(() => Boolean(
    api.findRequest('POST', '/api/v1/platform/design-review-differences'),
  ))
  const request = api.findRequest('POST', '/api/v1/platform/design-review-differences')
  expect(request?.body).toMatchObject({ review_id: REVIEW_ID, title: '导航栏高度不一致' })
})

e2e('navigates historical rounds and starts a recheck with stable deep links', async (page) => {
  await installDesignReviewAPI(page)
  await goto(page, detailPath)

  await page.getByRole('combobox', { name: '历史轮次' }).click()
  await page.getByRole('option', { name: '第 1 轮' }).click()
  await waitForUrl(page, `${listPath}/reviews/${ROOT_REVIEW_ID}`)

  await goto(page, detailPath)
  await page.getByRole('button', { name: '发起复查' }).click()
  await waitForUrl(page, new RegExp(`/workspace/design-management/reviews/new\\?.*requirement_id=101.*source_review_id=${REVIEW_ID}`))
  await waitForVisible(page.getByRole('heading', { name: '发起复查' }))
})

e2e('adds and removes keyboard-accessible alignment guides', async (page) => {
  await installDesignReviewAPI(page)
  await goto(page, detailPath)

  await page.getByRole('button', { name: '显示标尺' }).click()
  const ruler = page.getByRole('button', { name: '水平坐标标尺' }).first()
  await ruler.press('Enter')
  const guide = page.getByRole('button', { name: 'X 100 辅助线，激活以删除' }).first()
  await waitForCount(guide, 1)
  await guide.dispatchEvent('keydown', { key: 'Enter', code: 'Enter' })
  await waitForCount(page.getByRole('button', { name: 'X 100 辅助线，激活以删除' }), 0)
})

e2e('retries failed analysis on the dedicated retry endpoint', async (page) => {
  const api = await installDesignReviewAPI(page, { reviewStatus: 'failed' })
  await goto(page, detailPath)

  await page.getByRole('button', { name: '重试分析' }).click()
  await waitForCondition(() => Boolean(
    api.findRequest('POST', `/api/v1/platform/design-reviews/${REVIEW_ID}/analysis/retry`),
  ))
})

e2e('pairs and revokes a mobile collector after explicit confirmation', async (page) => {
  const api = await installDesignReviewAPI(page)
  await goto(page, `${listPath}/mobile-collectors`)

  await page.getByRole('button', { name: '生成配对码' }).click()
  await waitForVisible(page.getByText('482913'))
  await page.getByRole('button', { name: '撤销采集器' }).click()
  const dialog = page.getByRole('alertdialog', { name: '撤销移动采集器' })
  await dialog.getByRole('button', { name: '确认' }).click()
  await waitForCondition(() => Boolean(
    api.findRequest('DELETE', `/api/v1/platform/design-review-mobile-collectors/${COLLECTOR_ID}`),
  ))
})

e2e('supports Web and mobile creation modes without narrow-page overflow', async (page) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await installDesignReviewAPI(page)
  await goto(page, `${listPath}/reviews/new?requirement_id=101`)

  await waitForAttribute(page.getByRole('tab', { name: 'Web 页面' }), 'aria-selected', 'true')
  await page.getByRole('tab', { name: '移动 App' }).click()
  await waitForVisible(page.getByRole('combobox', { name: '移动采集器' }))
  await waitForVisible(page.getByRole('combobox', { name: '在线设备' }))
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})

e2e('invalidates captured evidence when Figma changes the target viewport', async (page) => {
  await installDesignReviewAPI(page)
  await goto(page, `${listPath}/reviews/new?requirement_id=101`)
  await page.getByLabel('Figma Design 节点链接').fill('https://www.figma.com/design/figma-file/Checkout?node-id=1-2')
  await page.getByLabel('实现页面地址').fill('https://implementation.example/checkout')
  await page.evaluate(() => {
    window.addEventListener('message', (event) => {
      const data = event.data as { type?: string; request_id?: string }
      if (data?.type !== 'PM_DESIGN_REVIEW_CAPTURE_REQUEST' || !data.request_id) return
      window.postMessage({
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_RESULT',
        request_id: data.request_id,
        screenshot: 'data:image/png;base64,YQ==',
        actual_url: 'https://implementation.example/checkout',
        elements: [],
        structure_truncated: true,
        warnings: [],
        viewport: { width: 1440, height: 900, devicePixelRatio: 1, pageZoom: 1 },
      }, window.origin)
    })
    window.postMessage({ source: 'pm-design-review-extension', type: 'PM_DESIGN_REVIEW_READY' }, window.origin)
  })

  await page.getByRole('button', { name: '采集嵌入页面' }).click()
  await waitForVisible(page.getByText(/implementation-capture\.png/))
  await page.getByRole('button', { name: '预览设计' }).click()

  await waitForCondition(async () => (await page.getByLabel('视口宽度').inputValue()) === '640')
  await waitForCondition(async () => (await page.getByLabel('视口高度').inputValue()) === '400')
  await waitForCount(page.getByText(/implementation-capture\.png/), 0)
  await page.getByRole('button', { name: '创建并开始分析' }).click()
  await waitForVisible(page.getByText('尚未采集或上传实现截图。请点击“采集当前目标标签页”，或手动选择图片。'))
})

e2e('fails closed for missing login and missing page permission', async (page) => {
  await installDesignReviewAPI(page, { authenticated: false })
  await goto(page, listPath)
  await waitForUrl(page, /\/login\?redirect=/)

  await page.evaluate(() => {
    window.localStorage.setItem('pm_standalone_token', 'design-review-token')
    window.localStorage.setItem('pm_workspace_id', '1')
  })
  await page.unroute('**/api/v1/**')
  await installDesignReviewAPI(page, { actionCodes: [] })
  await goto(page, listPath)
  await waitForVisible(page.getByRole('heading', { name: '身份权限不足' }))
})

e2e('runs the same detail workflow embedded without rendering the duplicate shell', async (page) => {
  await installDesignReviewAPI(page, { embedded: true })
  await goto(page, detailPath)

  await waitForVisible(page.getByRole('heading', { name: '工作台视觉走查' }))
  await waitForCount(page.getByRole('navigation'), 0)
  await waitForVisible(page.getByRole('button', { name: '差异 #1' }).first())
})

e2e('keeps row-level difference actions bound to the selected difference', async (page) => {
  const api = await installDesignReviewAPI(page)
  await goto(page, detailPath)

  const item = page.getByRole('listitem').filter({ has: page.getByRole('heading', { name: '布局间距不一致' }) })
  await item.getByRole('button', { name: '标记有效' }).click()
  await waitForCondition(() => Boolean(
    api.findRequest('POST', `/api/v1/platform/design-review-differences/${DIFFERENCE_ID}/verify`),
  ))
  expect(api.findRequest('POST', `/api/v1/platform/design-review-differences/${DIFFERENCE_ID}/verify`)?.body)
    .toMatchObject({ review_id: REVIEW_ID, status: 'confirmed' })
})
