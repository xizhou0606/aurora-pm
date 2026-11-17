import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useOverlayPortalContainer } from '../misc/overlay-portal-container'
import { Button } from '../primitives/button'
import { UserPreview } from '../user/user-preview'
import type { RichTextMention } from './lazy-rich-text-field'

interface MentionPopoverProps {
  filteredMentions: readonly RichTextMention[]
  activeMentionIndex: number
  mentionListLabel: string
  mentionEmptyLabel: string
  setActiveMentionIndex: (index: number) => void
  selectMention: (option: RichTextMention) => void
}

export function RichTextMentionPopover({
  filteredMentions,
  activeMentionIndex,
  mentionListLabel,
  mentionEmptyLabel,
  setActiveMentionIndex,
  selectMention,
}: MentionPopoverProps) {
  const anchor = useRef<HTMLSpanElement>(null)
  const portal = useOverlayPortalContainer()
  const [position, setPosition] = useState<{ left: number; top: number; width: number; maxHeight: number }>()
  useLayoutEffect(() => {
    const update = () => {
      const bounds = anchor.current?.parentElement?.getBoundingClientRect()
      if (!bounds) return
      const width = Math.min(Math.max(bounds.width, 240), window.innerWidth - 16)
      const below = window.innerHeight - bounds.bottom - 8
      const maxHeight = Math.max(0, Math.min(224, below >= 160 ? below : bounds.top - 12))
      setPosition({ width, maxHeight, left: Math.max(8, Math.min(bounds.left, window.innerWidth - width - 8)),
        top: below >= 160 ? bounds.bottom + 4 : Math.max(8, bounds.top - maxHeight - 4) })
    }
    update()
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => { window.removeEventListener('resize', update); window.removeEventListener('scroll', update, true) }
  }, [])
  return <><span ref={anchor} aria-hidden="true" />{position ? createPortal(
    <div
      role="listbox"
      aria-label={mentionListLabel}
      className="fixed top-(--mention-popover-top) left-(--mention-popover-left) z-[60] isolate w-(--mention-popover-width) max-h-(--mention-popover-max-h) overflow-y-auto rounded-lg border border-border bg-background p-1 text-foreground shadow-xl"
      style={{
        '--mention-popover-top': `${position.top}px`,
        '--mention-popover-left': `${position.left}px`,
        '--mention-popover-width': `${position.width}px`,
        '--mention-popover-max-h': `${position.maxHeight}px`,
      }}
    >
      {filteredMentions.length ? filteredMentions.map((option, index) => (
        <Button
          key={option.id}
          type="button"
          role="option"
          aria-selected={index === activeMentionIndex}
          tabIndex={-1}
          variant="ghost"
          className="h-auto w-full justify-start px-2 py-1.5 data-[active=true]:bg-muted"
          data-active={index === activeMentionIndex}
          onPointerMove={() => setActiveMentionIndex(index)}
          onPointerDown={(event) => {
            event.preventDefault()
            selectMention(option)
          }}
        >
          <UserPreview
            name={option.label}
            secondary={option.username ? `@${option.username}` : undefined}
            avatarUrl={option.avatarUrl}
            size="md"
          />
        </Button>
      )) : (
        <p role="status" className="m-0 px-2 py-3 text-sm text-muted-foreground">
          {mentionEmptyLabel}
        </p>
      )}
    </div>, portal ?? document.body,
  ) : null}</>
}
