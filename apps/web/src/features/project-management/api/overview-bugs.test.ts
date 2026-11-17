import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const { platformApiGet } = (() => ({
  platformApiGet: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
  platformApi: { get: platformApiGet },
}))

import { PMSafeApiError } from '@/shared/api'
import { listProjectOverviewBugs } from './overview-bugs'

describe('listProjectOverviewBugs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads unclosed bugs for a project set and plan unit', async () => {
    platformApiGet.mockResolvedValue({
      data: {
        items: [
          { requirement_id: 41, severity: 'critical', count: 2 },
          { requirement_id: 0, severity: 'normal', count: 1 },
        ],
        total: 2,
      },
    })

    await expect(listProjectOverviewBugs({
      projectSetId: 9,
      planUnitId: 31,
    })).resolves.toEqual([{ requirementId: 41, severity: 'critical', count: 2 }])
    expect(platformApiGet).toHaveBeenCalledWith('/project-sets/9/quality-overview', {
      params: {
        project_ids: '31',
      },
      signal: undefined,
    })
  })

  it('fails soft when quality is forbidden so overview still renders', async () => {
    platformApiGet.mockRejectedValue(new PMSafeApiError('WORKSPACE_FORBIDDEN', 403))
    await expect(listProjectOverviewBugs({ projectSetId: 9 })).resolves.toEqual([])
  })
})
