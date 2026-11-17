import { describe, expect, it } from 'bun:test'
import { PM_ACTION } from '@/generated/access-catalog'
import { resourceActionCode, resourceMenuActions } from './resourceActions'
import type { Resource } from './types'

const resource: Resource = { id: 'r', name: 'Table', workspaceId: 1, revision: 1, tables: [], allowedActions: [PM_ACTION.BitablesResourcesView, ...Object.values(resourceActionCode)] }
describe('directory actions intersect global and resource authority', () => {
  it('keeps readable links but hides mutations without object metadata or permission', () => {
    expect(resourceMenuActions({ ...resource, revision: undefined }, undefined, () => true)).toEqual(['open', 'share', 'permissions', 'copyLink'])
    expect(resourceMenuActions(resource, undefined, action => action === PM_ACTION.BitablesResourcesView)).toEqual(['open', 'share', 'permissions', 'copyLink'])
    expect(resourceMenuActions({ ...resource, allowedActions: [] }, undefined, () => true)).toEqual([])
  })
  it('restricts shortcuts to location and shortcut deletion rather than mutating the original table', () => {
    expect(resourceMenuActions(resource, { id: 's', targetResourceId: 'r', revision: 1, name: 'Link' }, () => true)).toEqual(['open', 'share', 'permissions', 'copyLink', 'location', 'delete'])
  })
  it('exposes the requested real resource actions when both authority sources allow them', () => {
    expect(resourceMenuActions(resource, undefined, () => true)).toEqual(['open', 'share', 'permissions', 'copyLink', 'duplicate', 'shortcuts', 'pin', 'favorite', 'owner', 'location', 'name', 'delete'])
  })
})
