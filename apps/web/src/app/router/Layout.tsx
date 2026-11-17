import { UserMentionPreview } from '@/features/user-mentions'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Outlet,
  useNavigate,
  useRouterState,
} from '@tanstack/react-router'

import { Standalone } from '@/app/shell'
import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { useLogoutWorkflow } from '@/features/auth/api/runtime'
import { pageCodeForPath, useActionPermissions } from '@/features/access-control'
import {
  useWorkspacesQuery,
  useWorkspaceSwitcher,
} from '@/features/workspace/api/runtime'
import { getPMRuntimeConfig } from '@/shared/config'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { EmptyState, LoadingSkeleton } from '@/shared/ui'
import { RouteBoundary } from './Boundary'
import { useRouteContentDim } from './PendingScreen'

function EmbeddedSessionBoundary() {
  const alertRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    alertRef.current?.focus()
  }, [])

  return (
    <main className="grid min-h-dvh place-items-center bg-background px-6">
      <div
        ref={alertRef}
        role="alert"
        aria-live="assertive"
        tabIndex={-1}
        className="w-full outline-none"
      >
        <EmptyState
          title={copy.auth.sessionUnavailableTitle}
        />
        <p className="m-0 max-w-md text-center text-sm text-muted-foreground">
          {copy.auth.sessionUnavailableDescription}
        </p>
      </div>
    </main>
  )
}

export function AppRouteLayout() {
  const runtime = useMemo(() => getPMRuntimeConfig(), [])
  const navigate = useNavigate()
  const currentPath = useRouterState({
    select: (state) => state.location.pathname,
  })
  const mode = useSessionStore((state) => state.mode)
  const token = useSessionStore((state) => state.token)
  const user = useSessionStore((state) => state.user)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const activePage = pageCodeForPath(currentPath)
  const actionPermissions = useActionPermissions(Boolean(
    token && activePage,
  ))
  const workspaces = useWorkspacesQuery()
  const logoutWorkflow = useLogoutWorkflow()
  const [failedWorkspaceId, setFailedWorkspaceId] = useState<number | null>(null)
  const contentDimmed = useRouteContentDim()

  const navigateToProjects = useCallback(
    () => navigate({ to: '/workspace/projects', replace: true }),
    [navigate],
  )
  const workspaceSwitcher = useWorkspaceSwitcher({
    navigate: navigateToProjects,
  })
  const requestWorkspaceChange = useCallback(
    async (nextWorkspaceId: number) => {
      setFailedWorkspaceId(null)
      const switched = await workspaceSwitcher.switchWorkspace(nextWorkspaceId)
      if (!switched) setFailedWorkspaceId(nextWorkspaceId)
    },
    [workspaceSwitcher.switchWorkspace],
  )
  const retryWorkspace = useCallback(() => {
    if (workspaces.isError) {
      setFailedWorkspaceId(null)
      void workspaces.refetch()
      return
    }
    if (failedWorkspaceId) void requestWorkspaceChange(failedWorkspaceId)
  }, [failedWorkspaceId, requestWorkspaceChange, workspaces])

  const navigateWithinShell = useCallback(
    (path: string) => {
      void navigate({ to: path } as never)
    },
    [navigate],
  )
  const logout = useCallback(() => {
    void logoutWorkflow.logout().then((completed) => {
      if (completed) {
        void navigate({
          to: '/login',
          search: {
            redirect: undefined,
            login_code: undefined,
            external_registration_code: undefined,
            login_provider: undefined,
          },
          replace: true,
        })
      }
    })
  }, [logoutWorkflow, navigate])
  const isNavigationPathVisible = useCallback((path: string) => {
    const page = pageCodeForPath(path)
    return page ? actionPermissions.canPage(page) : false
  }, [actionPermissions.canPage])

  const pageContent = !activePage ? <Outlet /> : actionPermissions.status === 'loading' ? (
    <LoadingSkeleton rows={6} label={copy.app.pageAccess.loading} />
  ) : actionPermissions.status === 'error' || !actionPermissions.canPage(activePage) ? (
    <EmptyState
      title={copy.app.pageAccess.deniedTitle}
    />
  ) : <Outlet />

  if (!currentPath.startsWith('/workspace/')) return <Outlet />
  if (!token) {
    return mode === 'embedded' ? (
      <EmbeddedSessionBoundary />
    ) : (
      <RouteBoundary id="pending" />
    )
  }
  if (mode === 'embedded') {
    return (
      <main
        id="pm-main-content"
        tabIndex={-1}
        aria-busy={contentDimmed || undefined}
        className={cn(
          'min-w-0 bg-background outline-none transition-opacity duration-200 ease-out animate-[pm-page-fade-in_250ms_ease-out]',
          contentDimmed && 'opacity-70',
          currentPath.startsWith('/workspace/projects')
          || currentPath.startsWith('/workspace/requirements')
          || currentPath.startsWith('/workspace/documents')
          || currentPath.startsWith('/workspace/bitables')
            ? 'flex h-dvh min-h-0 flex-col overflow-hidden'
            : 'min-h-dvh',
        )}
      >
        {pageContent}
        <UserMentionPreview key={workspaceId} workspaceId={workspaceId} />
      </main>
    )
  }

  const workspaceError = workspaces.isError
    ? copy.workspace.loadFailed
    : workspaceSwitcher.errorMessage ||
      (failedWorkspaceId ? copy.workspace.switchFailed : undefined)

  return (
    <Standalone
      productName={runtime.productName}
      shortName={runtime.shortName}
      userLabel={user?.display_name || user?.username}
      userUsername={user?.username}
      userAvatarUrl={user?.avatar_url}
      workspaceId={workspaceId}
      workspaceOptions={(workspaces.data ?? []).map((workspace) => ({
        id: workspace.id,
        label: workspace.name || workspace.slug || String(workspace.id),
        disabled: workspace.enabled === false,
      }))}
      workspaceLoading={workspaces.isPending}
      workspaceDisabled={workspaceSwitcher.pending || workspaces.isError}
      workspaceSwitching={workspaceSwitcher.pending}
      workspaceError={workspaceError}
      logoutPending={logoutWorkflow.pending}
      currentPath={currentPath}
      contentBusy={contentDimmed}
      onNavigate={navigateWithinShell}
      onMcpCli={isNavigationPathVisible('/workspace/mcp-cli')
        ? () => navigateWithinShell('/workspace/mcp-cli')
        : undefined}
      onProfile={() => navigateWithinShell('/workspace/profile')}
      onReleaseNotes={() => navigateWithinShell('/release-notes')}
      onWorkspaceCreate={() => navigate({ to: '/select-workspace' })}
      isNavigationPathVisible={isNavigationPathVisible}
      onWorkspaceRetry={workspaceError ? retryWorkspace : undefined}
      onWorkspaceChange={(nextWorkspaceId) => {
        void requestWorkspaceChange(nextWorkspaceId)
      }}
      onLogout={logout}
    >
      {pageContent}
        <UserMentionPreview key={workspaceId} workspaceId={workspaceId} />
    </Standalone>
  )
}
