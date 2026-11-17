import { expect } from 'bun:test'

import type { Page, Request, Route } from 'playwright-core'

export const ROOT_REVIEW_ID = '11111111-1111-4111-8111-111111111111'
export const REVIEW_ID = '22222222-2222-4222-8222-222222222222'
export const DIFFERENCE_ID = '33333333-3333-4333-8333-333333333333'
export const RESOLVED_DIFFERENCE_ID = '44444444-4444-4444-8444-444444444444'
export const COLLECTOR_ID = '55555555-5555-4555-8555-555555555555'
export const CAPTURE_ID = '66666666-6666-4666-8666-666666666666'

const designReviewActions = [
  'pm.design-reviews.reviews.view',
  'pm.design-reviews.reviews.create',
  'pm.design-reviews.differences.manage',
  'pm.design-reviews.analysis.manage',
  'pm.design-reviews.reviews.delete',
  'pm.design-reviews.mobile-collectors.manage',
]

type ReviewStatus = 'completed' | 'failed'

export type DesignReviewAPIOptions = {
  authenticated?: boolean
  embedded?: boolean
  actionCodes?: string[]
  reviewStatus?: ReviewStatus
  dimensionsMatch?: boolean
}

export type RecordedRequest = {
  method: string
  pathname: string
  search: string
  headers: Record<string, string>
  body?: unknown
}

export type DesignReviewAPI = {
  requests: RecordedRequest[]
  findRequest(method: string, pathname: string): RecordedRequest | undefined
}

export async function installDesignReviewAPI(
  page: Page,
  options: DesignReviewAPIOptions = {},
): Promise<DesignReviewAPI> {
  const authenticated = options.authenticated ?? true
  if (authenticated) {
    await page.addInitScript(({ embedded, reviewId }) => {
      if (embedded) {
        const hostData = {
          contractVersion: 1,
          embedded: true,
          parentOrigin: window.location.origin,
          auth: {
            token: 'embedded-design-review-token',
            user: { id: 7, username: 'reviewer', display_name: '设计走查员', role: 'member', status: 'active' },
          },
          workspace: { id: 1 },
          route: {
            path: `/workspace/design-management/reviews/${reviewId}`,
            fullPath: `/workspace/design-management/reviews/${reviewId}`,
          },
          timezone: 'Asia/Shanghai',
        }
        window.__MICRO_APP_ENVIRONMENT__ = true
        window.microApp = {
          addDataListener(listener: (data: unknown) => void, autoTrigger?: boolean) {
            if (autoTrigger) listener(hostData)
          },
          removeDataListener() {},
          dispatch() {},
        }
        return
      }
      window.localStorage.setItem('pm_standalone_token', 'design-review-token')
      window.localStorage.setItem('pm_workspace_id', '1')
    }, { embedded: options.embedded ?? false, reviewId: REVIEW_ID })
  }

  const requests: RecordedRequest[] = []
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    requests.push({
      method: request.method(),
      pathname: url.pathname,
      search: url.search,
      headers: await request.allHeaders(),
      body: request.postData() ? parseBody(request) : undefined,
    })
    await fulfillAPI(route, request, url, options)
  })

  return {
    requests,
    findRequest: (method, pathname) => requests.find((item) => (
      item.method === method && item.pathname === pathname
    )),
  }
}

