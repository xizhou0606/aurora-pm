import { describe, expect, it } from 'bun:test'

import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import {
  mapPlanUnitDraftToPayload,
  mapProjectDraftToPayload,
  PLAN_UNIT_RISK_LEVELS,
  PLAN_UNIT_TYPES,
  planUnitDraftFromCreate,
  planUnitDraftFromUnit,
  projectDraftFromCreate,
  projectDraftFromProject,
  PROJECT_STATUSES,
  rebasePlanUnitUpdateDraft,
  rebaseProjectUpdateDraft,
  validatePlanUnitDraft,
  validateProjectDraft,
  type PlanUnitCreateDraft,
  type ProjectCreateDraft,
} from './write-forms'
import {
  planUnitActionAvailability,
  projectActionAvailability,
} from './action-availability'
import {
  canCloseDeleteConfirm,
  canClosePlanUnitEditor,
  canCloseProjectEditor,
  canSubmitDeleteConfirm,
  canSubmitPlanUnitEditor,
  canSubmitProjectEditor,
  reduceDeleteConfirm,
  reducePlanUnitEditor,
  reduceProjectEditor,
  type DeleteConfirmState,
  type PlanUnitEditorState,
  type ProjectEditorState,
} from './write-state'
import { ownerDirectoryItems } from './owner-directory'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'Project Nine',
  description: ' Existing ',
  owner_user_id: 11,
  starts_at: '2026-08-01',
  due_at: '2026-08-31',
  status: 'active',
}

const unit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'Release Search',
  type: 'version',
  owner_user_id: 11,
  starts_at: '2026-08-03',
  due_at: '2026-08-30',
  actual_start_at: '2026-08-04',
  actual_completed_at: null,
  risk_level: 'medium',
  goal: ' Existing goal ',
  status: 'development',
  requirement_count: 0,
}

function validProjectCreate(): ProjectCreateDraft {
  return {
    ...projectDraftFromCreate({ teamId: 2, ownerUserId: 11 }),
    name: '  New project  ',
    description: '  Project detail  ',
    plannedStartAt: '2026-09-01',
    plannedEndAt: '2026-09-30',
  }
}

function validPlanUnitCreate(): PlanUnitCreateDraft {
  return {
    ...planUnitDraftFromCreate(project),
    workflowTemplateId: 8,
    title: '  Release 2  ',
    type: 'version',
    plannedStartAt: '2026-09-01',
    plannedEndAt: '2026-09-30',
    actualStartAt: '',
    actualCompletedAt: '',
    riskLevel: 'high',
    goal: '  Ship safely  ',
  }
}

describe('P6 write action availability', () => {
  it('normalizes owner labels through display name, username, and id fallback', () => {
    expect(ownerDirectoryItems([
      { id: 1, display_name: ' Display ', username: 'one' },
      { id: 2, display_name: '', username: ' user-two ' },
      { id: 3, display_name: '', username: '' },
      { id: 3, display_name: 'duplicate', username: 'duplicate' },
      { id: 0, display_name: 'invalid', username: 'invalid' },
    ])).toEqual([
      { id: 1, label: 'Display', username: 'one' },
      { id: 2, label: 'user-two', username: 'user-two' },
      { id: 3, label: '#3' },
    ])
  })
  it('enables project actions for existing resources without authorization metadata', () => {
    expect(projectActionAvailability(project)).toEqual({
      create: true,
      update: true,
      delete: true,
      archive: true,
      createPlanUnit: true,
    })
    expect(planUnitActionAvailability(project, unit))
      .toEqual({ create: true, update: true, delete: true })
  })

  it('depends only on resource context', () => {
    expect(projectActionAvailability({
      id: 9,
    })).toEqual({
      create: true,
      update: true,
      delete: true,
      archive: true,
      createPlanUnit: true,
    })
    expect(planUnitActionAvailability(
      { id: 9 },
      { project_new_id: 9, requirement_count: 0 },
    )).toEqual({ create: true, update: true, delete: true })
  })

  it('requires the parent relationship and exactly zero requirements for unit delete', () => {
    expect(planUnitActionAvailability(project, unit))
      .toEqual({ create: true, update: true, delete: true })
    expect(planUnitActionAvailability(project, {
      ...unit,
      project_new_id: 10,
    })).toEqual({ create: true, update: false, delete: false })
    for (const count of [undefined, 1, -1]) {
      expect(planUnitActionAvailability(project, {
        ...unit,
        requirement_count: count,
      }).delete).toBe(false)
    }
    expect(planUnitActionAvailability(project)).toEqual({
      create: true,
      update: false,
      delete: false,
    })
    expect(planUnitActionAvailability(undefined, unit)).toEqual({
      create: false,
      update: false,
      delete: false,
    })
  })
})

