import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '../copy'
import type { DesignDifference, DesignReviewDetail } from '../model'
import { ReviewWorkbench } from './ReviewWorkbench'

function difference(overrides: Partial<DesignDifference> = {}): DesignDifference {
  return {
    id: 'difference-1',
    review_id: 'review-2',
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
    ...overrides,
  }
}

function review(overrides: Partial<DesignReviewDetail> = {}): DesignReviewDetail {
  return {
    id: 'review-2',
    chain_root_id: 'review-1',
    source_review_id: 'review-1',
    round_number: 2,
    requirement_id: 101,
    requirement_title: '工作台视觉走查',
    project_set_id: 11,
    project_set_name: '增长产品集',
    plan_unit_id: 21,
    plan_unit_title: 'Web 2.0',
    implementation_kind: 'web',
    implementation_target: { kind: 'web', original_url: 'https://example.test/page', actual_url: 'https://example.test/page' },
    figma_url: 'https://www.figma.com/design/file?node-id=1-2',
    figma_node_name: '工作台 / Desktop',
    status: 'completed',
    conclusion: 'needs_fix',
    difference_count: 2,
    unresolved_difference_count: 1,
    created_by_user_id: 7,
    created_by_name: '设计走查员',
    created_at: '2026-09-03T08:00:00Z',
    updated_at: '2026-09-03T08:05:00Z',
    can_delete: true,
    viewport_width: 1440,
    viewport_height: 900,
    device_pixel_ratio: 2,
    page_zoom: 1,
    design_width: 1440,
    design_height: 900,
    dimensions_match: true,
    design_structure_available: true,
    implementation_structure_complete: true,
    capture_warnings: [],
    analysis_scope: { enabled: false },
    mobile_topbar: {},
    dom_normalization: { enabled: false },
    analysis_summary: '发现一项主要布局差异。',
    analysis_attempts: 1,
    assets: [],
    differences: [
      difference(),
      difference({
        id: 'difference-2',
        ordinal: 2,
        category: 'color',
        severity: 'minor',
        title: '按钮颜色偏差',
        description: '按钮颜色略有不同。',
        verification_status: 'false_positive',
        resolution_status: 'resolved',
        recheck_status: 'resolved',
        can_edit: false,
        can_delete: false,
        can_verify: false,
        can_resolve: false,
        can_reopen: true,
      }),
    ],
    rounds: [
      { id: 'review-1', round_number: 1, status: 'completed', conclusion: 'needs_fix', created_at: '2026-09-02T08:00:00Z' },
      { id: 'review-2', round_number: 2, status: 'completed', conclusion: 'needs_fix', created_at: '2026-09-03T08:00:00Z' },
    ],
    deep_review_jobs: [{
      id: 'deep-1', review_id: 'review-2', status: 'failed', created_by_user_id: 7,
      design_region: { x: 0.1, y: 0.1, width: 0.2, height: 0.2 },
      implementation_region: { x: 0.1, y: 0.1, width: 0.2, height: 0.2 },
      focus: '检查标题基线', attempts: 2, generated_difference_count: 0,
      last_error: '视觉模型暂时不可用', created_at: '2026-09-03T08:01:00Z',
      can_retry: true, can_cancel: false,
    }],
    recheck_verifications: [],
    pending_recheck_matches: [],
    capabilities: {
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
      allowed_actions: [],
    },
    ...overrides,
  }
}

function renderWorkbench(reviewValue = review(), propOverrides: Partial<React.ComponentProps<typeof ReviewWorkbench>> = {}) {
  const props: React.ComponentProps<typeof ReviewWorkbench> = {
    review: reviewValue,
    designURL: 'blob:design',
    implementationURL: 'blob:implementation',
    heatmapURL: 'blob:heatmap',
    assetState: 'idle',
    differenceState: 'idle',
    analysisState: 'idle',
    canCreate: true,
    canManageDifferences: true,
    canManageAnalysis: true,
    canDelete: true,
    onRoundChange: vi.fn(),
    onRecheck: vi.fn(),
    onRetryAnalysis: vi.fn(),
    onAbandonAnalysis: vi.fn(),
    onDeepReview: vi.fn(),
    onRetryDeepReview: vi.fn(),
    onCancelDeepReview: vi.fn(),
    onExport: vi.fn(),
    onDelete: vi.fn(),
    onCreateDifference: vi.fn(),
    onUpdateDifference: vi.fn(),
    onDeleteDifference: vi.fn(),
    onVerifyDifference: vi.fn(),
    onResolveDifference: vi.fn(),
    onReopenDifference: vi.fn(),
    onResolveRecheckMatch: vi.fn(),
    onOverrideRecheckVerification: vi.fn(),
    ...propOverrides,
  }
  render(<ReviewWorkbench {...props} />)
  return props
}

