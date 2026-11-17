import { describe, expect, it, vi } from 'bun:test'

import { resolveDesignReviewAccess } from './useDesignReviewAccess'
import type { ReviewCapabilities } from '../model'

const localCapabilities = (overrides: Partial<ReviewCapabilities> = {}): ReviewCapabilities => ({
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
  ...overrides,
})

describe('design review capability resolution', () => {
  it('fails closed until a local capability response exists', () => {
    const allows = vi.fn((_action: string, available = true) => available)

    expect(resolveDesignReviewAccess(allows)).toEqual({
      canView: false,
      canCreate: false,
      canCreateNow: false,
      canManageDifferences: false,
      canManageAnalysis: false,
      canAnalyzeNow: false,
      canDelete: false,
      canManageCollectors: false,
    })
  })

  it('requires both the global Action and the server-resolved local capability', () => {
    const allows = vi.fn((action: string, available = true) => (
      !action.endsWith('reviews.delete') && available
    ))
    const resolved = resolveDesignReviewAccess(allows, localCapabilities({
      can_manage_differences: false,
      analysis_available: false,
    }))

    expect(resolved).toMatchObject({
      canView: true,
      canCreate: true,
      canCreateNow: true,
      canManageDifferences: false,
      canManageAnalysis: true,
      canAnalyzeNow: false,
      canDelete: false,
      canManageCollectors: true,
    })
  })
})
