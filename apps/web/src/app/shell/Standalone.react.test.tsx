import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { Standalone } from './Standalone'

vi.mock('@/features/notifications', () => ({ MentionNotifications: () => null }))

function renderShell(
  overrides: Partial<React.ComponentProps<typeof Standalone>> = {},
) {
  const onNavigate = vi.fn()
  const onWorkspaceChange = vi.fn()
  const onLogout = vi.fn()
  const onProfile = vi.fn()
  const onMcpCli = vi.fn()
  const onReleaseNotes = vi.fn()
  const result = render(
    <Standalone
      productName="PM Product"
      shortName="PM"
      userLabel="Teo"
      workspaceId={7}
      workspaceOptions={[
        { id: 7, label: 'Workspace Seven' },
        { id: 9, label: 'Workspace Nine' },
      ]}
      currentPath="/workspace/projects/12/items/31"
      onNavigate={onNavigate}
      onMcpCli={onMcpCli}
      onWorkspaceChange={onWorkspaceChange}
      onProfile={onProfile}
      onReleaseNotes={onReleaseNotes}
      onLogout={onLogout}
      {...overrides}
    >
      <div>Content</div>
    </Standalone>,
  )
  return {
    ...result,
    onLogout,
    onMcpCli,
    onNavigate,
    onProfile,
    onReleaseNotes,
    onWorkspaceChange,
  }
}

