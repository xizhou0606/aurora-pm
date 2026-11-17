import { Button } from '@/shared/ui'
import { PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'
import { useCallback } from 'react'
import { useRouter } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { shouldUseClientNavigation } from '@/shared/navigation'

import { normalizedShellPath, SHELL_NAVIGATION_GROUPS } from './registry'
import { AccountDock, type AccountDockProps } from './AccountDock'

export interface NavigationProps {
  collapsed?: boolean
  onToggle?: () => void
  currentPath: string
  onNavigate: (path: string) => void
  isPathVisible?: (path: string) => boolean
  account: AccountDockProps
}

export function Navigation({ currentPath, onNavigate, isPathVisible = () => true, account, collapsed = false, onToggle }: NavigationProps) {
  const router = useRouter({ warn: false })
  const path = normalizedShellPath(currentPath)
  const groups = SHELL_NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => isPathVisible(item.path)),
  })).filter((group) => group.items.length > 0)
  // Sidebar items are plain anchors, so the router-wide `defaultPreload:
  // 'intent'` never fires for them. Warm the target route (chunk + loader
  // prefetch) on hover/focus so menu switches commit without the frozen-page
  // delay.
  const preloadNavigationTarget = useCallback((targetPath: string) => {
    try {
      void router.preloadRoute({ to: targetPath } as never).catch(() => undefined)
    } catch {
      // Targets outside the route tree cannot be preloaded; the click still
      // navigates normally.
    }
  }, [router])

  return (
    <aside
      data-slot="standalone-navigation"
      data-test="pm-primary-navigation"
      className="flex min-w-0 flex-col border-r border-border/70 bg-muted/50 px-2.5 py-3 dark:bg-sidebar max-[720px]:flex max-[720px]:items-center max-[720px]:overflow-x-auto max-[720px]:border-r-0 max-[720px]:border-b max-[720px]:px-2.5 max-[720px]:py-1"
    >
      <nav
        id="pm-navigation-items"
        aria-label={copy.app.shell.primaryNavigation}
        className="grid min-h-0 flex-1 content-start gap-3.5 overflow-y-auto max-[720px]:flex max-[720px]:gap-1 max-[720px]:overflow-y-visible"
      >
        {groups.map((group, index) => (
          <section
            key={group.id}
            aria-labelledby={`pm-navigation-group-${group.id}`}
            data-navigation-group={group.id}
            className="grid gap-0.75 max-[720px]:contents"
          >
            <div className={cn("flex items-center max-[720px]:hidden", collapsed ? "justify-center" : "justify-between")}>
            <h2
              id={`pm-navigation-group-${group.id}`}
              data-slot="standalone-navigation-group-title"
              className={cn("m-0 px-2.5 pb-1 text-xs font-medium tracking-wide text-muted-foreground/80", collapsed && "sr-only")}
            >
              {group.label}
            </h2>
            {index === 0 && onToggle ? (
              <Button type="button" variant="ghost" size="sm" className="size-8 shrink-0 rounded-md p-0 text-muted-foreground hover:bg-background hover:text-foreground" onClick={onToggle}
                aria-label={collapsed ? copy.app.shell.expandNavigation : copy.app.shell.collapseNavigation}
                title={collapsed ? copy.app.shell.expandNavigation : copy.app.shell.collapseNavigation}
                aria-expanded={!collapsed} aria-controls="pm-navigation-items">
                {collapsed ? <PhCaretRight aria-hidden="true" /> : <PhCaretLeft aria-hidden="true" />}
              </Button>
            ) : null}
            </div>
            {group.items.map((item) => {
              const active = item.active(path)
              const NavIcon = active ? item.icon.fill : item.icon.regular

              return (
                <a
                  key={item.id}
                  href={item.path}
                  title={collapsed ? item.label : undefined}
                  aria-current={active ? 'page' : undefined}
                  data-test={item.testId}
                  data-navigation-id={item.id}
                  className={cn(
                    'group relative flex min-h-8 min-w-0 items-center gap-2 rounded-lg border border-transparent px-2.5 text-sm text-muted-foreground no-underline outline-none transition-colors duration-150',
                    'hover:bg-background/70 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring',
                    'max-[720px]:min-h-11 max-[720px]:shrink-0 max-[720px]:whitespace-nowrap',
                    collapsed && 'min-[721px]:justify-center min-[721px]:px-0',
                    active && 'border-border/80 bg-card font-medium text-foreground shadow-xs hover:bg-card dark:border-border',
                  )}
                  onMouseEnter={() => preloadNavigationTarget(item.path)}
                  onFocus={() => preloadNavigationTarget(item.path)}
                  onClick={(event) => {
                    if (!shouldUseClientNavigation(event)) return
                    event.preventDefault()
                    onNavigate(item.path)
                  }}
                >
                  <NavIcon aria-hidden="true" className="size-4 shrink-0" />
                  <span className={collapsed ? "min-[721px]:sr-only" : undefined}>{item.label}</span>
                </a>
              )
            })}
          </section>
        ))}
      </nav>
      <div
        data-slot="standalone-account-dock"
        className="mt-auto pt-3 max-[720px]:ml-auto max-[720px]:border-l max-[720px]:border-border/60 max-[720px]:pl-1.5 max-[720px]:pt-0"
      >
        <AccountDock {...account} compact={collapsed} />
      </div>
    </aside>
  )
}
