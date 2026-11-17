import { describe, expect, it } from 'bun:test'

import {
  parseBiScreenSearch,
  parseDesignReviewListSearch,
  parseInboxSearch,
  parseNewDesignReviewSearch,
  parsePlanUnitSearch,
  parseProjectsSearch,
  parseQualityCreateSearch,
  parseQualityDetailSearch,
  parseQualitySearch,
  parseRequirementDetailSearch,
  parseRequirementPoolSearch,
  parseWorkItemsSearch,
} from '@/shared/router'

describe('native module Router search ownership', () => {
  it('keeps the project-set master selection on the list route', () => {
    expect(parseProjectsSearch({ project_id: '9' })).toEqual({ project_id: 9 })
    expect(parseProjectsSearch({ project_id: 'invalid' })).toEqual({})
  })

  it('keeps workflow-node deep links on project and requirement detail routes', () => {
    expect(parsePlanUnitSearch({ tab: 'overview', workflow_node: ' review ' })).toMatchObject({
      tab: 'overview',
      workflow_node: 'review',
    })
    expect(parseRequirementDetailSearch({ workflow_node: ' review ' })).toEqual({ workflow_node: 'review' })
    expect(parseRequirementDetailSearch({ workflow_node: '   ' })).toEqual({})
  })

  it('normalizes Inbox scope, search and independent feed pages', () => {
    expect(parseInboxSearch({
      project_set_ids: '2,1',
      team_id: '7',
      project_id: '9',
      q: '  overdue  ',
      workflow_page: '4',
      task_page: '2',
      bug_page: '1',
      mention_page: '-1',
    })).toEqual({
      project_set_ids: '2,1',
      team_id: 7,
      project_id: 9,
      q: 'overdue',
      workflow_page: 4,
      task_page: 2,
      invalid_scope: '1',
    })
  })

  it('keeps global work-items standalone layouts inside the real table/gantt baseline', () => {
    expect(parseWorkItemsSearch({ view: 'list', layout: 'split', page_size: '50' })).toEqual({
      view: 'list',
      layout: 'table',
      page_size: 50,
    })
    expect(parseWorkItemsSearch({ view: 'board', layout: 'gantt' })).toEqual({ view: 'board' })
    expect(parseWorkItemsSearch({ from: 'inbox' })).toEqual({
      view: 'list',
      layout: 'table',
      from: 'inbox',
    })
  })

  it('preserves authority-typed global work-items scope and assignee keys', () => {
    expect(parseWorkItemsSearch({
      view: 'list',
      team_ids: '7,3',
      classic_project_ids: '12,9',
      pm_assignee_ids: '21,8',
      one2all_assignee_ids: '34,5',
      filter_kinds: 'task,bug',
    })).toEqual({
      view: 'list',
      layout: 'table',
      team_ids: '7,3',
      classic_project_ids: '12,9',
      pm_assignee_ids: '21,8',
      one2all_assignee_ids: '34,5',
      filter_kinds: 'task,bug',
    })
  })

  it('marks malformed, duplicate and legacy scope as invalid instead of broadening it', () => {
    expect(parseBiScreenSearch({ project_set_ids: '2,bad' })).toEqual({ invalid_scope: '1' })
    expect(parseBiScreenSearch({ project_ids: '2,2' })).toEqual({ invalid_scope: '1' })
    expect(parseBiScreenSearch({ project_id: '2' })).toEqual({ project_id: 2, invalid_scope: '1' })
    expect(parseQualitySearch({ requirement_ids: ['8', '9'] })).toEqual({ invalid_scope: '1' })
    expect(parseQualityDetailSearch({ project_set_id: '3' })).toEqual({ invalid_scope: '1' })
    expect(parseWorkItemsSearch({ team_ids: '7,bad' })).toEqual({
      view: 'list', layout: 'table', invalid_scope: '1',
    })
  })

  it('normalizes BI and Quality deep-link search without server or local state', () => {
    expect(parseBiScreenSearch({
      period: 'month',
      global: 'true',
		bi_view: 'team',
		team_view: 'chart',
		team_dimension: 'month',
      team_dimension_key: '2026-08',
    })).toEqual({
      period: 'month',
		bi_view: 'team',
		team_view: 'chart',
		team_dimension: 'month',
      team_dimension_key: '2026-08',
      global: '1',
    })
    expect(parseBiScreenSearch({
      bi_view: 'team',
      team_dimension: 'week',
      team_dimension_key: '2026-W35',
    })).toEqual({
      bi_view: 'team',
      team_dimension: 'week',
      team_dimension_key: '2026-W35',
    })
    expect(parseQualitySearch({ severity_key: 'P1', page: '3', page_size: '100' })).toEqual({
      severity_key: 'P1',
      page: 3,
      page_size: 100,
    })
    expect(parseQualitySearch({ relation: 'resolved' })).toEqual({ relation: 'resolved' })
    expect(parseQualitySearch({ assigned_to_me: '1' })).toEqual({ relation: 'assigned' })
    expect(parseQualitySearch({
      responsible_by_me: '1',
      assignee_user_id: '7,9',
    })).toEqual({ assignee_user_id: '7,9', relation: 'responsible' })
    expect(parseQualityCreateSearch({ source_test_id: '8', actual_result: ' failed ' })).toEqual({
      source_test_id: 8,
      actual_result: 'failed',
    })
    expect(parseQualityDetailSearch({ edit: '1', from: 'inbox' })).toEqual({
      mode: 'edit',
      from: 'inbox',
    })
    expect(parseRequirementPoolSearch({
      q: ' 支付 ',
      project_set_ids: '11',
      project_ids: '12,13',
      unscheduled: 'true',
      current_node_id: 'product_review,__idle__',
      product_owner_user_id: '88,89',
      view: 'cards',
      page: '2',
      page_size: '50',
    })).toEqual({
      q: '支付',
      project_set_ids: '11',
      project_ids: '12,13',
      unscheduled: '1',
      current_node_id: 'product_review,__idle__',
      product_owner_user_id: '88,89',
      view: 'cards',
      page: 2,
      page_size: 50,
    })
  })

  it('normalizes design-review list filters and rejects non-contract statuses', () => {
    expect(parseDesignReviewListSearch({
      keyword: '  checkout  ',
      project_set_id: '4',
      plan_unit_id: '9',
      requirement_id: '12',
      status: 'analyzing',
      conclusion: 'needs_fix',
      page: '3',
      page_size: '50',
    })).toEqual({
      keyword: 'checkout',
      project_set_id: 4,
      plan_unit_id: 9,
      requirement_id: 12,
      status: 'analyzing',
      conclusion: 'needs_fix',
      page: 3,
      page_size: 50,
    })
    expect(parseDesignReviewListSearch({ status: 'running' })).toEqual({} as never)
  })

  it('keeps only valid requirement and review identifiers on design-review create links', () => {
    expect(parseNewDesignReviewSearch({
      requirement_id: '18',
      source_review_id: '123e4567-e89b-42d3-a456-426614174000',
    })).toEqual({
      requirement_id: 18,
      source_review_id: '123e4567-e89b-42d3-a456-426614174000',
    })
    expect(parseNewDesignReviewSearch({
      requirement_id: '-1',
      source_review_id: '../another-review',
    })).toEqual({} as never)
  })
})
