import { describe, expect, it } from 'bun:test'

import {
  filterDifferences,
  canvasZoomClass,
  editableMobileTarget,
  figmaPreviewErrorCopyKey,
  newReviewSubmitErrorCopyKey,
  figmaPreviewViewport,
  initialWorkbenchState,
  initialNewReviewDraft,
  newReviewValidationIssue,
  requirementScopeOptions,
  requirementScopeSelection,
  workbenchReducer,
  type DesignDifference,
} from './index'

const difference = (overrides: Partial<DesignDifference> = {}): DesignDifference => ({
  id: 'difference-1',
  review_id: 'review-1',
  ordinal: 1,
  source: 'ai',
  analysis_kind: 'full',
  category: 'layout',
  severity: 'major',
  title: 'Spacing mismatch',
  description: 'Implementation spacing differs from design.',
  recommendation: 'Use the design spacing token.',
  confidence: 0.9,
  evidence_basis: 'image',
  evidence_notes: '',
  verification_status: 'pending',
  resolution_status: 'unresolved',
  human_edited: false,
  created_at: '2026-09-04T08:00:00Z',
  updated_at: '2026-09-04T08:00:00Z',
  can_edit: false,
  can_delete: false,
  can_verify: false,
  can_resolve: false,
  can_reopen: false,
  ...overrides,
})

describe('design review presentation model', () => {
  it('combines all difference filters and treats a missing recheck status as new', () => {
    const items = [
      difference(),
      difference({
        id: 'difference-2',
        category: 'color',
        severity: 'minor',
        verification_status: 'confirmed',
        recheck_status: 'still_present',
      }),
    ]

    expect(filterDifferences(items, {
      category: 'layout',
      severity: 'major',
      verification: 'pending',
      recheck: 'new',
    })).toEqual([items[0]])
    expect(filterDifferences(items, {
      category: 'all',
      severity: 'all',
      verification: 'all',
      recheck: 'still_present',
    })).toEqual([items[1]])
  })

  it('derives project, version, and requirement options without widening selection', () => {
    const requirements = [
      { id: 11, title: 'Checkout', project_set_id: 2, project_set_name: 'Commerce', plan_unit_id: 8, plan_unit_title: 'R3' },
      { id: 12, title: 'Refund', project_set_id: 2, project_set_name: 'Commerce', plan_unit_id: 9, plan_unit_title: 'R4' },
    ]

    expect(requirementScopeOptions(requirements)).toMatchObject({
      projectSets: [{ id: 2, label: 'Commerce' }],
      projects: [
        { id: 8, label: 'R3', parentId: 2 },
        { id: 9, label: 'R4', parentId: 2 },
      ],
    })
    expect(requirementScopeSelection(12, requirements)).toEqual({
      projectSetIds: [2],
      projectIds: [9],
      requirementIds: [12],
    })
    expect(requirementScopeSelection(99, requirements)).toEqual({
      projectSetIds: [], projectIds: [], requirementIds: [],
    })
  })
})

describe('design review workbench reducer', () => {
  it('clamps zoom and keeps selection state local to the workbench', () => {
    const zoomedOut = workbenchReducer(initialWorkbenchState, { type: 'zoom', value: -2 })
    const zoomedIn = workbenchReducer(zoomedOut, { type: 'zoom', value: 9 })
    const selected = workbenchReducer(zoomedIn, { type: 'select-difference', id: 'difference-2' })

    expect(zoomedOut.zoom).toBe(0.25)
    expect(zoomedIn.zoom).toBe(2)
    expect(selected).toMatchObject({
      selectedDifferenceId: 'difference-2',
      differenceDrawerOpen: true,
    })
    expect(canvasZoomClass(0.25)).toBe('scale-[.25]')
    expect(canvasZoomClass(1.75)).toBe('scale-[1.75]')
    expect(canvasZoomClass(9)).toBe('scale-100')
  })

  it('tracks both evidence regions and clears them together', () => {
    const design = workbenchReducer(initialWorkbenchState, {
      type: 'select-region',
      side: 'design',
      region: { x: 0.1, y: 0.2, width: 0.3, height: 0.4 },
    })
    const implementation = workbenchReducer(design, {
      type: 'select-region',
      side: 'implementation',
      region: { x: 0.2, y: 0.1, width: 0.4, height: 0.3 },
    })

    expect(implementation.selections).toEqual({
      design: { x: 0.1, y: 0.2, width: 0.3, height: 0.4 },
      implementation: { x: 0.2, y: 0.1, width: 0.4, height: 0.3 },
    })
    expect(workbenchReducer(implementation, { type: 'clear-region' }).selections).toEqual({})
  })

  it('shares horizontal guides while keeping vertical guides side-specific', () => {
    const horizontal = workbenchReducer(initialWorkbenchState, {
      type: 'add-guide', side: 'design', axis: 'y', coordinate: 120,
    })
    const vertical = workbenchReducer(horizontal, {
      type: 'add-guide', side: 'implementation', axis: 'x', coordinate: 240,
    })
    const deduplicated = workbenchReducer(vertical, {
      type: 'add-guide', side: 'design', axis: 'y', coordinate: 120,
    })

    expect(vertical.guides).toEqual({
      horizontal: [120], design: [], implementation: [240],
    })
    expect(deduplicated).toBe(vertical)
    expect(workbenchReducer(vertical, {
      type: 'remove-guide', side: 'design', axis: 'y', coordinate: 120,
    }).guides.horizontal).toEqual([])
  })
})

