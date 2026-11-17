import type { ReactNode } from 'react'
import { useState, useRef } from 'react'
import { PhClipboardText, PhCircleNotch, PhNotebook, PhPenNib, PhUploadSimple, type IconComponent as IconComponent } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import {
  Button,
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
  buttonVariants,
} from '@/shared/ui'

const poolCopy = copy.aiPM.requirementPool

export type DeliverableKind = 'document' | 'design' | 'test-cases'
export type DeliverablePresentation = 'chip' | 'icon'

export const deliverableStyles: Record<DeliverableKind, { chip: string; icon: string; Icon: IconComponent; shortLabel: string }> = {
  document: {
    chip: 'border-emerald-100 dark:border-emerald-900 bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-100',
    icon: 'text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/40',
    Icon: PhNotebook,
    shortLabel: poolCopy.deliverableDocumentShort,
  },
  design: {
    chip: 'pm-deliverable--design border-cyan-100 text-cyan-700 dark:text-cyan-300 hover:bg-cyan-100',
    icon: 'text-cyan-600 hover:bg-cyan-50 dark:hover:bg-cyan-950/40',
    Icon: PhPenNib,
    shortLabel: poolCopy.deliverableDesignShort,
  },
  'test-cases': {
    chip: 'border-border bg-muted text-foreground hover:bg-muted/70',
    icon: 'text-muted-foreground hover:bg-muted',
    Icon: PhClipboardText,
    shortLabel: poolCopy.deliverableTestCasesShort,
  },
}

export function DeliverableMark({
  kind,
  muted = false,
  presentation = 'chip',
}: {
  kind: DeliverableKind
  muted?: boolean
  presentation?: DeliverablePresentation
}) {
  const Icon = deliverableStyles[kind].Icon
  if (presentation === 'icon') return <Icon aria-hidden="true" className="size-4" />
  return (
    <span className="grid justify-items-center gap-0.5">
      <Icon aria-hidden="true" className="size-4" />
      <span className={muted ? 'text-xs leading-3 font-medium text-muted-foreground' : 'text-xs leading-3 font-medium'}>
        {deliverableStyles[kind].shortLabel}
      </span>
    </span>
  )
}

export function DeliverableMissing({
  kind,
  label,
  presentation = 'chip',
}: {
  kind: DeliverableKind
  label: string
  presentation?: DeliverablePresentation
}) {
  return (
    <span
      role="img"
      aria-label={poolCopy.deliverableMissing(label)}
      title={poolCopy.deliverableMissing(label)}
      className={cn(
        'inline-flex shrink-0 items-center justify-center text-muted-foreground',
        presentation === 'icon'
          ? 'size-7 rounded-md'
          : 'h-auto min-h-9 min-w-9 flex-col rounded-lg border border-border bg-muted px-1.5 py-1',
      )}
    >
      <DeliverableMark kind={kind} muted presentation={presentation} />
    </span>
  )
}

export const DESIGN_UPLOAD_ACCEPT = 'image/png,image/jpeg,image/webp,image/gif,.png,.jpg,.jpeg,.webp,.gif'
export const TEST_CASE_UPLOAD_ACCEPT = '.csv,.xls,.xlsx,text/csv,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

