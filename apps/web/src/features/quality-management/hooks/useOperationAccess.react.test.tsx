import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import type { QualityBug } from '../model/types'
import { requireQualityOperation, useQualityOperationAccess } from './useOperationAccess'

const access = (() => ({ grants: new Set<string>() }))()

vi.mock('@/features/access-control', () => ({
  useOperationCapabilities: () => ({
    status: 'ready' as const,
    retry: vi.fn(),
    allows: (action: string, locallyAvailable = true) => (
      access.grants.has(action) && locallyAvailable
    ),
  }),
}))

function bug(capabilities: Partial<QualityBug['capabilities']> = {}): QualityBug {
  return {
    id: 31,
    workspaceId: 7,
    title: 'Checkout crash',
    reproductionMaterials: [],
    resolutionMaterials: [],
    capabilities: {
      canUpdate: true,
      canDelete: true,
      canAssign: true,
      canResolve: true,
      canReturn: true,
      canTransition: true,
      canUpload: true,
      allowedTransitions: ['done'],
      ...capabilities,
    },
  }
}

describe('useQualityOperationAccess', () => {
  beforeEach(() => access.grants.clear())

  it('combines global actions with object and business capabilities', () => {
    access.grants.add(PM_ACTION.QualityBugsUpdate)
    access.grants.add(PM_ACTION.QualityCommentsDelete)
    access.grants.add(PM_ACTION.QualitySceneOptionsDelete)
    const { result } = renderHook(useQualityOperationAccess)

    expect(result.current.canUpdateBug(bug())).toBe(true)
    expect(result.current.canUpdateBug(bug({ canUpdate: false }))).toBe(false)
    expect(result.current.canDeleteBug(bug())).toBe(false)
    expect(result.current.canDeleteComment({ id: 1, body: 'mine', canDelete: true })).toBe(true)
    expect(result.current.canDeleteComment({ id: 2, body: 'other', canDelete: false })).toBe(false)
    expect(result.current.canDeleteSceneOption({
      id: 3, name: 'staging', builtIn: false, legacy: false, usageCount: 0,
    })).toBe(true)
    expect(result.current.canDeleteSceneOption({
      id: 4, name: 'prod', builtIn: false, legacy: false, usageCount: 2,
    })).toBe(false)
  })

  it('keeps transition-only roles out of sensitive edit and delete operations', () => {
    access.grants.add(PM_ACTION.QualityBugsTransitionStatus)
    const { result } = renderHook(useQualityOperationAccess)

    expect(result.current.canTransitionBug(bug())).toBe(true)
    expect(result.current.canTransitionBug(bug({
      canTransition: false, canResolve: false, canReturn: true,
    }))).toBe(true)
    expect(result.current.canResolveBug(bug())).toBe(true)
    expect(result.current.canReturnBug(bug())).toBe(true)
    expect(result.current.canAssignBug(bug())).toBe(true)
    expect(result.current.canUpdateBug(bug())).toBe(false)
    expect(result.current.canDeleteBug(bug())).toBe(false)
  })

  it('raises a safe 403 before a denied mutation can reach the client', () => {
    expect(() => requireQualityOperation(false)).toThrow(expect.objectContaining({
      code: 'QUALITY_ACTION_PERMISSION_REQUIRED',
      status: 403,
    }))
  })
})
