import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const clientMocks = (() => ({
  delete: vi.fn(),
  get: vi.fn(),
  getRuntimeToken: vi.fn(),
  getRuntimeWorkspaceId: vi.fn(),
  patch: vi.fn(),
  post: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
  platformApi: {
    delete: clientMocks.delete,
    get: clientMocks.get,
    patch: clientMocks.patch,
    post: clientMocks.post,
  },
}))

vi.mock('@/shared/runtime', () => ({
  getRuntimeToken: clientMocks.getRuntimeToken,
  getRuntimeWorkspaceId: clientMocks.getRuntimeWorkspaceId,
}))

import {
  createProjectWorkTask,
  createScrumProductRequirementNew,
  deleteProjectWorkTask,
  deleteScrumProductRequirementNew,
  getProjectWorkTask,
  listScrumProductRequirementsNew,
  listScrumTasksNew,
  patchProjectWorkTask,
  patchScrumProductRequirementNew,
} from './index'

const requirementCreate = {
  version_task_new_id: 31,
  workflow_template_id: 8,
  title: 'Ship React requirements',
  product_doc_url: 'https://example.test/requirements/41',
  product_owner_user_id: 11,
  design_url: 'https://example.test/design/41',
  design_owner_user_id: 12,
  developer_user_ids: [13, 14],
  description: 'Preserve the PM behavior baseline.',
  acceptance_criteria: 'All P7 gates pass.',
  priority: 'p1' as const,
}

const requirementPatch = {
  expected_version: 7,
  title: 'Ship state-driven React requirements',
  priority: 'p0' as const,
}

const taskCreate = {
  requirement_id: 41,
  kind: 'task' as const,
  title: 'Implement requirement editor',
  description: 'Keep the workflow state discriminated.',
  priority: 'p1' as const,
  assignee_id: 13,
  estimated_minutes: 480,
  starts_at: '2026-08-03',
  due_at: '2026-08-04',
}

const taskPatch = {
  title: 'Implement and verify requirement editor',
  status: 'in_dev' as const,
  priority: 'p0' as const,
  assignee_id: 14,
}