describe('new design review evidence rules', () => {
  const preview = {
    file_key: 'figma-file',
    node_id: '1:2',
    node_name: 'Checkout',
    image_url: 'https://cdn.example/design.png',
    width: 1440,
    height: 900,
  }
  const draft = {
    ...initialNewReviewDraft,
    region: { ...initialNewReviewDraft.region },
    requirementId: 11,
    figmaUrl: 'https://www.figma.com/design/figma-file/Checkout?node-id=1-2',
    implementationUrl: 'https://implementation.example/checkout',
  }
  const evidence = {
    extension: 'captured' as const,
    implementation: new File(['image'], 'implementation.png', { type: 'image/png' }),
  }

  it('maps Figma preview failures to actionable copy categories', () => {
    expect(figmaPreviewErrorCopyKey(400)).toBe('invalidFigma')
    expect(figmaPreviewErrorCopyKey(403)).toBe('figmaPreviewForbidden')
    expect(figmaPreviewErrorCopyKey(404)).toBe('figmaPreviewNotFound')
    expect(figmaPreviewErrorCopyKey(503)).toBe('figmaPreviewUnavailable')
    expect(figmaPreviewErrorCopyKey(undefined)).toBe('figmaPreviewUnavailable')
    expect(figmaPreviewErrorCopyKey(409)).toBe('figmaPreviewFailed')
  })

  it('maps review submission failures to actionable copy categories', () => {
    expect(newReviewSubmitErrorCopyKey(400)).toBe('submitInvalid')
    expect(newReviewSubmitErrorCopyKey(403)).toBe('submitForbidden')
    expect(newReviewSubmitErrorCopyKey(409)).toBe('submitConflict')
    expect(newReviewSubmitErrorCopyKey(503)).toBe('submitUnavailable')
    expect(newReviewSubmitErrorCopyKey(undefined)).toBe('submitUnavailable')
    expect(newReviewSubmitErrorCopyKey(422)).toBe('submitFailed')
  })

  it('accepts matching web evidence and rejects undersized or out-of-bounds scopes', () => {
    expect(newReviewValidationIssue(draft, evidence, preview)).toBeUndefined()
    expect(newReviewValidationIssue({
      ...draft,
      scoped: true,
      region: { x: 0.1, y: 0.1, width: 0.001, height: 0.001 },
    }, evidence, preview)).toBe('invalidRegion')
    expect(newReviewValidationIssue({
      ...draft,
      scoped: true,
      region: { x: 0.9, y: 0.1, width: 0.2, height: 0.2 },
    }, evidence, preview)).toBe('invalidRegion')
  })

  it('rejects captured evidence after the target viewport changes', () => {
    expect(newReviewValidationIssue(draft, {
      ...evidence,
      captureViewport: { width: 390, height: 844, devicePixelRatio: 1 },
    }, preview)).toBe('evidenceViewportMismatch')
    expect(newReviewValidationIssue(draft, {
      ...evidence,
      captureViewport: { width: 1440, height: 900, devicePixelRatio: 1 },
    }, preview)).toBeUndefined()
  })

  it('distinguishes an unconfirmed Figma preview from a missing implementation screenshot', () => {
    expect(newReviewValidationIssue(draft, evidence)).toBe('figmaEvidenceRequired')
    expect(newReviewValidationIssue(draft, { extension: 'connected' }, preview)).toBe('implementationEvidenceRequired')
  })

  it('normalizes a matching Figma node size into a valid implementation viewport', () => {
    expect(figmaPreviewViewport({ ...preview, width: 389.6, height: 843.7 }, draft.figmaUrl)).toEqual({
      viewportWidth: 390,
      viewportHeight: 844,
    })
    expect(figmaPreviewViewport(preview, 'https://www.figma.com/design/other/Checkout?node-id=1-2')).toBeUndefined()
    expect(figmaPreviewViewport({ ...preview, width: 120 }, draft.figmaUrl)).toBeUndefined()
    expect(figmaPreviewViewport({ ...preview, height: Number.NaN }, draft.figmaUrl)).toBeUndefined()
  })

  it('writes editable mobile metadata without discarding collector evidence facts', () => {
    const target = editableMobileTarget({
      kind: 'mobile',
      platform: 'ios',
      app_id: 'old.app',
      app_name: 'Old',
      orientation: 'portrait',
      collector_version: '0.1.0',
      mobilewright_version: '1.0.0',
      device: { id: 'phone-1' },
    }, {
      ...draft,
      mode: 'mobile',
      platform: 'android',
      appId: ' com.example.checkout ',
      appName: ' Checkout ',
      appVersion: ' 2.3.0 ',
      orientation: 'landscape',
    })

    expect(target).toMatchObject({
      kind: 'mobile',
      platform: 'android',
      app_id: 'com.example.checkout',
      app_name: 'Checkout',
      app_version: '2.3.0',
      orientation: 'landscape',
      collector_version: '0.1.0',
      device: { id: 'phone-1' },
    })
  })
})
