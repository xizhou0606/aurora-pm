import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { requirementRoleLanes } from '../utils/requirement-role-owners'
import {
  applyRequirementPoolScopeSelection,
  availableRequirementPoolFilterFields,
  clearRequirementPoolFilter,
  emptyRequirementPoolFilterValue,
  REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
  requirementMatchesOwnerFilter,
  requirementMatchesScopeFilter,
  requirementMatchesSearchFilter,
  requirementPoolFilterFields,
  requirementPoolFilterHasValue,
  requirementPoolFiltersFromSearch,
  requirementPoolQueryFilters,
  requirementPoolRoleQueryKey,
  requirementPoolScopeQuery,
  requirementPoolScopeSelection,
  requirementPoolSearchFromFilters,
  toggleRequirementPoolFilter,
  toggleRequirementPoolUnscheduled,
} from '../utils/requirement-pool-filter-fields'

const poolCopy = copy.aiPM.requirementPool

describe('requirement pool filter fields', () => {
  it('keeps scope in filter semantics but outside configurable toolbar controls', () => {
    expect(requirementPoolFilterFields.map((field) => [field.id, field.columnId, field.label])).toEqual([
      ['scope', 'scope', poolCopy.scopeLevel],
      ['workflow', 'workflow', poolCopy.workflowOverview],
      ...requirementRoleLanes.map((lane) => [lane.id, lane.id, lane.label]),
    ])
    expect(REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.map((option) => option.id)).toEqual([
      'search',
      'workflow',
      ...requirementRoleLanes.map((lane) => lane.id),
    ])
    expect(REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS[0]?.required).toBe(true)
    expect(requirementPoolRoleQueryKey('role-ui')).toBe('design_owner_user_id')
    expect(requirementPoolRoleQueryKey('role-tester')).toBe('tester_user_id')
  })

  it('toggles multiple values and hides fields that already have a chip', () => {
    const withNodes = toggleRequirementPoolFilter(
      toggleRequirementPoolFilter(emptyRequirementPoolFilterValue, 'workflow', 'review'),
      'workflow',
      'dev',
    )
    expect(withNodes.currentNodeIds).toEqual(['review', 'dev'])
    expect(availableRequirementPoolFilterFields(withNodes).map((field) => field.id)).not.toContain('workflow')
    expect(toggleRequirementPoolFilter(withNodes, 'workflow', 'review').currentNodeIds).toEqual(['dev'])
    expect(clearRequirementPoolFilter(withNodes, 'workflow')).toEqual(emptyRequirementPoolFilterValue)
    expect(requirementPoolFilterHasValue(withNodes)).toBe(true)
    expect(requirementPoolFilterHasValue(emptyRequirementPoolFilterValue)).toBe(false)
  })

  it('maps a cascader path onto one scope chip and keeps unscheduled independently', () => {
    const planUnits = [{ id: 12, parentId: 11 }, { id: 13, parentId: 11 }, { id: 21, parentId: 99 }]
    const selected = applyRequirementPoolScopeSelection(
      emptyRequirementPoolFilterValue,
      { projectSetIds: [11], projectIds: [12, 13], requirementIds: [] },
    )
    expect(requirementPoolScopeSelection(selected)).toEqual({
      projectSetIds: [11],
      projectIds: [12, 13],
      requirementIds: [],
    })
    expect(requirementPoolScopeQuery(selected, planUnits)).toEqual({
      projectId: '11',
      planUnit: undefined,
    })
    expect(requirementPoolScopeQuery(
      applyRequirementPoolScopeSelection(
        emptyRequirementPoolFilterValue,
        { projectSetIds: [11], projectIds: [12], requirementIds: [] },
      ),
      planUnits,
    )).toEqual({
      projectId: undefined,
      planUnit: '12',
    })
    expect(requirementPoolScopeQuery(
      toggleRequirementPoolUnscheduled(selected),
      planUnits,
    )).toEqual({
      projectId: '11',
      planUnit: 'none',
    })
  })

  it('matches search, scope, and role owners on the client when the API ignores filters', () => {
    const planUnits = [{ id: 12, parentId: 11 }, { id: 13, parentId: 11 }]
    const owned = {
      id: 42,
      title: '支付流程改造',
      project_new_id: 11,
      version_task_new_id: 12,
      product_owner_user_ids: [88, 89],
      design_owner_user_ids: [21],
    }
    const other = {
      id: 99,
      title: '别人的需求',
      project_new_id: 22,
      version_task_new_id: 23,
      product_owner_user_id: 99,
    }
    expect(requirementMatchesSearchFilter(owned, '42')).toBe(true)
    expect(requirementMatchesSearchFilter(owned, '支付')).toBe(true)
    expect(requirementMatchesSearchFilter(owned, '别人')).toBe(false)

    const scoped = applyRequirementPoolScopeSelection(
      emptyRequirementPoolFilterValue,
      { projectSetIds: [11], projectIds: [12], requirementIds: [] },
    )
    expect(requirementMatchesScopeFilter(owned, scoped, planUnits)).toBe(true)
    expect(requirementMatchesScopeFilter(other, scoped, planUnits)).toBe(false)
    expect(requirementMatchesScopeFilter(owned, emptyRequirementPoolFilterValue, planUnits)).toBe(true)

    const product = { ...emptyRequirementPoolFilterValue, productOwnerUserIds: [88, 77] }
    expect(requirementMatchesOwnerFilter(owned, product)).toBe(true)
    expect(requirementMatchesOwnerFilter(other, product)).toBe(false)
    expect(requirementMatchesOwnerFilter(owned, { ...product, designOwnerUserIds: [21] })).toBe(true)
    expect(requirementMatchesOwnerFilter(owned, { ...product, designOwnerUserIds: [99] })).toBe(false)
  })

  it('round-trips filter chips into a shareable search state like quality management', () => {
    const selected = toggleRequirementPoolUnscheduled({
      ...emptyRequirementPoolFilterValue,
      q: '支付',
      projectIds: [11],
      planUnitIds: ['12'],
      currentNodeIds: ['product_review', '__idle__'],
      productOwnerUserIds: [88, 89],
    })
    const search = requirementPoolSearchFromFilters(selected, { page: 2, pageSize: 50, view: 'cards' })
    expect(search).toEqual({
      q: '支付',
      project_set_ids: '11',
      project_ids: '12',
      unscheduled: '1',
      current_node_id: 'product_review,__idle__',
      product_owner_user_id: '88,89',
      view: 'cards',
      page: 2,
      page_size: 50,
    })
    expect(requirementPoolFiltersFromSearch(search)).toEqual({
      ...emptyRequirementPoolFilterValue,
      q: '支付',
      projectIds: [11],
      planUnitIds: ['none', '12'],
      currentNodeIds: ['product_review', '__idle__'],
      productOwnerUserIds: [88, 89],
    })
  })

  it('builds identical pool request filters for the page and the route loader', () => {
    const planUnits = [{ id: 12, parentId: 11 }, { id: 13, parentId: 11 }, { id: 21, parentId: 99 }]
    const filters = {
      ...emptyRequirementPoolFilterValue,
      q: '  支付  ',
      projectIds: [11],
      planUnitIds: ['12'],
      currentNodeIds: ['product_review'],
      productOwnerUserIds: [88, 89],
      testerUserIds: [5],
    }

    expect(requirementPoolQueryFilters({
      filters,
      projects: planUnits,
      view: 'list',
      page: 3,
      pageSize: 50,
    })).toEqual({
      q: '支付',
      // Set 11 stays unselected (unit 13 not chosen), so filtering falls to plan units.
      projectId: undefined,
      planUnit: '12',
      currentNodeId: 'product_review',
      productOwnerUserId: '88,89',
      designOwnerUserId: undefined,
      frontendOwnerUserId: undefined,
      backendOwnerUserId: undefined,
      testerUserId: '5',
      limit: 50,
      offset: 100,
    })

    // Selecting every unit of set 11 collapses the scope to the project set id.
    expect(requirementPoolQueryFilters({
      filters: { ...filters, planUnitIds: ['12', '13'] },
      projects: planUnits,
      view: 'list',
      page: 1,
      pageSize: 20,
    })).toMatchObject({ projectId: '11', planUnit: undefined })

    // Cards view fetches everything; the loader must agree so one cache entry serves both.
    expect(requirementPoolQueryFilters({
      filters,
      projects: planUnits,
      view: 'cards',
      page: 1,
      pageSize: 20,
    }).limit).toBe('all')

    // Without scope data, the set counts as fully selected and the unit filter
    // stays, matching what the page computed before the shared builder.
    expect(requirementPoolQueryFilters({
      filters,
      projects: [],
      view: 'list',
      page: 1,
      pageSize: 20,
    })).toMatchObject({ projectId: '11', planUnit: '12' })

    expect(requirementPoolQueryFilters({
      filters: emptyRequirementPoolFilterValue,
      projects: planUnits,
      view: 'list',
      page: 1,
      pageSize: 20,
    })).toEqual({
      q: undefined,
      projectId: undefined,
      planUnit: undefined,
      currentNodeId: undefined,
      productOwnerUserId: undefined,
      designOwnerUserId: undefined,
      frontendOwnerUserId: undefined,
      backendOwnerUserId: undefined,
      testerUserId: undefined,
      limit: 20,
      offset: 0,
    })
  })
})
