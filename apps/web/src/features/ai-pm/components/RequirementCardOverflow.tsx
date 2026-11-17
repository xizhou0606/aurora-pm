import { useState } from 'react'
import { PhDotsThree, PhFolderPlus, PhTrash } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import {
  Button,
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
  buttonVariants,
} from '@/shared/ui'
import type { RequirementTableRow } from './RequirementPoolTable'

const poolCopy = copy.aiPM.requirementPool

export function RequirementCardOverflow({
  requirement,
  onMove,
  onDelete,
}: {
  requirement: RequirementTableRow
  onMove?: (requirement: RequirementTableRow) => void
  onDelete?: (requirement: RequirementTableRow) => void
}) {
  const [open, setOpen] = useState(false)
  const choose = (action: (requirement: RequirementTableRow) => void) => {
    setOpen(false)
    action(requirement)
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        type="button"
        aria-label={poolCopy.moreActionsFor(requirement.title)}
        title={poolCopy.moreActions}
        data-slot="requirement-card-overflow"
        className={buttonVariants({
          variant: 'ghost',
          size: 'icon-sm',
          className: 'rounded-full border border-border bg-card text-muted-foreground pointer-events-auto opacity-0 group-hover:opacity-100 focus-visible:opacity-100 aria-expanded:opacity-100 data-popup-open:opacity-100 hover:bg-card hover:text-foreground [@media(hover:none)]:opacity-100',
        })}
        onClick={(event) => event.stopPropagation()}
      >
        <PhDotsThree aria-hidden="true" />
      </PopoverTrigger>
      <PopoverContent align="start" sideOffset={6} className="w-52 p-1">
        <div role="menu" aria-label={poolCopy.moreActions} className="grid gap-0.5">
          <PopoverTitle className="sr-only">{poolCopy.moreActions}</PopoverTitle>
          <PopoverDescription className="sr-only">{poolCopy.moreActionsFor(requirement.title)}</PopoverDescription>
          {onMove ? <Button
            type="button"
            role="menuitem"
            variant="ghost"
            className="h-8 justify-start gap-2 px-2 font-normal"
            onClick={(event) => {
              event.stopPropagation()
              choose(onMove)
            }}
          >
            <PhFolderPlus aria-hidden="true" />
            {poolCopy.moveTo}
          </Button> : null}
          {onMove && onDelete ? <div className="my-0.5 h-px bg-border" /> : null}
          {onDelete ? <Button
            type="button"
            role="menuitem"
            variant="ghost"
            className="h-8 justify-start gap-2 px-2 font-normal text-destructive hover:bg-destructive/10 hover:text-destructive"
            onClick={(event) => {
              event.stopPropagation()
              choose(onDelete)
            }}
          >
            <PhTrash aria-hidden="true" />
            {poolCopy.delete}
          </Button> : null}
        </div>
      </PopoverContent>
    </Popover>
  )
}