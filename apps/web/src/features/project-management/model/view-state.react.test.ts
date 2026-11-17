import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ScrumBoardNew,
  ScrumProjectAuditLogNew,
  ScrumProjectNew,
} from '../api'
import {
  resolveActivityState,
  resolvePlanUnitTab,
  resolveProjectListState,
  resolveProjectScreenState,
} from './view-state'
import { formatPMDateRange } from './selectors'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'Project Nine',
  status: 'active',
  owner_user_id: 11,
}

const board: ScrumBoardNew = {
  project,
  version_tasks: [{
    id: 31,
    team_id: 2,
    project_new_id: 9,
    title: 'Release Search',
    type: 'version',
    owner_user_id: 11,
    starts_at: '2026-08-01',
    due_at: '2026-08-31',
    status: 'development',
  }],
  requirements: [],
  tasks: [],
}

const users = [{ id: 11, username: 'teo', display_name: 'Teo' }]

describe('P5 project list state machine', () => {
  it('makes loading, error and both empty reasons mutually exclusive', () => {
    const retry = vi.fn()
    const common = {
      projects: [],
      total: 0,
      fetching: false,
      selectedProjectId: undefined,
      users,
      retry,
    }

    expect(resolveProjectListState({ ...common, status: 'pending', search: '' }))
      .toMatchObject({ status: 'loading' })
    expect(resolveProjectListState({ ...common, status: 'error', search: '' }))
      .toMatchObject({ status: 'error', retry })
    expect(resolveProjectListState({ ...common, status: 'success', search: '' }))
      .toMatchObject({ status: 'empty', reason: 'initial' })
    expect(resolveProjectListState({ ...common, status: 'success', search: 'nine' }))
      .toMatchObject({ status: 'empty', reason: 'filtered' })
  })

  it('resolves ready rows, selection, enrichment and refreshing once', () => {
    const state = resolveProjectListState({
      status: 'success',
      projects: [project],
      total: 4,
      fetching: true,
      search: '',
      selectedProjectId: 9,
      users,
      retry: vi.fn(),
    })

    expect(state.status).toBe('ready')
    if (state.status !== 'ready') throw new Error('expected ready state')
    expect(state.refreshing).toBe(true)
    expect(state.items[0]).toMatchObject({
      owner: { userId: 11, name: 'Teo', secondary: 'teo' },
      selected: true,
    })
  })
})

describe('P5 activity state machine', () => {
  const rows: ScrumProjectAuditLogNew[] = [
    { id: 1, user_id: 11, resource: 'version', resource_id: 31, action: 'update' },
    { id: 2, user_id: 11, resource: 'version', resource_id: 32, action: 'update' },
  ]
  const facts = { requirements: [], tasks: [], bugs: [] }

  it('preserves retry and scopes plan-unit activity before display mapping', () => {
    const retry = vi.fn()
    expect(resolveActivityState({
      scope: 'project',
      audit: { status: 'error', error: new Error('offline'), fetching: false },
      users,
      retry,
    })).toMatchObject({ status: 'error', scope: 'project', retry })

    const state = resolveActivityState({
      scope: 'plan-unit',
      planUnitId: 31,
      audit: { status: 'success', data: rows, fetching: true },
      facts: { status: 'success', data: facts, fetching: false },
      users,
      retry,
    })
    expect(state.status).toBe('ready')
    if (state.status !== 'ready') throw new Error('expected ready state')
    expect(state.rows).toHaveLength(1)
    expect(state.rows[0]).toMatchObject({
      id: 1,
      actor: { userId: 11, name: 'Teo', secondary: 'teo' },
    })
    expect(state.refresh).toEqual({ status: 'refreshing' })
  })

  it('distinguishes loading and scoped empty states', () => {
    expect(resolveActivityState({
      scope: 'project',
      audit: { status: 'pending', fetching: false },
      users: [],
      retry: vi.fn(),
    })).toEqual({ status: 'loading', scope: 'project' })
    expect(resolveActivityState({
      scope: 'plan-unit',
      planUnitId: 99,
      audit: { status: 'success', data: rows, fetching: false },
      facts: { status: 'success', data: facts, fetching: false },
      users: [],
      retry: vi.fn(),
    })).toEqual({
      status: 'empty',
      scope: 'plan-unit',
      refresh: { status: 'idle' },
    })
  })

  it('keeps cached rows only for transport/5xx stale errors', () => {
    const retry = vi.fn()
    const stale = resolveActivityState({
      scope: 'plan-unit',
      planUnitId: 31,
      audit: {
        status: 'error',
        data: rows,
        error: new PMSafeApiError('AUDIT_UNAVAILABLE', 503),
        fetching: false,
      },
      facts: { status: 'success', data: facts, fetching: false },
      users: [],
      retry,
    })
    expect(stale).toMatchObject({
      status: 'ready',
      rows: [{ id: 1 }],
      refresh: { status: 'stale_error', retry },
    })

    for (const status of [403, 404, 409]) {
      expect(resolveActivityState({
        scope: 'project',
        audit: {
          status: 'error',
          data: rows,
          error: new PMSafeApiError('AUDIT_SCOPE_REJECTED', status),
          fetching: false,
        },
        users: [],
        retry,
      })).toEqual({ status: 'error', scope: 'project', retry })
    }
  })

  it('requires item aggregate facts and preserves all project audit rows', () => {
    const retry = vi.fn()
    expect(resolveActivityState({
      scope: 'plan-unit',
      planUnitId: 31,
      audit: { status: 'success', data: rows, fetching: false },
      facts: { status: 'pending', fetching: false },
      users: [],
      retry,
    })).toEqual({ status: 'loading', scope: 'plan-unit' })

    const projectState = resolveActivityState({
      scope: 'project',
      audit: { status: 'success', data: rows, fetching: false },
      users: [],
      retry,
    })
    expect(projectState.status).toBe('ready')
    if (projectState.status !== 'ready') throw new Error('expected project activity')
    expect(projectState.rows.map((row) => row.id)).toEqual([1, 2])
  })
})

