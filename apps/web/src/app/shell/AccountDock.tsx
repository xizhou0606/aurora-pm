import { cn } from '@/shared/lib/utils'
import { useState } from 'react'
import {
  PhCheck,
  PhDotsThree,
  PhInfo,
  PhPlus,
  PhSignOut,
  PhUser,
} from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import {
  Button,
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  UserPreview,
} from '@/shared/ui'

export type ShellWorkspaceOption = {
  id: number
  label: string
  disabled?: boolean
}

function initial(name: string): string {
  return [...name.trim()][0]?.toLocaleUpperCase() || '·'
}

const MENU_ITEM_CLASS =
  'flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm outline-none hover:bg-muted focus-visible:bg-muted'

// ChatGPT-style account dock: a compact identity row pinned to the sidebar
// bottom, opening an upward popover with the workspace list, workspace
// onboarding, and account actions.
export type AccountDockProps = {
  compact?: boolean
  userLabel?: string
  username?: string
  userAvatarUrl?: string
  workspaceId: number
  workspaceOptions: readonly ShellWorkspaceOption[]
  workspaceLoading?: boolean
  workspaceSwitching?: boolean
  workspaceError?: string
  logoutPending?: boolean
  onWorkspaceRetry?: () => void
  onWorkspaceChange: (workspaceId: number) => void
  onWorkspaceCreate?: () => void
  onProfile?: () => void
  onReleaseNotes?: () => void
  onLogout: () => void
}

export function AccountDock({
  compact = false,
  userLabel,
  username,
  userAvatarUrl,
  workspaceId,
  workspaceOptions,
  workspaceLoading = false,
  workspaceSwitching = false,
  workspaceError,
  logoutPending = false,
  onWorkspaceRetry,
  onWorkspaceChange,
  onWorkspaceCreate,
  onProfile,
  onReleaseNotes,
  onLogout,
}: {
  compact?: boolean
  userLabel?: string
  username?: string
  userAvatarUrl?: string
  workspaceId: number
  workspaceOptions: readonly ShellWorkspaceOption[]
  workspaceLoading?: boolean
  workspaceSwitching?: boolean
  workspaceError?: string
  logoutPending?: boolean
  onWorkspaceRetry?: () => void
  onWorkspaceChange: (workspaceId: number) => void
  onWorkspaceCreate?: () => void
  onProfile?: () => void
  onReleaseNotes?: () => void
  onLogout: () => void
}) {
  const [open, setOpen] = useState(false)

  return (
    <div data-slot="account-dock">
      {workspaceSwitching ? (
        <span role="status" aria-live="polite" className="sr-only">
          {copy.workspace.switching}
        </span>
      ) : null}
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          type="button"
          aria-label={copy.app.shell.accountMenu}
          aria-busy={workspaceSwitching || undefined}
          data-test="pm-account-dock"
          className={cn("group flex w-full items-center gap-2 rounded-lg border border-border/80 bg-card p-2 text-left shadow-xs outline-none transition-colors duration-150 hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-muted max-[720px]:w-auto max-[720px]:border-transparent max-[720px]:bg-transparent max-[720px]:p-1 max-[720px]:shadow-none", compact && "min-[721px]:justify-center min-[721px]:gap-0 min-[721px]:px-0")}
        >
          <UserPreview
            name={userLabel || copy.app.shell.accountMenu}
            avatarUrl={userAvatarUrl}
            size="sm"
            showName={false}
          />
          <span className={cn("min-w-0 flex-1 max-[720px]:hidden", compact && "min-[721px]:hidden")}>
            <span className="block truncate text-sm font-medium leading-4 text-foreground">
              {userLabel}
            </span>
            <span className="block truncate text-xs leading-4 text-muted-foreground">
              {username ? `@${username}` : copy.app.shell.workspaceLabel}
            </span>
          </span>
          <PhDotsThree
            aria-hidden="true"
            className={cn("size-4 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground max-[720px]:hidden", compact && "min-[721px]:hidden")}
          />
        </PopoverTrigger>
        <PopoverContent
          side="top"
          align="start"
          sideOffset={8}
          role="menu"
          aria-label={copy.app.shell.accountMenu}
          className="w-64 rounded-xl p-1.5"
        >
          <PopoverTitle className="flex items-center gap-2 px-2 py-1.5">
            <UserPreview
              name={userLabel || copy.app.shell.accountMenu}
              secondary={username ? `@${username}` : undefined}
              avatarUrl={userAvatarUrl}
              size="md"
            />
          </PopoverTitle>
          <div className="my-1 border-t border-border" aria-hidden="true" />
          <div role="group" aria-label={copy.app.shell.workspaceLabel}>
            {workspaceOptions.map((option) => (
              <PopoverClose
                key={option.id}
                type="button"
                role="menuitem"
                disabled={option.disabled || workspaceSwitching}
                className={`${MENU_ITEM_CLASS} disabled:opacity-50`}
                onClick={() => onWorkspaceChange(option.id)}
              >
                <span
                  aria-hidden="true"
                  className="grid size-5 shrink-0 place-items-center rounded-md border border-border bg-muted text-xs font-semibold text-foreground"
                >
                  {initial(option.label)}
                </span>
                <span className="min-w-0 flex-1 truncate text-left">{option.label}</span>
                {option.id === workspaceId ? (
                  <PhCheck aria-hidden="true" className="size-4 shrink-0 text-foreground" />
                ) : null}
              </PopoverClose>
            ))}
            {workspaceLoading ? (
              <p className="px-2 py-1.5 text-xs text-muted-foreground">
                {copy.app.shell.workspacePlaceholder}
              </p>
            ) : null}
            {workspaceError ? (
              <div className="grid gap-1.5 px-2 py-1.5">
                <p role="alert" className="m-0 text-xs leading-4 text-destructive">{workspaceError}</p>
                {onWorkspaceRetry ? (
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    className="justify-self-start"
                    onClick={onWorkspaceRetry}
                  >
                    {copy.workspace.retry}
                  </Button>
                ) : null}
              </div>
            ) : null}
            {onWorkspaceCreate ? (
              <PopoverClose
                type="button"
                role="menuitem"
                className={MENU_ITEM_CLASS}
                onClick={onWorkspaceCreate}
              >
                <PhPlus aria-hidden="true" className="size-4 text-muted-foreground" />
                {copy.app.shell.createOrJoinWorkspace}
              </PopoverClose>
            ) : null}
          </div>
          <div className="my-1 border-t border-border" aria-hidden="true" />
          {onProfile ? (
            <PopoverClose
              type="button"
              role="menuitem"
              className={MENU_ITEM_CLASS}
              onClick={onProfile}
            >
              <PhUser aria-hidden="true" className="size-4" />
              {copy.app.shell.profile}
            </PopoverClose>
          ) : null}
          {onReleaseNotes ? (
            <PopoverClose
              type="button"
              role="menuitem"
              className={MENU_ITEM_CLASS}
              onClick={onReleaseNotes}
            >
              <PhInfo aria-hidden="true" className="size-4" />
              {copy.app.shell.versionInfo}
            </PopoverClose>
          ) : null}
          <PopoverClose
            type="button"
            role="menuitem"
            disabled={logoutPending}
            aria-busy={logoutPending || undefined}
            className={`${MENU_ITEM_CLASS} disabled:opacity-50`}
            onClick={onLogout}
          >
            <PhSignOut aria-hidden="true" className="size-4" />
            {logoutPending ? copy.app.shell.loggingOut : copy.app.shell.logout}
          </PopoverClose>
        </PopoverContent>
      </Popover>
    </div>
  )
}