describe('P6 write drafts, validation and payload mapping', () => {
  it('creates one-time drafts from DTOs with owned delivery state', () => {
    expect(projectDraftFromProject({
      ...project,
      starts_at: '2026-08-01T00:00:00+08:00',
      due_at: '2026-08-31T23:59:00+08:00',
    })).toEqual({
      mode: 'update',
      projectId: 9,
      name: 'Project Nine',
      description: ' Existing ',
      plannedStartAt: '2026-08-01',
      plannedEndAt: '2026-08-31',
      status: 'active',
    })
    expect(planUnitDraftFromUnit({
      ...unit,
      starts_at: '2026-08-03T09:30:00+08:00',
      due_at: '2026-08-30T18:00:00+08:00',
      actual_start_at: '2026-08-04T10:00:00+08:00',
      delivery_nodes: [{ type: 'acceptance', due_at: '2026-09-01' }],
    })).toEqual({
      mode: 'update',
      projectId: 9,
      planUnitId: 31,
      title: 'Release Search',
      type: 'version',
      ownerUserId: 11,
      plannedStartAt: '2026-08-03',
      plannedEndAt: '2026-08-30',
      actualStartAt: '2026-08-04',
      actualCompletedAt: '',
      riskLevel: 'medium',
      goal: ' Existing goal ',
      businessStatus: 'development',
      deliveryNodeLabels: [],
      deliveryNodes: [],
    })
  })

  it('maps project create/update with their asymmetric fields and null dates', () => {
    expect(mapProjectDraftToPayload(validProjectCreate())).toEqual({
      ok: true,
      value: {
        team_id: 2,
        name: 'New project',
        description: 'Project detail',
        owner_user_id: 11,
        planned_start_at: '2026-09-01',
        planned_end_at: '2026-09-30',
        status: 'active',
      },
    })
    const update = mapProjectDraftToPayload({
      ...projectDraftFromProject(project),
      name: '  Renamed  ',
      description: '   ',
      plannedStartAt: '',
      plannedEndAt: '',
      status: 'archived',
    }, project)
    expect(update).toEqual({
      ok: true,
      value: {
        name: 'Renamed',
        description: '',
        planned_start_at: null,
        expected_planned_start_at: '2026-08-01',
        planned_end_at: null,
        expected_planned_end_at: '2026-08-31',
        status: 'archived',
      },
    })
    if (!update.ok) throw new Error('expected update payload')
    expect(update.value).not.toHaveProperty('team_id')
    expect(update.value).not.toHaveProperty('owner_user_id')
  })

  it('rebases untouched project fields to fresh null dates without losing local edits', () => {
    const draft = {
      ...projectDraftFromProject(project),
      name: 'Local project name',
    }
    const latest = {
      ...project,
      description: 'External description',
      starts_at: null,
      due_at: '2026-09-15T12:00:00Z',
    }

    const rebased = rebaseProjectUpdateDraft(draft, project, latest)
    expect(rebased).toMatchObject({
      name: 'Local project name',
      description: 'External description',
      plannedStartAt: '',
      plannedEndAt: '2026-09-15',
    })
    const payload = mapProjectDraftToPayload(rebased, latest)
    expect(payload).toMatchObject({ ok: true, value: { name: 'Local project name', description: 'External description' } })
    if (!payload.ok) throw new Error('expected rebased project payload')
    expect(payload.value).not.toHaveProperty('planned_start_at')
    expect(payload.value).not.toHaveProperty('planned_end_at')
  })

  it('keeps the selected backend project status in the create payload', () => {
    expect(mapProjectDraftToPayload({
      ...validProjectCreate(),
      status: 'archived',
    })).toMatchObject({
      ok: true,
      value: { status: 'archived' },
    })
  })

  it('reports stable project field codes for required, IDs, dates and enum failures', () => {
    expect(validateProjectDraft({
      ...validProjectCreate(),
      teamId: 0,
      ownerUserId: -2,
      name: '   ',
      plannedStartAt: 'not-a-date',
    })).toEqual({
      teamId: 'INVALID_ID',
      name: 'REQUIRED',
      ownerUserId: 'INVALID_ID',
      plannedStartAt: 'INVALID_DATE',
    })
    expect(validateProjectDraft({
      ...validProjectCreate(),
      plannedStartAt: '2026-02-30',
    })).toMatchObject({ plannedStartAt: 'INVALID_DATE' })
    expect(validateProjectDraft({
      ...validProjectCreate(),
      status: 'planning',
    })).toMatchObject({ status: 'INVALID_ENUM' })
    expect(validateProjectDraft({
      ...projectDraftFromProject(project),
      projectId: 0,
      status: 'deleted',
      plannedStartAt: '2026-10-02',
      plannedEndAt: '2026-10-01',
    })).toEqual({
      projectId: 'INVALID_ID',
      status: 'INVALID_ENUM',
      plannedEndAt: 'DATE_ORDER',
    })
  })

  it('maps plan-unit create without status or legacy delivery nodes and maps update without the parent id', () => {
    const create = mapPlanUnitDraftToPayload(validPlanUnitCreate())
    expect(create).toEqual({
      ok: true,
      value: {
        project_new_id: 9,
        workflow_template_id: 8,
        title: 'Release 2',
        type: 'version',
        owner_user_id: 11,
        planned_start_at: '2026-09-01',
        planned_end_at: '2026-09-30',
        actual_start_at: null,
        actual_completed_at: null,
        risk_level: 'high',
        goal: 'Ship safely',
      },
    })
    if (!create.ok) throw new Error('expected create payload')
    expect(create.value).not.toHaveProperty('status')
    expect(create.value).not.toHaveProperty('delivery_node_labels')
    expect(create.value).not.toHaveProperty('delivery_nodes')

    const update = mapPlanUnitDraftToPayload({
      ...planUnitDraftFromUnit(unit),
      title: '  Updated release  ',
      actualCompletedAt: '',
      businessStatus: 'released',
    }, unit)
    expect(update.ok).toBe(true)
    if (!update.ok) throw new Error('expected update payload')
    expect(update.value).toMatchObject({
      title: 'Updated release',
    })
    expect(update.value).not.toHaveProperty('actual_completed_at')
    expect(update.value).not.toHaveProperty('planned_start_at')
    expect(update.value).not.toHaveProperty('planned_end_at')
    expect(update.value).not.toHaveProperty('actual_start_at')
    expect(update.value).not.toHaveProperty('design_freeze_due_at')
    expect(update.value).not.toHaveProperty('development_complete_due_at')
    expect(update.value).not.toHaveProperty('test_acceptance_due_at')
    expect(update.value).not.toHaveProperty('status')
    expect(update.value).not.toHaveProperty('project_new_id')
    expect(update.value).not.toHaveProperty('delivery_nodes')
  })

  it('sends only changed plan-unit dates with the exact frozen expected values', () => {
    const source = {
      ...unit,
      starts_at: '2026-08-03T09:30:00+08:00',
      due_at: '2026-08-30T18:00:00+08:00',
      actual_start_at: '2026-08-04T10:00:00+08:00',
      actual_completed_at: null,
      design_freeze_due_at: '2026-08-10T18:00:00+08:00',
      development_complete_due_at: '2026-08-20T18:00:00+08:00',
      test_acceptance_due_at: '2026-08-28T18:00:00+08:00',
    }
    const result = mapPlanUnitDraftToPayload({
      ...planUnitDraftFromUnit(source),
      plannedEndAt: '2026-09-01',
      actualStartAt: '',
    }, source)
    expect(result).toMatchObject({
      ok: true,
      value: {
        planned_end_at: '2026-09-01',
        expected_planned_end_at: '2026-08-30T18:00:00+08:00',
        actual_start_at: null,
        expected_actual_start_at: '2026-08-04T10:00:00+08:00',
      },
    })
    if (!result.ok) throw new Error('expected update payload')
    expect(result.value).not.toHaveProperty('planned_start_at')
    expect(result.value).not.toHaveProperty('design_freeze_due_at')
    expect(result.value).not.toHaveProperty('development_complete_due_at')
    expect(result.value).not.toHaveProperty('test_acceptance_due_at')
  })

  it('rebases untouched plan-unit dates to fresh null values and preserves a local date edit', () => {
    const draft = {
      ...planUnitDraftFromUnit(unit),
      title: 'Local release title',
      plannedEndAt: '2026-09-05',
    }
    const latest = {
      ...unit,
      title: 'External release title',
      starts_at: null,
      due_at: '2026-09-01T12:00:00Z',
      actual_start_at: null,
      actual_completed_at: '2026-09-02T12:00:00Z',
    }

    const rebased = rebasePlanUnitUpdateDraft(draft, unit, latest)
    expect(rebased).toMatchObject({
      title: 'Local release title',
      plannedStartAt: '',
      plannedEndAt: '2026-09-05',
      actualStartAt: '',
      actualCompletedAt: '2026-09-02',
    })
    const payload = mapPlanUnitDraftToPayload(rebased, latest)
    expect(payload).toMatchObject({
      ok: true,
      value: {
        title: 'Local release title',
        planned_end_at: '2026-09-05',
        expected_planned_end_at: '2026-09-01T12:00:00Z',
      },
    })
    if (!payload.ok) throw new Error('expected rebased plan-unit payload')
    expect(payload.value).not.toHaveProperty('planned_start_at')
    expect(payload.value).not.toHaveProperty('actual_start_at')
    expect(payload.value).not.toHaveProperty('actual_completed_at')
  })

  it('ignores legacy delivery nodes on create and uses persisted label ids on update', () => {
    const createDraft = validPlanUnitCreate()
    createDraft.deliveryNodes = [{
      label_client_key: 'acceptance',
      due_at: '2026-09-25T18:00:00Z',
      actual_completed_at: null,
      owner_user_id: 11,
    }]
    const create = mapPlanUnitDraftToPayload(createDraft)
    expect(create.ok).toBe(true)
    if (!create.ok) throw new Error('expected create payload')
    expect(create.value).not.toHaveProperty('delivery_nodes')

    const source = {
      ...unit,
      delivery_nodes: [{
        label_id: 81,
        label_name: '灰度发布',
        due_at: '2026-09-25T18:00:00Z',
        actual_completed_at: null,
        owner_user_id: 12,
      }],
    }
    const draft = planUnitDraftFromUnit(source)
    draft.deliveryNodes = draft.deliveryNodes.map((node) => ({
      ...node,
      due_at: '2026-09-26T18:00:00Z',
    }))
    const update = mapPlanUnitDraftToPayload(draft, source)
    expect(update.ok && update.value.delivery_nodes).toEqual([{
      label_id: 81,
      due_at: '2026-09-26T18:00:00.000Z',
      actual_completed_at: null,
      owner_user_id: 12,
    }])
  })

  it('validates every plan-unit ID, enum and date range', () => {
    expect(validatePlanUnitDraft({
      ...planUnitDraftFromUnit(unit),
      projectId: 0,
      planUnitId: -1,
      title: '',
      type: 'epic',
      ownerUserId: -3,
      plannedStartAt: '2026-09-03',
      plannedEndAt: '2026-09-02',
      actualStartAt: 'bad-date',
      riskLevel: 'urgent',
      businessStatus: 'historical-stage',
    })).toEqual({
      projectId: 'INVALID_ID',
      planUnitId: 'INVALID_ID',
      title: 'REQUIRED',
      type: 'INVALID_ENUM',
      ownerUserId: 'INVALID_ID',
      riskLevel: 'INVALID_ENUM',
      plannedEndAt: 'DATE_ORDER',
      actualStartAt: 'INVALID_DATE',
    })
  })

  it.each([...PROJECT_STATUSES])('accepts project status %s', (status) => {
    expect(validateProjectDraft({ ...projectDraftFromProject(project), status: status as never }))
      .not.toHaveProperty('status')
  })

  it.each([...PLAN_UNIT_TYPES])('accepts plan-unit type %s', (type) => {
    expect(validatePlanUnitDraft({ ...validPlanUnitCreate(), type: type as never }))
      .not.toHaveProperty('type')
  })

  it.each([...PLAN_UNIT_RISK_LEVELS])('accepts plan-unit risk %s', (riskLevel) => {
    expect(validatePlanUnitDraft({ ...validPlanUnitCreate(), riskLevel: riskLevel as never }))
      .not.toHaveProperty('riskLevel')
  })

  it('preserves a historical plan-unit status for display without validating or submitting it', () => {
    const draft = { ...planUnitDraftFromUnit(unit), businessStatus: 'historical-stage' }
    expect(validatePlanUnitDraft(draft)).not.toHaveProperty('status')
    const result = mapPlanUnitDraftToPayload(draft, unit)
    expect(result.ok && result.value).not.toHaveProperty('status')
  })
})

