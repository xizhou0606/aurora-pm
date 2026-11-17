import { useEffect, useRef, useState } from 'react'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '../primitives/popover'
import { UserPreview } from './user-preview'

const stackCopy = copy.ui.userFaceStack
const DEFAULT_MAX_VISIBLE = 2
const OPEN_DELAY_MS = 80
const CLOSE_DELAY_MS = 140

export type UserFace = {
  id?: number | string
  name: string
  avatarUrl?: string
}

export type UserFaceStackProps = {
  people: readonly UserFace[]
  emptyLabel?: string
  size?: 'sm' | 'md'
  maxVisible?: number
  className?: string
  /**
   * When false the stack renders as plain, non-interactive faces without the
   * hover roster popover — for embedding inside another clickable control
   * (buttons must not nest).
   */
  interactive?: boolean
}

function faceKey(face: UserFace, index: number) {
  return face.id == null ? `${face.name}-${index}` : String(face.id)
}

function useHoverPopover() {
  const [open, setOpen] = useState(false)
  const openTimer = useRef(0)
  const closeTimer = useRef(0)
  const cancel = () => {
    window.clearTimeout(openTimer.current)
    window.clearTimeout(closeTimer.current)
  }
  useEffect(() => cancel, [])
  return {
    open,
    setOpen,
    onPointerEnter: () => {
      window.clearTimeout(closeTimer.current)
      openTimer.current = window.setTimeout(() => setOpen(true), OPEN_DELAY_MS)
    },
    onPointerLeave: () => {
      window.clearTimeout(openTimer.current)
      closeTimer.current = window.setTimeout(() => setOpen(false), CLOSE_DELAY_MS)
    },
  }
}

function EmptyFace({
  label,
  size,
}: {
  label: string
  size: 'sm' | 'md'
}) {
  return (
    <span className="flex min-w-0 max-w-full items-center gap-1.5 text-muted-foreground" title={label}>
      <span
        data-slot="user-face-stack-empty"
        aria-hidden="true"
        className={cn(
          'shrink-0 rounded-full border border-dashed border-border bg-muted',
          size === 'sm' ? 'size-5' : 'size-7',
        )}
      />
      <span className={cn('min-w-0 truncate', size === 'sm' ? 'text-xs' : 'text-sm')}>{label}</span>
    </span>
  )
}

function FaceDisc({
  face,
  size,
  overlap,
}: {
  face: UserFace
  size: 'sm' | 'md'
  overlap: boolean
}) {
  return (
    <span
      className={cn(
        'relative shrink-0 rounded-full ring-1 ring-background',
        overlap && (size === 'sm' ? '-ml-1.5' : '-ml-2'),
      )}
    >
      <UserPreview
        name={face.name}
        avatarUrl={face.avatarUrl}
        size={size}
        showName={false}
        title={face.name}
      />
    </span>
  )
}

function FaceStack({
  people,
  size,
  extra,
}: {
  people: readonly UserFace[]
  size: 'sm' | 'md'
  extra: number
}) {
  return (
    <span data-slot="user-face-stack" className="flex min-w-0 items-center">
      {people.map((face, index) => (
        <FaceDisc key={faceKey(face, index)} face={face} size={size} overlap={index > 0} />
      ))}
      {extra > 0 ? (
        <span
          data-slot="user-face-stack-extra"
          aria-hidden="true"
          className={cn(
            'relative grid shrink-0 place-items-center rounded-full bg-muted-foreground font-semibold text-background ring-1 ring-background',
            size === 'sm'
              ? '-ml-1.5 h-5 min-w-5 px-1 text-xs leading-none'
              : '-ml-2 h-7 min-w-7 px-1.5 text-xs',
          )}
        >
          {stackCopy.extra(extra)}
        </span>
      ) : null}
    </span>
  )
}

export function UserFaceStack({
  people,
  emptyLabel = stackCopy.empty,
  size = 'sm',
  maxVisible = DEFAULT_MAX_VISIBLE,
  className,
  interactive = true,
}: UserFaceStackProps) {
  const hover = useHoverPopover()
  if (people.length === 0) {
    return <EmptyFace label={emptyLabel} size={size} />
  }
  if (people.length === 1) {
    return (
      <UserPreview
        name={people[0].name}
        avatarUrl={people[0].avatarUrl}
        size={size}
        title={people[0].name}
        className={cn('max-w-full', className)}
      />
    )
  }
  const visible = people.slice(0, Math.max(1, maxVisible))
  const extra = Math.max(0, people.length - visible.length)
  if (!interactive) {
    return (
      <span className={cn('inline-flex min-w-0 max-w-full items-center', className)}>
        <FaceStack people={visible} size={size} extra={extra} />
      </span>
    )
  }
  const names = people.map((face) => face.name).join(stackCopy.nameSeparator)
  const hoverHandlers = {
    onPointerEnter: hover.onPointerEnter,
    onPointerLeave: hover.onPointerLeave,
  }
  return (
    <Popover open={hover.open} onOpenChange={hover.setOpen}>
      <PopoverTrigger
        type="button"
        aria-label={stackCopy.openRoster(names)}
        className={cn(
          'inline-flex h-auto min-h-0 min-w-0 max-w-full items-center rounded-md px-0.5 py-0.5 hover:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
          className,
        )}
        {...hoverHandlers}
      >
        <FaceStack people={visible} size={size} extra={extra} />
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="z-50 min-w-40 p-1.5"
        {...hoverHandlers}
      >
        <PopoverTitle className="sr-only">{stackCopy.roster}</PopoverTitle>
        <PopoverDescription className="sr-only">{names}</PopoverDescription>
        <ul className="m-0 grid list-none gap-1 p-0">
          {people.map((face, index) => (
            <li key={faceKey(face, index)} className="min-w-0">
              <UserPreview name={face.name} avatarUrl={face.avatarUrl} size={size} />
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}
