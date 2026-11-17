import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'bun:test'

import {
  PM_ACTION,
  PM_OPERATION_ACTIONS_BY_DOMAIN,
  PM_PAGE_BY_ROUTE,
  PM_TOOL_NAMES_BY_ACTION,
} from '@/generated/access-catalog'
import { coreApi } from '@/shared/api'
import { permissionsStore } from '../model/permissions-store'
import {
  ActionPermission,
  pageCodeForPath,
  resolveOperationCapability,
  useActionPermissions,
  useCanAction,
  useOperationCapabilities,
} from '..'

const originalAdapter = coreApi.defaults.adapter

afterEach(() => {
  coreApi.defaults.adapter = originalAdapter
  permissionsStore.getState().reset()
})

function Probe() {
  const canDelete = useCanAction(PM_ACTION.AccessManagementRolesDelete)
  const permissions = useActionPermissions()
  return (
    <>
      <ActionPermission code={PM_ACTION.AccessManagementRolesCreate}>
        <button type="button">allowed action</button>
      </ActionPermission>
      <ActionPermission code={PM_ACTION.AccessManagementGroupsDelete}>
        <button type="button">denied action</button>
      </ActionPermission>
      <span>{canDelete ? 'delete allowed' : 'delete denied'}</span>
      <span>{permissions.canPage(PM_PAGE_BY_ROUTE['/workspace/pm/access-management']) ? 'page allowed' : 'page denied'}</span>
    </>
  )
}

function OperationProbe() {
  const operations = useOperationCapabilities('work-items')
  return (
    <>
      <span>{operations.allows(PM_ACTION.WorkItemsTasksUpdate) ? 'update allowed' : 'update denied'}</span>
      <span>{operations.allows(PM_ACTION.WorkItemsTasksUpdate, false) ? 'local allowed' : 'local denied'}</span>
      <span>{operations.allows(PM_ACTION.WorkItemsTasksTransitionStatus) ? 'transition allowed' : 'transition denied'}</span>
      <span>{operations.allows(PM_ACTION.WorkItemsTasksDelete) ? 'delete allowed' : 'delete denied'}</span>
    </>
  )
}

describe('access-control', () => {
  it('keeps all governed view and operation actions in the generated domain registry', () => {
    const actions = Object.values(PM_OPERATION_ACTIONS_BY_DOMAIN).flat()
    expect(actions).toHaveLength(185)
    expect(new Set(actions).size).toBe(185)
    expect(PM_OPERATION_ACTIONS_BY_DOMAIN.documents).toEqual([
      PM_ACTION.DocumentsScopeAll,
      PM_ACTION.DocumentsAccessView,
      PM_ACTION.DocumentsAccessRequest,
      PM_ACTION.DocumentsAccessManage,
      PM_ACTION.DocumentsSubjectsView,
      PM_ACTION.DocumentsShareLinksView,
      PM_ACTION.DocumentsShareLinksManage,
      PM_ACTION.DocumentsShareLinksExchange,
      PM_ACTION.DocumentsCollaborationView,
      PM_ACTION.DocumentsCollaborationEdit,
      PM_ACTION.DocumentsNodesView,
      PM_ACTION.DocumentsNodesCreate,
      PM_ACTION.DocumentsNodesCopy,
      PM_ACTION.DocumentsNodesUpdate,
      PM_ACTION.DocumentsNodesMove,
      PM_ACTION.DocumentsNodesArchive,
      PM_ACTION.DocumentsShortcutsCreate,
      PM_ACTION.DocumentsPreferencesUpdate,
      PM_ACTION.DocumentsOwnershipTransfer,
      PM_ACTION.DocumentsContentView,
      PM_ACTION.DocumentsContentUpdate,
      PM_ACTION.DocumentsContentExport,
      PM_ACTION.DocumentsBindingsUpdate,
      PM_ACTION.DocumentsAssetsView,
      PM_ACTION.DocumentsAssetsUpload,
      PM_ACTION.DocumentsAssetsDownload,
      PM_ACTION.DocumentsAssetsDelete,
      PM_ACTION.DocumentsCommentsView,
      PM_ACTION.DocumentsCommentsCreate,
      PM_ACTION.DocumentsCommentsReply,
      PM_ACTION.DocumentsCommentsEditOwn,
      PM_ACTION.DocumentsCommentsDeleteOwn,
      PM_ACTION.DocumentsCommentsResolve,
      PM_ACTION.DocumentsCommentsReopen,
    ])
    expect(PM_TOOL_NAMES_BY_ACTION[PM_ACTION.ProjectsProjectSetsCreate]).toEqual([
      'project.set.create',
    ])
  })

  it('loads one shared action set per scope and keeps missing/unknown actions closed', async () => {
    let permissionRequests = 0
    coreApi.defaults.adapter = async (config) => {
      if (String(config.url).includes('/access/me/action-permissions')) {
        permissionRequests += 1
      }
      return {
        data: { data: { action_codes: [
          PM_ACTION.AccessManagementRolesCreate,
          PM_ACTION.AccessManagementRolesDelete,
          'pm.unknown.action',
        ] } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    render(<Probe />)

    expect(screen.queryByRole('button', { name: 'allowed action' })).toBeNull()
    expect(await screen.findByRole('button', { name: 'allowed action' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'denied action' })).toBeNull()
    expect(screen.getByText('delete allowed')).toBeTruthy()
    expect(screen.getByText('page allowed')).toBeTruthy()
    expect(permissionRequests).toBe(1)
  })

  it('fails closed when the permission request fails', async () => {
    coreApi.defaults.adapter = async (config) => {
      throw { response: { status: 403, data: { code: 'FORBIDDEN' } }, config }
    }
    render(<Probe />)
    expect(await screen.findByText('delete denied')).toBeTruthy()
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('resolves generated static and parameterized route patterns without widening unmapped routes', () => {
    expect(pageCodeForPath('/workspace/projects')).toBe(PM_PAGE_BY_ROUTE['/workspace/projects'])
    expect(pageCodeForPath('/workspace/documents/17')).toBe(PM_PAGE_BY_ROUTE['/workspace/documents/$documentId'])
    expect(pageCodeForPath('/workspace/projects/7/items/9')).toBe(PM_PAGE_BY_ROUTE['/workspace/projects/$projectId/items/$projectItemId'])
    expect(pageCodeForPath('/workspace/pm/workspace-management')).toBe(PM_PAGE_BY_ROUTE['/workspace/pm/workspace-management'])
    expect(pageCodeForPath('/workspace/not-cataloged')).toBeUndefined()
  })

  it('combines global grants and domain-owned availability through one operation resolver', async () => {
    coreApi.defaults.adapter = async (config) => ({
      data: { data: { action_codes: [PM_ACTION.WorkItemsTasksUpdate] } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    render(<OperationProbe />)

    expect(await screen.findByText('update allowed')).toBeTruthy()
    expect(screen.getByText('transition allowed')).toBeTruthy()
    expect(screen.getByText('local denied')).toBeTruthy()
    expect(screen.getByText('delete denied')).toBeTruthy()
    expect(resolveOperationCapability(true, false)).toBe(false)
  })
})
