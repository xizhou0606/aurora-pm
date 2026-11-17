import { describe, expect, it } from 'bun:test'

import { defaultRequirementRoleOwners, ownerIDs, requirementRoleOwners, sameOwnerIDs } from '../utils/requirement-role-owners'

describe('requirement role owners', () => {
  it('prefers the multi-select list and falls back to the legacy scalar', () => {
    expect(ownerIDs([2, 2, 0, 3], 1)).toEqual([2, 3])
    expect(ownerIDs([], 9)).toEqual([9])
    expect(ownerIDs(undefined, null)).toEqual([])
  })

  it('defaults product to the creator and keeps other roles empty', () => {
    expect(defaultRequirementRoleOwners(9)).toEqual({
      product_owner_user_ids: [9],
      design_owner_user_ids: [],
      frontend_owner_user_ids: [],
      backend_owner_user_ids: [],
      tester_user_ids: [],
    })
  })

  it('reads mixed scalar and array payloads from a requirement', () => {
    expect(requirementRoleOwners({
      product_owner_user_id: 1,
      design_owner_user_ids: [8, 8],
      frontend_owner_user_id: 3,
      backend_owner_user_ids: [4],
      tester_user_ids: [5, 6],
    })).toEqual({
      product_owner_user_ids: [1],
      design_owner_user_ids: [8],
      frontend_owner_user_ids: [3],
      backend_owner_user_ids: [4],
      tester_user_ids: [5, 6],
    })
    expect(sameOwnerIDs([1, 2], [1, 2])).toBe(true)
    expect(sameOwnerIDs([1, 2], [2, 1])).toBe(false)
  })
})
