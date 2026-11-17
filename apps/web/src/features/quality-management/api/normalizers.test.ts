import { describe, expect, it } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import {
  isNotFound,
  normalizeActivityPage,
  normalizeQualityComments,
  normalizeQualityDetail,
  normalizeQualityDashboard,
  normalizeQualityList,
  normalizeUpload,
  QualityContractError,
} from './normalizers'
import type { QualityRawPage } from './contracts'
import {
  isQualityInlineImage,
  isQualityMaterialFile,
  isQualityPasteMaterialFile,
} from '../model/payloads'
import { materialDisplayName } from '../model/presenters'

function capabilities(overrides: Record<string, unknown> = {}) {
  return {
    can_update: false,
    can_delete: false,
    allowed_actions: [],
    ...overrides,
  }
}

function bug(overrides: Record<string, unknown> = {}) {
  return {
    id: 31,
    workspace_id: 7,
    bug_key: 'BUG-31',
    title: 'Checkout crash',
    ...capabilities(),
    reproduction_materials: [
      { url: 'https://cdn.example.test/evidence.png' },
      { url: 'javascript:alert(1)' },
    ],
    ...overrides,
  }
}

function page(row = bug(), meta: Record<string, unknown> = {}): QualityRawPage {
  const body = {
    data: [row],
    total: 1,
    summary: { total: 1, open: 1 },
    ...meta,
  }
  return { body, items: [row], total: 1 }
}