async function fulfillAPI(
  route: Route,
  request: Request,
  url: URL,
  options: DesignReviewAPIOptions,
) {
  const method = request.method()
  const path = url.pathname
  const actionCodes = options.actionCodes ?? designReviewActions

  if (method === 'GET' && path === '/api/v1/core/auth/me') {
    return json(route, { id: 7, username: 'reviewer', display_name: '设计走查员', role: 'member', status: 'active' })
  }
  if (method === 'GET' && path === '/api/v1/core/auth/me/workspaces') {
    return json(route, [{ id: 1, name: '产品研发', slug: 'product', is_default: true, enabled: true }])
  }
  if (method === 'GET' && path === '/api/v1/core/access/me/action-permissions') {
    return json(route, { action_codes: actionCodes })
  }
  if (method === 'GET' && path === '/api/v1/platform/design-reviews/capabilities') {
    return json(route, capabilities(actionCodes))
  }
  if (method === 'GET' && path === '/api/v1/platform/design-reviews/requirements') {
    return json(route, requirements)
  }
  if (method === 'GET' && path === '/api/v1/platform/design-reviews') {
    return json(route, { items: [reviewSummary(options)], page: Number(url.searchParams.get('page') || 1), page_size: Number(url.searchParams.get('page_size') || 20), total: 42 })
  }
  if (method === 'GET' && /^\/api\/v1\/platform\/design-reviews\/[^/]+\/assets\/[^/]+$/.test(path)) {
    const heatmap = path.endsWith('/asset-heatmap')
    return route.fulfill({
      status: 200,
      contentType: 'image/svg+xml',
      body: evidenceSVG(heatmap ? '#dc2626' : path.endsWith('/asset-design') ? '#2563eb' : '#059669'),
    })
  }
  const detailMatch = /^\/api\/v1\/platform\/design-reviews\/([^/]+)$/.exec(path)
  if (method === 'GET' && detailMatch) {
    return json(route, reviewDetail(detailMatch[1], options))
  }
  if (method === 'GET' && path === '/api/v1/platform/design-review-mobile-collectors') {
    return json(route, collectors)
  }
  if (method === 'GET' && path === '/api/v1/platform/design-review-mobile-captures') {
    return json(route, captures)
  }
  if (method === 'POST' && path === '/api/v1/platform/design-review-mobile-collectors/pairing-codes') {
    return json(route, { code: '482913', expires_at: '2026-09-04T10:10:00Z' })
  }
  if (method === 'POST' && path === '/api/v1/platform/design-reviews/figma-preview') {
    return json(route, {
      file_key: 'figma-file', node_id: '1:2', node_name: '工作台 / Desktop',
      image_url: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="640" height="400"%3E%3Crect width="640" height="400" fill="%232563eb"/%3E%3C/svg%3E',
      width: 640, height: 400,
    })
  }
  if (method === 'POST' && path === '/api/v1/platform/design-review-differences') {
    return json(route, differences[0])
  }
  if (method === 'POST' && path === '/api/v1/platform/design-reviews') {
    return json(route, reviewDetail(REVIEW_ID, options))
  }
  if (method === 'POST' && /\/rechecks$/.test(path)) {
    return json(route, reviewDetail(REVIEW_ID, options))
  }
  if (method === 'GET' && /\/export$/.test(path)) {
    return route.fulfill({ status: 200, contentType: 'text/plain', body: '# 设计走查报告' })
  }
  if (['POST', 'PATCH', 'DELETE'].includes(method)) return json(route, { ok: true })

  return route.fulfill({
    status: 501,
    contentType: 'application/json',
    body: JSON.stringify({ code: 'E2E_ROUTE_NOT_IMPLEMENTED', message: `${method} ${path}` }),
  })
}

function json(route: Route, data: unknown) {
  return route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ data }),
  })
}

function parseBody(request: Request): unknown {
  const contentType = request.headers()['content-type'] || ''
  if (!contentType.includes('application/json')) return request.postData()
  try {
    return request.postDataJSON()
  } catch {
    return request.postData()
  }
}

function capabilities(actionCodes: string[]) {
  const allowed = new Set(actionCodes)
  return {
    can_view: allowed.has('pm.design-reviews.reviews.view'),
    can_create: allowed.has('pm.design-reviews.reviews.create'),
    can_manage_differences: allowed.has('pm.design-reviews.differences.manage'),
    can_manage_analysis: allowed.has('pm.design-reviews.analysis.manage'),
    can_delete: allowed.has('pm.design-reviews.reviews.delete'),
    can_manage_collectors: allowed.has('pm.design-reviews.mobile-collectors.manage'),
    creation_available: true,
    figma_preview_available: true,
    analysis_available: true,
    object_storage_available: true,
    unavailable_reasons: [],
    allowed_actions: actionCodes,
  }
}

