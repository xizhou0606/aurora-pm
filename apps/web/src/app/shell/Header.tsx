import { BrandMark, ThemeAppearanceControl } from '@/shared/ui'
import { MentionNotifications } from '@/features/notifications'
import { RgbMcpEntry } from './RgbMcpEntry'

export type ShellWorkspaceOption = {
  id: number
  label: string
  disabled?: boolean
}

export interface HeaderProps {
  productName: string
  appVersion?: string
  appVersionSummary?: readonly string[]
  appReleaseDate?: string
  shortName: string
  userLabel?: string
  userUsername?: string
  userAvatarUrl?: string
  workspaceId: number
  workspaceOptions: readonly ShellWorkspaceOption[]
  workspaceLoading?: boolean
  workspaceDisabled?: boolean
  workspaceSwitching?: boolean
  workspaceError?: string
  logoutPending?: boolean
  mcpCliHref?: string
  onWorkspaceRetry?: () => void
  onWorkspaceChange: (workspaceId: number) => void
  onWorkspaceCreate?: () => void
  onMcpCli?: () => void
  onProfile?: () => void
  onLogout: () => void
}

// The header only carries the product identity, the MCP/CLI entry and the
// appearance control. The workspace switcher and account actions live in the
// sidebar account dock (ChatGPT-style), see AccountDock.
export function Header({
  productName,
  mcpCliHref,
  onMcpCli,
}: Pick<HeaderProps, 'productName' | 'mcpCliHref' | 'onMcpCli'>) {
  return (
    <header
      data-slot="standalone-header"
      className="relative z-[4] flex items-center justify-between gap-4 border-b border-border/70 bg-card px-4 max-[720px]:min-h-12 max-[720px]:flex-wrap max-[720px]:px-2.5 max-[720px]:py-2"
    >
      <div
        data-slot="standalone-brand"
        className="flex min-w-0 items-center gap-2.5 text-foreground"
      >
        <BrandMark className="text-primary" size={26} />
        <strong className="truncate text-sm font-semibold tracking-tight max-[720px]:hidden">{productName}</strong>
      </div>

      <div
        data-slot="standalone-header-actions"
        className="flex min-w-0 items-center gap-2"
      >
        <MentionNotifications />
        <RgbMcpEntry href={mcpCliHref} onActivate={onMcpCli} />
        <ThemeAppearanceControl triggerClassName="size-8 rounded-lg border border-border/70 bg-background text-muted-foreground shadow-xs hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring" />
      </div>
    </header>
  )
}