async function chooseOption(comboboxName: string, optionName: string) {
  fireEvent.click(screen.getByRole('combobox', { name: comboboxName }))
  const option = await screen.findByRole('option', { name: optionName })
  fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
  fireEvent.click(option)
}

describe('ReviewWorkbench', () => {
  it('switches between compare, overlay and difference canvases', () => {
    renderWorkbench()

    expect(screen.getByRole('tab', { name: copy.designManagement.detail.compare }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getAllByAltText(copy.designManagement.detail.designCanvas).length).toBeGreaterThan(0)
    expect(screen.getAllByAltText(copy.designManagement.detail.implementationCanvas).length).toBeGreaterThan(0)

    fireEvent.click(screen.getByRole('tab', { name: copy.designManagement.detail.overlay }))
    expect(screen.getByRole('tab', { name: copy.designManagement.detail.overlay }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('slider').getAttribute('value')).toBe('50')

    fireEvent.click(screen.getByRole('tab', { name: copy.designManagement.detail.difference }))
    expect(screen.getByRole('tab', { name: copy.designManagement.detail.difference }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByAltText(copy.designManagement.detail.difference).getAttribute('src')).toBe('blob:heatmap')
  })

  it('forces compare mode and disables composite modes when dimensions differ', () => {
    renderWorkbench(review({ dimensions_match: false, design_width: 1280 }))

    expect(screen.getByText(copy.designManagement.detail.dimensionsMismatch)).not.toBeNull()
    expect(screen.getByRole('tab', { name: copy.designManagement.detail.overlay }).getAttribute('aria-disabled')).toBe('true')
    expect(screen.getByRole('tab', { name: copy.designManagement.detail.difference }).getAttribute('aria-disabled')).toBe('true')
    expect(screen.queryByRole('slider')).toBeNull()
  })

  it('filters both the difference list and all visible canvas markers', async () => {
    renderWorkbench()

    expect(screen.getAllByRole('button', { name: copy.designManagement.differences.ordinal(1) }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole('button', { name: copy.designManagement.differences.ordinal(2) }).length).toBeGreaterThan(0)

    await chooseOption(copy.designManagement.differences.category, copy.designManagement.category.color)

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: copy.designManagement.differences.ordinal(1) })).toBeNull()
    })
    expect(screen.getAllByRole('button', { name: copy.designManagement.differences.ordinal(2) }).length).toBeGreaterThan(0)
    expect(screen.queryByRole('heading', { name: '布局间距不一致' })).toBeNull()
    expect(screen.getByRole('heading', { name: '按钮颜色偏差' })).not.toBeNull()
  })

  it('exposes only row-level difference actions and invokes their intents', () => {
    const props = renderWorkbench()
    const editableRow = screen.getByRole('heading', { name: '布局间距不一致' }).closest('li')
    const resolvedRow = screen.getByRole('heading', { name: '按钮颜色偏差' }).closest('li')
    expect(editableRow).not.toBeNull()
    expect(resolvedRow).not.toBeNull()

    fireEvent.click(within(editableRow!).getByRole('button', { name: copy.designManagement.differences.markValid }))
    fireEvent.click(within(editableRow!).getByRole('button', { name: copy.designManagement.differences.resolve }))
    fireEvent.click(within(resolvedRow!).getByRole('button', { name: copy.designManagement.differences.reopen }))

    expect(props.onVerifyDifference).toHaveBeenCalledWith('difference-1', 'confirmed')
    expect(props.onResolveDifference).toHaveBeenCalledWith('difference-1')
    expect(props.onReopenDifference).toHaveBeenCalledWith('difference-2')
    expect(within(resolvedRow!).queryByRole('button', { name: copy.designManagement.differences.edit })).toBeNull()
    expect(within(resolvedRow!).queryByRole('button', { name: copy.designManagement.common.delete })).toBeNull()
  })

  it('changes rounds, starts a recheck and exports both report formats', async () => {
    const props = renderWorkbench()

    await chooseOption(copy.designManagement.detail.rounds, copy.designManagement.detail.roundValue(1))
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.recheck }))
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.exportMarkdown }))
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.exportJson }))

    expect(props.onRoundChange).toHaveBeenCalledWith('review-1')
    expect(props.onRecheck).toHaveBeenCalledTimes(1)
    expect(props.onExport).toHaveBeenNthCalledWith(1, 'markdown')
    expect(props.onExport).toHaveBeenNthCalledWith(2, 'json')
  })

  it('allows abandoning only a failed recheck round', () => {
    const rootProps = renderWorkbench(review({ id: 'review-root', chain_root_id: 'review-root', source_review_id: undefined, round_number: 1, status: 'failed', conclusion: undefined }))
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.retryAnalysis }))
    expect(screen.queryByRole('button', { name: copy.designManagement.detail.abandonAnalysis })).toBeNull()
    expect(rootProps.onRetryAnalysis).toHaveBeenCalledTimes(1)
  })

  it('shows abandon for failed rechecks and opens deep-review history', async () => {
    const failedRecheck = review({ status: 'failed', conclusion: undefined, last_error: '分析失败' })
    const props = renderWorkbench(failedRecheck)

    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.abandonAnalysis }))
    expect(props.onAbandonAnalysis).toHaveBeenCalledTimes(1)
  })

  it('switches the narrow canvas and toggles the bottom difference drawer', () => {
    renderWorkbench()
    const implementationTabs = screen.getAllByRole('tab', { name: copy.designManagement.detail.implementationView })
    expect(implementationTabs).toHaveLength(1)
    fireEvent.click(implementationTabs[0])
    expect(implementationTabs[0].getAttribute('aria-selected')).toBe('true')

    const toggle = screen.getByRole('button', { name: copy.designManagement.detail.hideDifferences })
    const drawer = screen.getByRole('region', { name: copy.designManagement.differences.title }).parentElement
    expect(drawer?.className).toContain('max-h-[48%]')
    fireEvent.click(toggle)
    expect(screen.getByRole('button', { name: copy.designManagement.detail.showDifferences })).not.toBeNull()
    expect(drawer?.className).toContain('hidden')
    expect(drawer?.className).toContain('md:hidden')
  })

  it('opens deep review history and dispatches retry intent', async () => {
    const props = renderWorkbench()
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.deepReview }))

    const dialog = await screen.findByRole('dialog', { name: copy.designManagement.detail.deepReview })
    expect(within(dialog).getByText(copy.designManagement.detail.deepSelectionMissing)).not.toBeNull()
    expect(within(dialog).getByText('检查标题基线')).not.toBeNull()
    fireEvent.click(within(dialog).getByRole('button', { name: copy.designManagement.detail.retryDeepReview }))
    expect(props.onRetryDeepReview).toHaveBeenCalledWith('deep-1')
  })

  it('renders loading and asset-error overlays without removing the workbench', () => {
    const { rerender } = render(<ReviewWorkbench {...renderProps(review(), { assetState: 'pending' })} />)
    expect(screen.getByText(copy.designManagement.detail.assetLoading)).not.toBeNull()

    rerender(<ReviewWorkbench {...renderProps(review(), { assetState: 'error', assetMessage: '证据对象已过期' })} />)
    expect(screen.getByText('证据对象已过期')).not.toBeNull()
    expect(screen.getByText(copy.designManagement.detail.analysisSummary)).not.toBeNull()
  })
})

function renderProps(reviewValue: DesignReviewDetail, overrides: Partial<React.ComponentProps<typeof ReviewWorkbench>> = {}): React.ComponentProps<typeof ReviewWorkbench> {
  return {
    review: reviewValue,
    designURL: 'blob:design', implementationURL: 'blob:implementation', heatmapURL: 'blob:heatmap',
    assetState: 'idle', differenceState: 'idle', analysisState: 'idle',
    canCreate: true, canManageDifferences: true, canManageAnalysis: true, canDelete: true,
    onRoundChange: vi.fn(), onRecheck: vi.fn(), onRetryAnalysis: vi.fn(), onAbandonAnalysis: vi.fn(),
    onDeepReview: vi.fn(), onRetryDeepReview: vi.fn(), onCancelDeepReview: vi.fn(), onExport: vi.fn(), onDelete: vi.fn(),
    onCreateDifference: vi.fn(), onUpdateDifference: vi.fn(), onDeleteDifference: vi.fn(), onVerifyDifference: vi.fn(),
    onResolveDifference: vi.fn(), onReopenDifference: vi.fn(), onResolveRecheckMatch: vi.fn(), onOverrideRecheckVerification: vi.fn(),
    ...overrides,
  }
}