describe('P5 project screen state machine', () => {
  const retry = vi.fn()
  const resolve = (
    route: Parameters<typeof resolveProjectScreenState>[0]['route'],
    status: Parameters<typeof resolveProjectScreenState>[0]['status'] = 'success',
    source: ScrumBoardNew | undefined = board,
    error?: unknown,
    fetching = false,
  ) => resolveProjectScreenState({
    route,
    status,
    board: source,
    error,
    fetching,
    users,
    retry,
  })

  it('preserves workspace users on project list resources', () => {
    const state = resolveProjectScreenState({
      route: { kind: 'project', projectId: 9, tab: 'items' },
      status: 'success',
      board,
      fetching: false,
      users,
      retry,
    })

    expect(state).toMatchObject({
      status: 'ready',
      resource: {
        kind: 'project',
      },
    })
    if (state.status !== 'ready' || state.resource.kind !== 'project') {
      throw new Error('expected a ready project resource')
    }
    expect(state.resource.content).toMatchObject({ kind: 'items', users })
  })

  it('covers selection, lifecycle failures and not-found resources', () => {
    expect(resolve({ kind: 'list' }, 'pending')).toEqual({ status: 'select' })
    expect(resolveProjectScreenState({
      route: { kind: 'project', projectId: 9, tab: 'overview' },
      status: 'pending',
      board: undefined,
      fetching: false,
      users,
      retry,
    })).toEqual({ status: 'loading' })
    expect(resolveProjectScreenState({
      route: { kind: 'project', projectId: 9, tab: 'overview' },
      status: 'error',
      board: undefined,
      error: new PMSafeApiError('STALE_FORBIDDEN', 403),
      fetching: true,
      users,
      retry,
    })).toEqual({ status: 'loading' })
    expect(resolve({ kind: 'project', projectId: 9, tab: 'overview' }, 'error'))
      .toEqual({ status: 'error', retry })
    expect(resolve({ kind: 'project', projectId: 10, tab: 'overview' }))
      .toEqual({ status: 'project-not-found' })
    expect(resolve({
      kind: 'plan-unit',
      projectId: 9,
      planUnitId: 99,
      tab: 'overview',
    })).toEqual({ status: 'item-not-found', projectId: 9, planUnitId: 99 })
  })

  it('builds exhaustive project and plan-unit ready content models', () => {
    const projectState = resolve({ kind: 'project', projectId: 9, tab: 'overview' })
    expect(projectState.status).toBe('ready')
    if (projectState.status !== 'ready' || projectState.resource.kind !== 'project') {
      throw new Error('expected project ready state')
    }
    expect(projectState.resource.content).toMatchObject({ kind: 'overview', users })

    const itemState = resolve({
      kind: 'plan-unit',
      projectId: 9,
      planUnitId: 31,
      tab: 'activity',
    })
    expect(itemState.status).toBe('ready')
    if (itemState.status !== 'ready' || itemState.resource.kind !== 'plan-unit') {
      throw new Error('expected plan-unit ready state')
    }
    expect(itemState.resource.content).toEqual({
      kind: 'activity',
      projectId: 9,
      planUnitId: 31,
    })

    const deliveryState = resolve({
      kind: 'plan-unit',
      projectId: 9,
      planUnitId: 31,
      tab: 'delivery',
    })
    expect(deliveryState.status).toBe('ready')
    if (deliveryState.status !== 'ready' || deliveryState.resource.kind !== 'plan-unit') {
      throw new Error('expected plan-unit delivery state')
    }
    expect(deliveryState.resource.content).toMatchObject({
      kind: 'delivery',
    })
    expect(deliveryState.resource.header.meta).toEqual(expect.arrayContaining([
      `${copy.projectManagement.projects}：${project.name}`,
      `${copy.projectManagement.planUnitDetail.status}：${copy.projectManagement.statusLabels.development}`,
      `${copy.projectManagement.planUnitDetail.dateRange}：${formatPMDateRange('2026-08-01', '2026-08-31')}`,
    ]))
    expect(deliveryState.resource.header.meta).toHaveLength(3)
    expect(deliveryState.resource.header.owner).toEqual({
      label: copy.projectManagement.planUnitDetail.owner,
      preview: { userId: 11, name: 'Teo', secondary: 'teo' },
    })
  })

  it('keeps a cached board on transport or 5xx refresh failures', () => {
    const unavailable = resolve(
      { kind: 'project', projectId: 9, tab: 'overview' },
      'error',
      board,
      new PMSafeApiError('BOARD_UNAVAILABLE', 503),
    )
    expect(unavailable.status).toBe('ready')
    if (unavailable.status !== 'ready') throw new Error('expected cached project board')
    expect(unavailable.resource.project.id).toBe(project.id)
    expect(unavailable.refresh).toEqual({ status: 'stale_error', retry })

    const refreshing = resolve(
      { kind: 'project', projectId: 9, tab: 'items' },
      'success',
      board,
      undefined,
      true,
    )
    expect(refreshing).toMatchObject({
      status: 'ready',
      refresh: { status: 'refreshing' },
    })
  })

  it.each([403, 404, 409])(
    'fails closed for a cached board when refresh returns %s',
    (status) => {
      expect(resolve(
        { kind: 'project', projectId: 9, tab: 'overview' },
        'error',
        board,
        new PMSafeApiError('BOARD_SCOPE_REJECTED', status),
      )).toEqual({ status: 'error', retry })
    },
  )

  it('fails closed for a cached board on an unnormalized contract error', () => {
    expect(resolve(
      { kind: 'project', projectId: 9, tab: 'overview' },
      'error',
      board,
      new Error('contract mismatch'),
    )).toEqual({ status: 'error', retry })
  })

  it('narrows every project tab into the supported plan-unit tab contract', () => {
    expect([
      resolvePlanUnitTab('overview'),
      resolvePlanUnitTab('items'),
      resolvePlanUnitTab('delivery'),
      resolvePlanUnitTab('activity'),
    ]).toEqual(['overview', 'overview', 'delivery', 'activity'])
  })
})

it('keeps the referenced owner name after removal from the selectable directory', () => {
 const state = resolveProjectListState({ status: 'success', projects: [{ ...project, owner_user_id: 110, owner_name: 'PM Administrator' }], total: 1, fetching: false, search: '', selectedProjectId: undefined, users: [], retry: vi.fn() })
 expect(state.status).toBe('ready')
 if (state.status !== 'ready') throw new Error('expected ready')
 expect(state.items[0].owner).toMatchObject({ userId: 110, name: 'PM Administrator' })
})