const requirements = [{
  id: 101,
  title: '工作台视觉走查',
  project_set_id: 11,
  project_set_name: '增长产品集',
  plan_unit_id: 21,
  plan_unit_title: 'Web 2.0',
}]

const differences = [{
  id: DIFFERENCE_ID,
  review_id: REVIEW_ID,
  ordinal: 1,
  source: 'ai',
  analysis_kind: 'full',
  category: 'layout',
  severity: 'major',
  title: '布局间距不一致',
  description: '卡片间距与设计稿不同。',
  recommendation: '按设计稿调整间距。',
  confidence: 0.94,
  design_region: { x: 0.1, y: 0.1, width: 0.25, height: 0.2 },
  implementation_region: { x: 0.12, y: 0.1, width: 0.25, height: 0.2 },
  evidence_basis: '视觉对比',
  evidence_notes: '双侧截图均完整。',
  verification_status: 'confirmed',
  resolution_status: 'unresolved',
  recheck_status: 'still_present',
  human_edited: false,
  created_at: '2026-09-03T08:00:00Z',
  updated_at: '2026-09-03T08:00:00Z',
  can_edit: true,
  can_delete: true,
  can_verify: true,
  can_resolve: true,
  can_reopen: false,
}, {
  id: RESOLVED_DIFFERENCE_ID,
  review_id: REVIEW_ID,
  ordinal: 2,
  source: 'manual',
  analysis_kind: 'full',
  category: 'color',
  severity: 'minor',
  title: '按钮颜色偏差',
  description: '按钮颜色略有不同。',
  recommendation: '核对主题色。',
  confidence: 1,
  design_region: { x: 0.55, y: 0.4, width: 0.2, height: 0.15 },
  implementation_region: { x: 0.55, y: 0.4, width: 0.2, height: 0.15 },
  evidence_basis: '人工判断',
  evidence_notes: '已确认属于误报。',
  verification_status: 'false_positive',
  resolution_status: 'resolved',
  recheck_status: 'resolved',
  human_edited: true,
  created_at: '2026-09-03T08:01:00Z',
  updated_at: '2026-09-03T08:01:00Z',
  can_edit: false,
  can_delete: false,
  can_verify: false,
  can_resolve: false,
  can_reopen: true,
}]

function reviewSummary(options: DesignReviewAPIOptions) {
  return {
    id: REVIEW_ID,
    chain_root_id: ROOT_REVIEW_ID,
    source_review_id: ROOT_REVIEW_ID,
    round_number: 2,
    requirement_id: 101,
    requirement_title: '工作台视觉走查',
    project_set_id: 11,
    project_set_name: '增长产品集',
    plan_unit_id: 21,
    plan_unit_title: 'Web 2.0',
    implementation_kind: 'web',
    implementation_target: { kind: 'web', original_url: 'https://example.test/workbench', actual_url: 'https://example.test/workbench' },
    figma_url: 'https://www.figma.com/design/file?node-id=1-2',
    figma_node_name: '工作台 / Desktop',
    status: options.reviewStatus ?? 'completed',
    conclusion: options.reviewStatus === 'failed' ? undefined : 'needs_fix',
    difference_count: 2,
    unresolved_difference_count: 1,
    created_by_user_id: 7,
    created_by_name: '设计走查员',
    created_at: '2026-09-03T08:00:00Z',
    updated_at: '2026-09-03T08:05:00Z',
    can_delete: true,
  }
}

