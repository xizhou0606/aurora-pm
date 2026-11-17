import { AlertDialog } from '@base-ui/react/alert-dialog'
import { useRef, type ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

import { Button, buttonVariants } from '../primitives/button'
import { dialogMotionClassName, overlayMotionClassName, usePanelMotion } from '../misc/dialog-motion'
import { useModalScrollLock } from '../misc/modal-scroll-lock'
import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const backdropClassName =
  `fixed inset-0 z-50 min-h-dvh bg-foreground/35 ${overlayMotionClassName}`

export interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  title: ReactNode
  description: ReactNode
  cancelLabel: ReactNode
  confirmLabel: ReactNode
  pending?: boolean
  error?: ReactNode
  className?: string
  confirmVariant?: 'default' | 'destructive'
}

export function ConfirmDialog({
  open,
  onOpenChange,
  onConfirm,
  title,
  description,
  cancelLabel,
  confirmLabel,
  pending = false,
  error,
  className,
  confirmVariant = 'destructive',
}: ConfirmDialogProps) {
  const { visible, exiting, handleOpenChange, handleOpenChangeComplete } = usePanelMotion(
    open,
    onOpenChange,
    pending,
  )
  const snapshotRef = useRef({ title, description, cancelLabel, confirmLabel, error })
  if (open && !exiting) {
    snapshotRef.current = { title, description, cancelLabel, confirmLabel, error }
  }
  const view = snapshotRef.current
  useModalScrollLock(open || exiting)
  const portalContainer = useOverlayPortalContainer()
  const cancelButtonRef = useRef<HTMLButtonElement>(null)

  const handleConfirm = () => {
    if (pending) return
    onConfirm()
  }

  return (
    <AlertDialog.Root
      open={visible}
      onOpenChange={handleOpenChange}
      onOpenChangeComplete={handleOpenChangeComplete}
    >
      <AlertDialog.Portal container={portalContainer ?? undefined}>
        <AlertDialog.Backdrop
          data-slot="confirm-dialog-backdrop"
          className={backdropClassName}
        />
        <AlertDialog.Viewport className="fixed inset-0 z-50 grid place-items-center overflow-y-auto overscroll-contain p-4">
          <AlertDialog.Popup
            data-slot="confirm-dialog"
            aria-busy={pending || undefined}
            initialFocus={cancelButtonRef}
            className={cn(
              'w-full max-w-md overflow-hidden rounded-2xl border border-border bg-card text-foreground shadow-[var(--design-shadow-elevated)] outline-none focus-visible:ring-2 focus-visible:ring-ring',
              dialogMotionClassName,
              className,
            )}
          >
            <div className="px-5 py-4">
              <AlertDialog.Title className="text-base font-semibold leading-6">
                {view.title}
              </AlertDialog.Title>
              <AlertDialog.Description className="mt-1 text-sm leading-5 text-muted-foreground">
                {view.description}
              </AlertDialog.Description>
              {view.error ? (
                <div
                  data-slot="confirm-dialog-error"
                  role="alert"
                  className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
                >
                  {view.error}
                </div>
              ) : null}
            </div>
            <div className="flex flex-wrap-reverse items-center justify-end gap-2 border-t border-border bg-muted/30 px-5 py-3">
              <AlertDialog.Close
                ref={cancelButtonRef}
                type="button"
                disabled={pending}
                className={buttonVariants({ variant: 'outline' })}
              >
                {view.cancelLabel}
              </AlertDialog.Close>
              <Button
                type="button"
                variant={confirmVariant}
                disabled={pending}
                onClick={handleConfirm}
              >
                {view.confirmLabel}
              </Button>
            </div>
          </AlertDialog.Popup>
        </AlertDialog.Viewport>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  )
}