describe('quality API contract normalization', () => {
  it('normalizes comments and keeps PM deletion capability', () => {
    expect(normalizeQualityComments({
      body: [],
      total: 2,
      items: [
        {
          id: 9, quality_bug_id: 31, user_id: 42, display_name: 'Teo',
          body: '请复核', created_at: '2026-08-19T10:00:00Z', can_delete: true,
        },
        { id: 0, body: '' },
      ],
    })).toEqual([{
      id: 9,
      body: '请复核',
      userId: 42,
      userName: 'Teo',
      createdAt: '2026-08-19T10:00:00Z',
      canDelete: true,
    }])
  })
  it('normalizes dashboard metrics and drops malformed dimensions', () => {
    expect(normalizeQualityDashboard({
      summary: { submitted: 4, resolved: 2, closed: 1, backlog: 3 },
      by_person: [{
        user_id: 42, user_name: 'Teo', submitted: 4, resolved: 2, closed: 1, backlog: 3,
      }],
      by_status: [{ key: 'triage', label: '待确认', count: 2 }, { key: '', count: 9 }],
      by_severity: [{ key: 'p0', label: '致命（P0）', count: 1 }],
    })).toEqual({
      summary: { submitted: 4, resolved: 2, closed: 1, backlog: 3 },
      byPerson: [{
        userId: 42, userName: 'Teo', avatarUrl: undefined,
        submitted: 4, resolved: 2, closed: 1, backlog: 3,
      }],
      byStatus: [{ key: 'triage', label: '待确认', count: 2 }],
      bySeverity: [{ key: 'p0', label: '致命（P0）', count: 1 }],
    })
  })

  it('validates workspace ownership from each PM row without requiring nonexistent page metadata', () => {
    expect(normalizeQualityList({ body: { items: [], total: 0 }, items: [], total: 0 }, 7))
      .toMatchObject({ status: 'available', rows: [], total: 0 })
    expect(normalizeQualityList(page(bug({ workspace_id: 8 })), 7)).toEqual({
      status: 'unavailable',
      reason: 'workspace_contract',
    })
  })

  it('accepts rows without legacy capability metadata', () => {
    expect(normalizeQualityList(page(bug({ allowed_actions: undefined })), 7).status)
      .toBe('available')
    expect(normalizeQualityList(page(bug({ can_update: undefined })), 7).status)
      .toBe('available')
  })

  it('treats missing or unknown defect domains as functional and preserves explicit UI', () => {
    const legacy = normalizeQualityList(page(bug()), 7)
    expect(legacy.status).toBe('available')
    if (legacy.status === 'available') expect(legacy.rows[0].defectDomain).toBe('functional')

    const ui = normalizeQualityList(page(bug({ defect_domain: 'UI' })), 7)
    expect(ui.status).toBe('available')
    if (ui.status === 'available') expect(ui.rows[0].defectDomain).toBe('ui')
  })

  it('maps an available list with explicit capabilities and sanitizes materials', () => {
    const result = normalizeQualityList(page(bug({
      status: 'done',
      ...capabilities({
        can_update: true,
        allowed_actions: ['update', 'reset_triage'],
      }),
    })), 7)

    expect(result.status).toBe('available')
    if (result.status !== 'available') return
    expect(result.rows[0].capabilities).toMatchObject({
      canUpdate: true,
      canDelete: true,
      canTransition: true,
      allowedTransitions: ['triage'],
    })
    expect(result.rows[0].reproductionMaterials).toEqual([
      { url: 'https://cdn.example.test/evidence.png' },
    ])
  })

  it('normalizes exact status facets while retaining the aggregate closed count', () => {
    const result = normalizeQualityList(page(bug(), {
      facets: { status: {
        todo: 2, triage: 3, in_progress: 4, reopened: 5, review: 6,
        done: 6, canceled: 7, closed: 13,
      } },
    }), 7)

    expect(result.status).toBe('available')
    if (result.status !== 'available') return
    expect(result.statusFacets).toEqual({
      todo: 2, triage: 3, handling: 4, reopened: 5, review: 6,
      done: 6, canceled: 7, closed: 13,
    })
  })

  it('keeps the all-status count compatible with an older aggregate-only response', () => {
    const result = normalizeQualityList(page(bug(), {
      facets: { status: { triage: 3, in_progress: 4, review: 5, closed: 6 } },
    }), 7)

    expect(result.status).toBe('available')
    if (result.status !== 'available') return
    expect(result.statusFacets).toMatchObject({
      todo: 0, triage: 3, handling: 4, reopened: 0, review: 5,
      done: 6, canceled: 0, closed: 6,
    })
  })

  it('derives lifecycle transitions from workflow actions while keeping writes available', () => {
    const detail = normalizeQualityDetail(bug({
      status: 'review',
      can_update: true,
      can_delete: true,
      allowed_actions: ['update_report', 'complete', 'return', 'delete', 'future_action'],
    }), 7)

    expect(detail.capabilities).toEqual({
      canUpdate: true,
      canDelete: true,
      canAssign: true,
      canResolve: false,
      canReturn: true,
      canTransition: true,
      canUpload: true,
      allowedTransitions: ['done'],
    })
  })

  it('rejects a detail outside the current workspace', () => {
    expect(() => normalizeQualityDetail(bug({ workspace_id: 8 }), 7)).toThrow(QualityContractError)
  })

  it('maps only a missing resource to the not-found state', () => {
    expect(isNotFound(new PMSafeApiError('FORBIDDEN', 403))).toBe(false)
    expect(isNotFound(new PMSafeApiError('NOT_FOUND', 404))).toBe(true)
    expect(isNotFound(new PMSafeApiError('FAILED', 500))).toBe(false)
  })

  it('advances activity pagination by raw rows while dropping malformed and duplicate rows', () => {
    const result = normalizeActivityPage({
      body: [],
      items: [
        { id: 1, summary: 'created' },
        { summary: 'malformed' },
        { id: 1, summary: 'duplicate' },
      ],
      total: 5,
    }, 0)
    expect(result.rows).toEqual([expect.objectContaining({ id: 1, summary: 'created' })])
    expect(result.nextOffset).toBe(3)
  })

  it('accepts only safe upload URLs', () => {
    expect(normalizeUpload({
      url: 'https://cdn.example.test/a.png',
    })).toEqual({
      url: 'https://cdn.example.test/a.png',
    })
    expect(() => normalizeUpload({
      url: 'javascript:alert(1)',
      workspace_id: 7,
    })).toThrow(QualityContractError)
  })

  it('never exposes material query credentials as the display label', () => {
    expect(materialDisplayName(
      'https://cdn.example.test/files/evidence.png?X-Amz-Signature=secret',
    )).toBe('evidence.png')
  })

  it('accepts all valid pasted materials while only small images render inline', () => {
    expect(isQualityMaterialFile(new File(['image'], 'proof.png', { type: 'image/png' }))).toBe(true)
    expect(isQualityMaterialFile(new File(['video'], 'oceans.mp4', { type: 'video/mp4' }))).toBe(true)
    expect(isQualityMaterialFile(new File(['rows'], '1.4 需求迭代.csv', { type: 'text/csv' }))).toBe(true)
    expect(isQualityMaterialFile(new File([], 'empty.png', { type: 'image/png' }))).toBe(false)
    expect(isQualityPasteMaterialFile(new File(['image'], 'proof.png', { type: 'image/png' }))).toBe(true)
    expect(isQualityPasteMaterialFile(new File(['video'], 'oceans.mp4', { type: 'video/mp4' }))).toBe(true)
    expect(isQualityPasteMaterialFile(new File(['rows'], 'tasks.csv', { type: 'text/csv' }))).toBe(true)
    expect(isQualityInlineImage(new File(['image'], 'proof.png', { type: 'image/png' }))).toBe(true)
    expect(isQualityInlineImage(new File(['video'], 'oceans.mp4', { type: 'video/mp4' }))).toBe(false)
  })
})
