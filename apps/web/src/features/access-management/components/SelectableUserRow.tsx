import { PhCheckBold } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'
import { UserIdentity } from '@/shared/ui'

export type SelectableUserRowUser = {
  id: number
  name: string
  username?: string
  email?: string
  secondary?: string
  avatarUrl?: string
}

type SelectableUserRowProps = {
  user: SelectableUserRowUser
  selected: boolean
  label: string
  disabled?: boolean
  onToggle: () => void
}

export function SelectableUserRow({
  user,
  selected,
  label,
  disabled = false,
  onToggle,
}: SelectableUserRowProps) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={selected}
      aria-label={label}
      disabled={disabled}
      className={cn(
        'flex w-full cursor-pointer items-center gap-3 rounded-lg px-2 py-2 text-left outline-none transition-control',
        'enabled:hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50',
        selected && 'bg-accent text-accent-foreground',
      )}
      onClick={onToggle}
    >
      <span
        aria-hidden="true"
        className={cn(
          'grid size-4 shrink-0 place-items-center rounded-sm border shadow-xs transition-control',
          selected ? 'border-primary bg-primary text-primary-foreground' : 'border-input bg-background',
        )}
      >
        {selected ? <PhCheckBold className="size-3" /> : null}
      </span>
      <UserIdentity
        className="min-w-0 flex-1"
        name={user.name}
        secondary={user.secondary || user.email || user.username}
        avatarUrl={user.avatarUrl}
      />
    </button>
  )
}