describe('Standalone shell', () => {
  it('collapses, preserves navigation and restores the saved preference', async () => {
    localStorage.removeItem('pm.navigation.collapsed')
    const first = renderShell()
    const collapse = screen.getByRole('button', { name: copy.app.shell.collapseNavigation })
    expect(collapse.getAttribute('aria-expanded')).toBe('true')
    fireEvent.click(collapse)
    expect(localStorage.getItem('pm.navigation.collapsed')).toBe('true')
    expect(first.container.querySelector('[data-slot="standalone-shell-body"]')?.className).toContain('64px')
    const projects = screen.getByRole('link', { name: copy.app.navigation.projects })
    expect(projects.title).toBe(copy.app.navigation.projects)
    fireEvent.click(projects)
    expect(first.onNavigate).toHaveBeenCalledWith('/workspace/projects')
    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    expect((await screen.findByRole('menu')).getAttribute('aria-label')).toBe(copy.app.shell.accountMenu)
    first.unmount()
    const second = renderShell()
    expect(screen.getByRole('button', { name: copy.app.shell.expandNavigation }).getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.expandNavigation }))
    expect(second.container.querySelector('[data-slot="standalone-shell-body"]')?.className).toContain('240px')
    expect(localStorage.getItem('pm.navigation.collapsed')).toBe('false')
    localStorage.removeItem('pm.navigation.collapsed')
  })

  it('locks the documents workspace to the viewport so its four columns own scrolling', () => {
    renderShell({ currentPath: '/workspace/documents/2' })
    expect(screen.getByRole('main').className).toContain('flex overflow-hidden')
  })

  it('renders business and host-management navigation with accessible landmarks', () => {
    const { container, onMcpCli, onNavigate } = renderShell()
    const navigation = screen.getByRole('navigation', {
      name: copy.app.shell.primaryNavigation,
    })
    const links = Array.from(navigation.querySelectorAll('a'))

    expect(links.map((link) => link.textContent)).toEqual([
      copy.app.navigation.inbox,
      copy.app.navigation.biScreen,
      copy.app.navigation.reporting,
      copy.app.navigation.projects,
      copy.app.navigation.requirements,
      copy.app.navigation.documents,
      copy.app.navigation.workItems,
      copy.app.navigation.designReviews,
      copy.app.navigation.quality,
      copy.app.navigation.testing,
      copy.app.navigation.workflows,
      copy.app.navigation.aiRuns,
      copy.app.navigation.workspaceManagement,
      copy.app.navigation.accessManagement,
      copy.app.navigation.hostNavigation,
      copy.app.navigation.scmIntegration,
    ])
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/workspace/inbox',
      '/workspace/bi-screen',
      '/workspace/reporting',
      '/workspace/projects',
      '/workspace/requirements',
      '/workspace/documents',
      '/workspace/work-items',
      '/workspace/design-management',
      '/workspace/quality-management',
      '/workspace/testing',
      '/workspace/pm/workflows',
      '/workspace/pm/ai-runs',
      '/workspace/pm/workspace-management',
      '/workspace/pm/access-management',
      '/workspace/pm/host-navigation',
      '/workspace/pm/scm-integration',
    ])
    expect(screen.getAllByRole('heading', { level: 2 }).map((heading) => heading.textContent)).toEqual([
      copy.app.shell.navigationGroups.workbench,
      copy.app.shell.navigationGroups.delivery,
      copy.app.shell.navigationGroups.assurance,
      copy.app.shell.navigationGroups.intelligence,
      copy.app.shell.navigationGroups.system,
    ])
    expect(screen.getByRole('link', { name: copy.app.navigation.projects }).getAttribute('aria-current'))
      .toBe('page')
    expect(screen.getByRole('main').id).toBe('pm-main-content')
    expect(screen.getByRole('link', { name: copy.app.shell.skipToContent }).getAttribute('href'))
      .toBe('#pm-main-content')
    expect(screen.getByRole('button', { name: copy.app.shell.accountMenu })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.app.shell.rgbMcpEntryLabel }).textContent)
      .toBe(copy.app.shell.mcpCliEntry)
    expect(screen.getByRole('button', { name: copy.ui.appearance.menu })).toBeTruthy()

    fireEvent.click(screen.getByRole('link', { name: copy.app.navigation.inbox }))
    expect(onNavigate).toHaveBeenCalledWith('/workspace/inbox')
    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.rgbMcpEntryLabel }))
    expect(onMcpCli).toHaveBeenCalledTimes(1)

    expect(container.querySelector('[data-slot="brand-mark"]')).toBeTruthy()
    expect(container.querySelector('[data-slot="standalone-brand"] img')).toBeNull()
    expect(container.querySelector('[data-slot="standalone-shell"]')?.className)
      .toContain('[grid-template-rows:44px_minmax(0,1fr)]')
    expect(container.querySelector('[data-slot="standalone-shell-body"]')?.className)
      .toContain('[grid-template-columns:240px_minmax(0,1fr)]')
    expect(container.querySelector('[data-slot="standalone-navigation"]')?.className)
      .toContain('max-[720px]:overflow-x-auto')
  })

  it('exposes workspace switching and account actions through the account dock', async () => {
    const { onLogout, onProfile, onReleaseNotes, onWorkspaceChange } = renderShell()

    const openDock = async () => {
      fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
      return await screen.findByRole('menu')
    }

    const menu = await openDock()
    const option = within(menu).getByRole('menuitem', { name: 'Workspace Nine' })
    fireEvent.click(option)
    expect(onWorkspaceChange).toHaveBeenCalledWith(9)

    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    fireEvent.click(await screen.findByRole('menuitem', { name: copy.app.shell.profile }))
    expect(onProfile).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    fireEvent.click(await screen.findByRole('menuitem', { name: copy.app.shell.versionInfo }))
    expect(onReleaseNotes).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    fireEvent.click(await screen.findByRole('menuitem', { name: copy.app.shell.logout }))
    expect(onLogout).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: copy.ui.appearance.menu })).toBeTruthy()
  })

  it('offers workspace onboarding from the account dock', async () => {
    const onWorkspaceCreate = vi.fn()
    renderShell({ onWorkspaceCreate })

    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    fireEvent.click(await screen.findByRole('menuitem', { name: copy.app.shell.createOrJoinWorkspace }))
    expect(onWorkspaceCreate).toHaveBeenCalledTimes(1)
  })

  it('opens the account dock as a labelled menu with account actions', async () => {
    renderShell()
    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    const menu = await screen.findByRole('menu')
    expect(menu.getAttribute('aria-label')).toBe(copy.app.shell.accountMenu)
    const profileItem = screen.getByRole('menuitem', { name: copy.app.shell.profile })
    expect(profileItem.className).toContain('hover:bg-muted')
  })

  it('announces workspace failures inside the account dock and exposes a retry action', async () => {
    const onWorkspaceRetry = vi.fn()
    renderShell({
      workspaceError: copy.workspace.loadFailed,
      onWorkspaceRetry,
    })

    fireEvent.click(screen.getByRole('button', { name: copy.app.shell.accountMenu }))
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain(copy.workspace.loadFailed)

    fireEvent.click(screen.getByRole('button', { name: copy.workspace.retry }))
    expect(onWorkspaceRetry).toHaveBeenCalledTimes(1)
  })

  it('announces workspace switching on the account dock', () => {
    renderShell({ workspaceDisabled: true, workspaceSwitching: true })

    expect(screen.getByRole('status').textContent).toBe(copy.workspace.switching)
    expect(screen.getByRole('button', { name: copy.app.shell.accountMenu }).getAttribute('aria-busy')).toBe('true')
  })

  it('fills the main surface for host-content and project-management routes', () => {
    const { rerender } = renderShell({ currentPath: '/workspace/inbox' })
    expect(screen.getByRole('main').className).toContain('overflow-hidden')

    rerender(
      <Standalone
        productName="PM Product"
        shortName="PM"
        workspaceId={7}
        workspaceOptions={[]}
        currentPath="/workspace/projects"
        onNavigate={vi.fn()}
        onMcpCli={vi.fn()}
        onWorkspaceChange={vi.fn()}
        onProfile={vi.fn()}
        onLogout={vi.fn()}
      >
        <div>Content</div>
      </Standalone>,
    )
    expect(screen.getByRole('main').className).toContain('flex')
    expect(screen.getByRole('main').className).toContain('overflow-hidden')
    expect(screen.getByRole('main').className).not.toContain('overflow-auto')

    rerender(
      <Standalone
        productName="PM Product"
        shortName="PM"
        workspaceId={7}
        workspaceOptions={[]}
        currentPath="/workspace/requirements"
        onNavigate={vi.fn()}
        onMcpCli={vi.fn()}
        onWorkspaceChange={vi.fn()}
        onProfile={vi.fn()}
        onLogout={vi.fn()}
      >
        <div>Content</div>
      </Standalone>,
    )
    expect(screen.getByRole('main').className).toContain('flex')
    expect(screen.getByRole('main').className).toContain('overflow-hidden')
    expect(screen.getByRole('main').className).not.toContain('overflow-auto')
  })

  it('filters inaccessible navigation items while preserving the default direct-render contract', () => {
    renderShell({
      isNavigationPathVisible: (path) => path === '/workspace/projects',
    })

    const navigation = screen.getByRole('navigation', {
      name: copy.app.shell.primaryNavigation,
    })
    expect(within(navigation).getByRole('link', { name: copy.app.navigation.projects })).toBeTruthy()
    expect(within(navigation).queryByRole('link', { name: copy.app.navigation.accessManagement })).toBeNull()
    expect(within(navigation).getAllByRole('heading', { level: 2 })).toHaveLength(1)
  })
})