export function DeliverableFilePicker({
  accept,
  disabled,
  onPick,
  children,
}: {
  accept: string
  disabled?: boolean
  onPick: (file: File) => void
  children: (open: () => void) => ReactNode
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  return (
    <>
      {children(() => inputRef.current?.click())}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        hidden
        tabIndex={-1}
        disabled={disabled}
        onChange={(event) => {
          const file = event.target.files?.[0]
          event.target.value = ''
          if (file) onPick(file)
        }}
      />
    </>
  )
}

export function DeliverableUploadExtra({
  label,
  pending,
  onPick,
}: {
  label: string
  pending?: boolean
  onPick: () => void
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      disabled={pending}
      aria-label={poolCopy.deliverableUploadFile(label)}
      className="justify-start gap-1.5 text-muted-foreground"
      onClick={onPick}
    >
      {pending ? <PhCircleNotch aria-hidden="true" className="size-3.5 animate-spin" /> : <PhUploadSimple aria-hidden="true" className="size-3.5" />}
      {poolCopy.deliverableUploadFile(label)}
    </Button>
  )
}

const deliverableActionClass = buttonVariants({
  variant: 'ghost',
  size: 'icon-sm',
  className: 'text-muted-foreground hover:bg-card hover:text-foreground',
})

const deliverableDisabledActionClass = buttonVariants({
  variant: 'ghost',
  size: 'icon-sm',
  className: 'cursor-not-allowed text-muted-foreground opacity-60',
})

export function DeliverableActions({
  kind,
  label,
  presentation = 'chip',
  triggerLabel,
  className,
  children,
}: {
  kind: DeliverableKind
  label: string
  presentation?: DeliverablePresentation
  triggerLabel?: string
  className?: string
  children: (close: () => void) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const keyboardOpenRef = useRef(false)
  const contentRef = useRef<HTMLDivElement>(null)
  const close = () => {
    keyboardOpenRef.current = false
    setOpen(false)
  }
  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) keyboardOpenRef.current = false
      }}
    >
      <PopoverTrigger
        type="button"
        openOnHover
        delay={180}
        closeDelay={220}
        aria-label={triggerLabel ?? poolCopy.deliverableActions(label)}
        onKeyDown={(event) => {
          if (event.key !== 'Enter' && event.key !== ' ') return
          event.preventDefault()
          keyboardOpenRef.current = true
          if (open) {
            contentRef.current?.querySelector<HTMLElement>('a[href], button:not(:disabled), [tabindex]:not([tabindex="-1"])')?.focus()
            return
          }
          setOpen(true)
        }}
        className={buttonVariants({
          variant: 'ghost',
          className: cn(
            presentation === 'icon'
              ? kind === 'document'
                ? 'size-7 rounded-md p-0 text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/40'
                : kind === 'design'
                  ? 'size-7 rounded-md p-0 text-cyan-600 hover:bg-cyan-50 dark:hover:bg-cyan-950/40'
                  : 'size-7 rounded-md p-0 text-muted-foreground hover:bg-muted'
              : kind === 'document'
                ? 'h-auto min-h-9 min-w-9 flex-col gap-0 rounded-lg border px-1.5 py-1 border-emerald-100 dark:border-emerald-900 bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-100'
                : kind === 'design'
                  ? 'h-auto min-h-9 min-w-9 flex-col gap-0 rounded-lg border px-1.5 py-1 pm-deliverable--design border-cyan-100 text-cyan-700 dark:text-cyan-300 hover:bg-cyan-100'
                  : 'h-auto min-h-9 min-w-9 flex-col gap-0 rounded-lg border px-1.5 py-1 border-border bg-muted text-foreground hover:bg-muted/70',
          ),
        })}
      >
        <DeliverableMark kind={kind} presentation={presentation} />
      </PopoverTrigger>
      <PopoverContent
        ref={contentRef}
        side="top"
        align="center"
        initialFocus={(interactionType) => keyboardOpenRef.current || interactionType === 'touch'}
        className={className ?? 'flex items-center gap-1 rounded-md p-1.5'}
      >
        <PopoverTitle className="sr-only">{poolCopy.deliverableActions(label)}</PopoverTitle>
        <PopoverDescription className="sr-only">{poolCopy.deliverableReady(label)}</PopoverDescription>
        {children(close)}
      </PopoverContent>
    </Popover>
  )
}

import { Input } from '@/shared/ui'
import { isHttpURL } from '../../utils/workflow-artifact-file'

export function DeliverableLinkField({
  pending,
  onSave,
}: {
  pending?: boolean
  onSave: (url: string) => void
}) {
  const [value, setValue] = useState('')
  const trimmed = value.trim()
  const valid = !trimmed || isHttpURL(trimmed)
  return (
    <div className="grid gap-1 px-1 pt-1">
      <Input
        type="url"
        value={value}
        placeholder={poolCopy.deliverableLinkPlaceholder}
        aria-label={poolCopy.deliverableLinkLabel}
        aria-invalid={!valid || undefined}
        className="h-8"
        onChange={(event) => setValue(event.target.value)}
      />
      {!valid ? <p className="m-0 text-xs text-destructive">{poolCopy.deliverableLinkInvalid}</p> : null}
      {trimmed && valid ? (
        <Button type="button" size="sm" variant="outline" disabled={pending} onClick={() => onSave(trimmed)}>
          {poolCopy.deliverableSaveLink}
        </Button>
      ) : null}
    </div>
  )
}

export { deliverableActionClass, deliverableDisabledActionClass }