function reviewDetail(id: string, options: DesignReviewAPIOptions) {
  const current = id === ROOT_REVIEW_ID ? {
    id: ROOT_REVIEW_ID,
    chain_root_id: ROOT_REVIEW_ID,
    source_review_id: undefined,
    round_number: 1,
  } : reviewSummary(options)
  return {
    ...reviewSummary(options),
    ...current,
    viewport_width: 640,
    viewport_height: 400,
    device_pixel_ratio: 2,
    page_zoom: 1,
    design_width: options.dimensionsMatch === false ? 600 : 640,
    design_height: 400,
    dimensions_match: options.dimensionsMatch ?? true,
    design_structure_available: true,
    implementation_structure_complete: true,
    capture_warnings: [],
    analysis_scope: { enabled: false },
    mobile_topbar: {},
    dom_normalization: { enabled: false },
    analysis_summary: '发现一项主要布局差异。',
    model: 'gpt-5.2',
    reasoning_effort: 'high',
    prompt_version: 'design-recheck-v1',
    response_id: 'response-e2e',
    analysis_duration_ms: 2400,
    analysis_attempts: 1,
    last_error: options.reviewStatus === 'failed' ? '视觉模型暂时不可用' : undefined,
    assets: [
      { id: 'asset-design', kind: 'design_normalized', content_type: 'image/svg+xml', byte_size: 300, sha256: 'a'.repeat(64), width: 640, height: 400, created_at: '2026-09-03T08:00:00Z' },
      { id: 'asset-implementation', kind: 'implementation_normalized', content_type: 'image/svg+xml', byte_size: 300, sha256: 'b'.repeat(64), width: 640, height: 400, created_at: '2026-09-03T08:00:00Z' },
      { id: 'asset-heatmap', kind: 'pixel_heatmap', content_type: 'image/svg+xml', byte_size: 300, sha256: 'c'.repeat(64), width: 640, height: 400, created_at: '2026-09-03T08:00:00Z' },
    ],
    differences,
    rounds: [
      { id: ROOT_REVIEW_ID, round_number: 1, status: 'completed', conclusion: 'needs_fix', created_at: '2026-09-02T08:00:00Z' },
      { id: REVIEW_ID, round_number: 2, status: options.reviewStatus ?? 'completed', conclusion: options.reviewStatus === 'failed' ? undefined : 'needs_fix', created_at: '2026-09-03T08:00:00Z' },
    ],
    deep_review_jobs: [],
    recheck_verifications: [],
    pending_recheck_matches: [],
    capabilities: capabilities(options.actionCodes ?? designReviewActions),
  }
}

const collectors = [{
  id: COLLECTOR_ID,
  name: 'MacBook 采集器',
  version: '0.1.0',
  mobilewright_version: '1.3.1',
  hostname: 'review-mac.local',
  architecture: 'arm64',
  inventory: {
    devices: [{ id: 'ios-17', name: 'iPhone 15', platform: 'ios', type: 'simulator', os_version: '17.6' }],
    doctor: { ready: true, summary: '依赖检查通过' },
  },
  doctor_ready: true,
  doctor_summary: '依赖检查通过',
  last_seen_at: '2026-09-04T09:00:00Z',
  created_at: '2026-09-03T08:00:00Z',
}]

const captures = [{
  id: CAPTURE_ID,
  collector_id: COLLECTOR_ID,
  status: 'succeeded',
  platform: 'ios',
  device_id: 'ios-17',
  app_id: 'com.example.pm',
  screenshot_available: true,
  expires_at: '2026-09-04T12:00:00Z',
  captured_at: '2026-09-04T09:02:00Z',
}]

function evidenceSVG(color: string): string {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="400" viewBox="0 0 640 400"><rect width="640" height="400" fill="${color}"/><rect x="64" y="40" width="160" height="80" fill="#ffffff" opacity="0.9"/><rect x="352" y="160" width="128" height="60" fill="#f8fafc" opacity="0.8"/></svg>`
}

export async function expectWorkspaceHeaders(request: RecordedRequest | undefined) {
  expect(request).toBeTruthy()
  expect(request?.headers.authorization).toBe('Bearer design-review-token')
  expect(request?.headers['x-workspace-id']).toBe('1')
  expect(request?.headers['x-timezone']).toBeTruthy()
}