describe('P6 editor and delete-confirm state machines', () => {
  it('keeps project validation, submit, error and close states mutually exclusive', () => {
    let state: ProjectEditorState = { status: 'closed' }
    state = reduceProjectEditor(state, { type: 'prepare-create' })
    expect(state).toEqual({ status: 'preparing', mode: 'create' })
    state = reduceProjectEditor(state, { type: 'open', draft: projectDraftFromCreate() })
    expect(canSubmitProjectEditor(state)).toBe(false)
    expect(state.status === 'editing' && state.validationVisible).toBe(false)
    state = reduceProjectEditor(state, { type: 'submit' })
    expect(state.status === 'editing' && state.submit.status).toBe('idle')
    expect(state.status === 'editing' && state.validationVisible).toBe(true)

    const draft = validProjectCreate()
    state = reduceProjectEditor(state, { type: 'change', draft })
    expect(canSubmitProjectEditor(state)).toBe(true)
    expect(state.status === 'editing' && state.validationVisible).toBe(true)
    state = reduceProjectEditor(state, { type: 'submit' })
    const pending = state
    expect(canCloseProjectEditor(state)).toBe(false)
    expect(reduceProjectEditor(state, { type: 'close' })).toBe(pending)
    expect(reduceProjectEditor(state, { type: 'submit' })).toBe(pending)

    state = reduceProjectEditor(state, {
      type: 'submit-failed',
      errorCode: 'PROJECT_CREATE_FAILED',
    })
    expect(state).toMatchObject({
      status: 'editing',
      draft,
      submit: { status: 'error', errorCode: 'PROJECT_CREATE_FAILED' },
    })
    state = reduceProjectEditor(state, { type: 'submit' })
    state = reduceProjectEditor(state, { type: 'submit-succeeded' })
    expect(state).toEqual({ status: 'closed' })
  })

  it('rejects identity/mode swaps and stale project results', () => {
    const open = reduceProjectEditor(
      { status: 'closed' },
      { type: 'open', draft: projectDraftFromProject(project) },
    )
    expect(reduceProjectEditor(open, {
      type: 'change',
      draft: { ...projectDraftFromProject(project), projectId: 10 },
    })).toBe(open)
    expect(reduceProjectEditor(open, {
      type: 'change',
      draft: validProjectCreate(),
    })).toBe(open)
    expect(reduceProjectEditor(open, {
      type: 'submit-failed',
      errorCode: 'STALE',
    })).toBe(open)
  })

  it('preserves plan-unit draft on failure and locks pending transitions', () => {
    const draft = validPlanUnitCreate()
    let state: PlanUnitEditorState = reducePlanUnitEditor(
      { status: 'closed' },
      { type: 'open', draft },
    )
    expect(canSubmitPlanUnitEditor(state)).toBe(true)
    state = reducePlanUnitEditor(state, { type: 'submit' })
    const pending = state
    expect(canClosePlanUnitEditor(state)).toBe(false)
    expect(reducePlanUnitEditor(state, { type: 'close' })).toBe(pending)
    expect(reducePlanUnitEditor(state, {
      type: 'open',
      draft: { ...draft, projectId: 10 },
    })).toBe(pending)
    state = reducePlanUnitEditor(state, {
      type: 'submit-failed',
      errorCode: 'PLAN_UNIT_CREATE_FAILED',
    })
    expect(state).toMatchObject({ status: 'editing', draft })
    expect(state.status === 'editing' && state.submit).toEqual({
      status: 'error',
      errorCode: 'PLAN_UNIT_CREATE_FAILED',
    })
  })

  it('keeps existing nodes when the edit label catalog fails and unlocks only after retry', () => {
    const draft = planUnitDraftFromUnit({
      ...unit,
      delivery_nodes: [{ label_id: 81, label_name: '灰度发布', due_at: '2026-09-25' }],
    })
    let state: PlanUnitEditorState = reducePlanUnitEditor(
      { status: 'closed' },
      { type: 'open', draft },
    )
    expect(state.status === 'editing' && state.deliveryCatalog.status).toBe('loading')
    expect(canSubmitPlanUnitEditor(state)).toBe(false)
    state = reducePlanUnitEditor(state, {
      type: 'delivery-failed',
      errorCode: 'DELIVERY_LABELS_FAILED',
    })
    expect(state.status === 'editing' && state.draft.deliveryNodes).toEqual(draft.deliveryNodes)
    expect(reducePlanUnitEditor(state, { type: 'submit' })).toBe(state)
    state = reducePlanUnitEditor(state, { type: 'delivery-loading' })
    state = reducePlanUnitEditor(state, {
      type: 'delivery-loaded',
      labels: [{ id: 81, name: '灰度发布', sort_order: 10 }],
    })
    expect(state).toMatchObject({
      status: 'editing',
      deliveryCatalog: { status: 'ready' },
      draft: {
        deliveryNodeLabels: [{ id: 81, name: '灰度发布', sort_order: 10, system_key: null, deleted: false }],
        deliveryNodes: [{ label_id: 81, label_name: '灰度发布' }],
      },
    })
    expect(canSubmitPlanUnitEditor(state)).toBe(true)
  })

  it('keeps delete target through failure and closes only after pending success', () => {
    const target = { kind: 'plan-unit' as const, projectId: 9, planUnitId: 31, title: 'Release', itemType: 'version' }
    let state: DeleteConfirmState = reduceDeleteConfirm(
      { status: 'closed' },
      { type: 'open', target },
    )
    expect(canSubmitDeleteConfirm(state)).toBe(true)
    state = reduceDeleteConfirm(state, { type: 'submit' })
    const pending = state
    expect(canCloseDeleteConfirm(state)).toBe(false)
    expect(reduceDeleteConfirm(state, { type: 'close' })).toBe(pending)
    state = reduceDeleteConfirm(state, {
      type: 'submit-failed',
      errorCode: 'PLAN_UNIT_DELETE_FAILED',
    })
    expect(state).toEqual({
      status: 'confirming',
      target,
      submit: { status: 'error', errorCode: 'PLAN_UNIT_DELETE_FAILED' },
    })
    state = reduceDeleteConfirm(state, { type: 'submit' })
    state = reduceDeleteConfirm(state, { type: 'submit-succeeded' })
    expect(state).toEqual({ status: 'closed' })
  })
})