describe('P7 requirement and project-task adapters', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('forwards normalized requirement/task list params and cancellation signals', async () => {
    const signal = new AbortController().signal
    clientMocks.get
      .mockResolvedValueOnce({
        data: {
          data: [
            { id: 41, title: 'Requirement 41' },
            { id: 0, title: 'Invalid requirement' },
          ],
          total: 2,
          users: [{ id: 13, username: 'frontend', display_name: '前端同学', avatar_url: '/frontend.png' }],
        },
      })
      .mockResolvedValueOnce({
        data: {
          data: [
            { id: 51, kind: 'task', title: 'Task 51' },
            { id: -1, kind: 'task', title: 'Invalid task' },
          ],
          total: 2,
          users: [{ id: 14, username: 'backend', display_name: '后端同学', avatar_url: '/backend.png' }],
        },
      })

    await expect(listScrumProductRequirementsNew({
      projectNewId: 9,
      versionTaskNewId: 31,
      q: '  compiler  ',
      status: 'development',
      priority: 'p1',
      limit: 20,
      offset: 40,
    }, signal)).resolves.toMatchObject({
      rows: [{ id: 41 }],
      total: 2,
      users: [{ id: 13, display_name: '前端同学', avatar_url: '/frontend.png' }],
    })
    await expect(listScrumTasksNew({
      projectNewId: 9,
      versionTaskNewId: 31,
      requirementNewId: 41,
      limit: 50,
      offset: 100,
    }, signal)).resolves.toMatchObject({
      rows: [{ id: 51 }],
      total: 2,
      users: [{ id: 14, display_name: '后端同学', avatar_url: '/backend.png' }],
    })

    expect(clientMocks.get).toHaveBeenNthCalledWith(1, '/product-requirements-new', {
      params: {
        limit: '20',
        offset: '40',
        priority: 'p1',
        project_new_id: '9',
        q: 'compiler',
        status: 'development',
        version_task_new_id: '31',
      },
      signal,
    })
    expect(clientMocks.get).toHaveBeenNthCalledWith(2, '/scrum-tasks-new', {
      params: {
        limit: '50',
        offset: '100',
        project_new_id: '9',
        requirement_new_id: '41',
        version_task_new_id: '31',
      },
      signal,
    })
  })

  it('reads an exact PM task without scanning list pages', async () => {
    const signal = new AbortController().signal
    clientMocks.get.mockResolvedValueOnce({
      data: { data: { id: 51, title: 'Task 51', can_update: false } },
    })

    await expect(getProjectWorkTask(51, signal)).resolves.toMatchObject({
      id: 51,
      can_update: false,
    })
    expect(clientMocks.get).toHaveBeenCalledWith('/tasks/51', { signal })
  })

  it('uses the canonical mutation endpoints and preserves the named payloads', async () => {
    clientMocks.post
      .mockResolvedValueOnce({ data: { data: { id: 41, title: requirementCreate.title } } })
      .mockResolvedValueOnce({ data: { data: { id: 51, kind: 'task', title: taskCreate.title } } })
    clientMocks.patch
      .mockResolvedValueOnce({ data: { data: {
        id: 41,
        version: 8,
        title: requirementPatch.title,
        status: 'development',
        priority: requirementPatch.priority,
      } } })
      .mockResolvedValueOnce({ data: { data: { id: 51, kind: 'task', ...taskPatch } } })
    clientMocks.delete
      .mockResolvedValueOnce({ data: { data: { deleted: true } } })
      .mockResolvedValueOnce({ data: { data: { deleted: true } } })

    await createScrumProductRequirementNew(requirementCreate)
    await patchScrumProductRequirementNew(41, requirementPatch)
    await deleteScrumProductRequirementNew(41)
    await createProjectWorkTask(taskCreate)
    await patchProjectWorkTask(51, taskPatch)
    await deleteProjectWorkTask(51)

    expect(clientMocks.post).toHaveBeenNthCalledWith(
      1,
      '/product-requirements-new',
      requirementCreate,
    )
    expect(clientMocks.patch).toHaveBeenNthCalledWith(
      1,
      '/product-requirements-new/41',
      requirementPatch,
    )
    expect(clientMocks.post.mock.calls[0]?.[1]).not.toHaveProperty('status')
    expect(clientMocks.patch.mock.calls[0]?.[1]).not.toHaveProperty('status')
    expect(clientMocks.delete).toHaveBeenNthCalledWith(
      1,
      '/product-requirements-new/41',
    )
    expect(clientMocks.post).toHaveBeenNthCalledWith(2, '/tasks', taskCreate)
    expect(clientMocks.post.mock.calls[1]?.[1]).toMatchObject({ kind: 'task' })
    expect(clientMocks.patch).toHaveBeenNthCalledWith(2, '/tasks/51', taskPatch)
    expect(clientMocks.delete).toHaveBeenNthCalledWith(2, '/tasks/51')
  })

  it('requires positive integer IDs in create and non-empty mutation DTOs', async () => {
    clientMocks.post
      .mockResolvedValueOnce({ data: { data: { id: 0 } } })
      .mockResolvedValueOnce({ data: { data: { id: -1, kind: 'task' } } })
    clientMocks.patch
      .mockResolvedValueOnce({ data: { data: { id: 1.5 } } })
      .mockResolvedValueOnce({ data: { data: { id: Number.NaN, kind: 'task' } } })

    await expect(createScrumProductRequirementNew(requirementCreate)).rejects.toMatchObject({
      code: 'REQUIREMENT_CREATE_FAILED',
    })
    await expect(createProjectWorkTask(taskCreate)).rejects.toMatchObject({
      code: 'TASK_CREATE_FAILED',
    })
    await expect(patchScrumProductRequirementNew(41, requirementPatch)).rejects.toMatchObject({
      code: 'REQUIREMENT_UPDATE_FAILED',
    })
    await expect(patchProjectWorkTask(51, taskPatch)).rejects.toMatchObject({
      code: 'TASK_UPDATE_FAILED',
    })
  })

  it('accepts legacy empty 2xx patch/delete bodies but rejects contradictory DTOs', async () => {
    clientMocks.patch
      .mockResolvedValueOnce({ data: { data: {} } })
      .mockResolvedValueOnce({ data: { data: '   ' } })
      .mockResolvedValueOnce({ data: { data: { id: 0 } } })
      .mockResolvedValueOnce({ data: { data: { id: -1 } } })
    clientMocks.delete
      .mockResolvedValueOnce({ data: { data: undefined } })
      .mockResolvedValueOnce({ data: { data: '' } })
      .mockResolvedValueOnce({ data: { data: { deleted: false } } })
      .mockResolvedValueOnce({ data: { data: { deleted: 'yes' } } })

    const result = await patchScrumProductRequirementNew(41, requirementPatch)
    expect(result).toEqual({
      id: 41,
      version: 8,
      title: requirementPatch.title,
      priority: requirementPatch.priority,
    } as never)
    expect(result).not.toHaveProperty('expected_version')
    await expect(patchProjectWorkTask(51, taskPatch)).resolves.toEqual({
      id: 51,
      ...taskPatch,
    } as never)
    await expect(deleteScrumProductRequirementNew(41)).resolves.toEqual({ deleted: true })
    await expect(deleteProjectWorkTask(51)).resolves.toEqual({ deleted: true })

    await expect(patchScrumProductRequirementNew(41, requirementPatch)).rejects.toMatchObject({
      code: 'REQUIREMENT_UPDATE_FAILED',
    })
    await expect(patchProjectWorkTask(51, taskPatch)).rejects.toMatchObject({
      code: 'TASK_UPDATE_FAILED',
    })
    await expect(deleteScrumProductRequirementNew(41)).rejects.toMatchObject({
      code: 'REQUIREMENT_DELETE_FAILED',
    })
    await expect(deleteProjectWorkTask(51)).rejects.toMatchObject({
      code: 'TASK_DELETE_FAILED',
    })
  })

  it('maps every transport failure to its safe operation code without leaking Axios details', async () => {
    const rawError = () => ({
      config: { headers: { Authorization: 'Bearer p7-secret' } },
      request: { body: 'private request details' },
      response: { status: 502, data: { error: 'private upstream details' } },
    })
    const cases = [
      [clientMocks.get, () => listScrumProductRequirementsNew({ projectNewId: 9 }), 'REQUIREMENTS_FAILED'],
      [clientMocks.get, () => listScrumTasksNew({ projectNewId: 9, versionTaskNewId: 31 }), 'TASKS_FAILED'],
      [clientMocks.post, () => createScrumProductRequirementNew(requirementCreate), 'REQUIREMENT_CREATE_FAILED'],
      [clientMocks.patch, () => patchScrumProductRequirementNew(41, requirementPatch), 'REQUIREMENT_UPDATE_FAILED'],
      [clientMocks.delete, () => deleteScrumProductRequirementNew(41), 'REQUIREMENT_DELETE_FAILED'],
      [clientMocks.post, () => createProjectWorkTask(taskCreate), 'TASK_CREATE_FAILED'],
      [clientMocks.patch, () => patchProjectWorkTask(51, taskPatch), 'TASK_UPDATE_FAILED'],
      [clientMocks.delete, () => deleteProjectWorkTask(51), 'TASK_DELETE_FAILED'],
    ] as const

    for (const [request, action, code] of cases) {
      request.mockRejectedValueOnce(rawError())
      const error = await action().catch((reason: unknown) => reason)
      expect(error).toMatchObject({ name: 'PMSafeApiError', code, status: 502 })
      expect(error).not.toHaveProperty('config')
      expect(error).not.toHaveProperty('request')
      expect(JSON.stringify(error)).not.toContain('p7-secret')
      expect(JSON.stringify(error)).not.toContain('private request details')
    }
  })
})
