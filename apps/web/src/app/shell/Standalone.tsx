import { useNavigationCollapse } from './useNavigationCollapse'
import type { ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { nativeModuleIdForPath } from '@/app/native-modules'

import { Header, type HeaderProps } from './Header'
import { Navigation } from './Navigation'
import { normalizedShellPath } from './registry'

export type StandaloneProps = HeaderProps & {
  children: ReactNode
  currentPath: string
  onNavigate: (path: string) => void
  isNavigationPathVisible?: (path: string) => boolean
  onReleaseNotes?: () => void
  /** True while a path-changing navigation is in flight; eases the content area to 70% opacity. */
  contentBusy?: boolean
  className?: string
}

export function Standalone({
  children,
  className,
  currentPath,
  isNavigationPathVisible,
  onNavigate,
  contentBusy,
  productName,
  userLabel,
  userUsername,
  userAvatarUrl,
  workspaceId,
  workspaceOptions,
  workspaceLoading,
  workspaceSwitching,
  workspaceError,
  logoutPending,
  onWorkspaceRetry,
  onWorkspaceChange,
  onWorkspaceCreate,
  onProfile,
  onReleaseNotes,
  onLogout,
  mcpCliHref,
  onMcpCli,
}: StandaloneProps) {
  const navigation = useNavigationCollapse()
  const path = normalizedShellPath(currentPath)
  const fillsViewport = Boolean(nativeModuleIdForPath(path))
    || path.startsWith('/workspace/projects')
    || path.startsWith('/workspace/requirements')
    || path.startsWith('/workspace/documents')
    || path.startsWith('/workspace/bitables')

  return (
    <div
      data-slot="standalone-shell"
      className={cn(
        'grid h-dvh min-h-screen w-full overflow-hidden bg-background [grid-template-rows:44px_minmax(0,1fr)] max-[720px]:[grid-template-rows:auto_minmax(0,1fr)]',
        className,
      )}
    >
      <a
        href="#pm-main-content"
        className="sr-only z-50 rounded-md bg-card px-3 py-2 text-sm text-foreground shadow-lg focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:ring-2 focus:ring-ring"
      >
        {copy.app.shell.skipToContent}
      </a>
      <Header productName={productName} mcpCliHref={mcpCliHref} onMcpCli={onMcpCli} />
      <div
        data-slot="standalone-shell-body"
        className={cn(
          'grid min-h-0 min-w-0 max-[720px]:[grid-template-rows:53px_minmax(0,1fr)] max-[720px]:[grid-template-columns:minmax(0,1fr)]',
          navigation.collapsed ? 'min-[721px]:[grid-template-columns:64px_minmax(0,1fr)]' : 'min-[721px]:[grid-template-columns:240px_minmax(0,1fr)]',
        )}
      >
        <Navigation
          collapsed={navigation.collapsed}
          onToggle={navigation.toggle}
          currentPath={currentPath}
          onNavigate={onNavigate}
          isPathVisible={isNavigationPathVisible}
          account={{
            userLabel,
            username: userUsername,
            userAvatarUrl,
            workspaceId,
            workspaceOptions,
            workspaceLoading,
            workspaceSwitching,
            workspaceError,
            logoutPending,
            onWorkspaceRetry,
            onWorkspaceChange,
            onWorkspaceCreate,
            onProfile,
            onReleaseNotes,
            onLogout,
          }}
        />
        <main
          id="pm-main-content"
          tabIndex={-1}
          aria-busy={contentBusy || undefined}
          data-slot="standalone-main"
          className={cn(
            'min-h-0 min-w-0 outline-none transition-opacity duration-200 ease-out animate-[pm-page-fade-in_250ms_ease-out]',
            contentBusy && 'opacity-70',
            fillsViewport ? 'flex overflow-hidden' : 'overflow-auto',
          )}
        >
          {children}
        </main>
      </div>
    </div>
  )
}
