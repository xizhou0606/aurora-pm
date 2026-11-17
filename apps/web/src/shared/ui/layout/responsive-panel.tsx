import { Dialog } from '@base-ui/react/dialog'
import { Drawer } from '@base-ui/react/drawer'
import { PhX } from '@/shared/ui/icons'
import { useEffect, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react'

import { copy } from '@/shared/copy'
import { useMediaQuery } from '@/shared/hooks'
import { cn } from '@/shared/lib/utils'

import { buttonVariants } from '../primitives/button'
import {
  dialogMotionClassName,
  drawerMotionClassName,
  overlayMotionClassName,
  usePanelMotion,
} from '../misc/dialog-motion'
import { useModalScrollLock } from '../misc/modal-scroll-lock'
import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const DESKTOP_PANEL_QUERY = '(min-width: 768px)'

const backdropClassName =
  `fixed inset-0 z-40 min-h-dvh bg-black/50 ${overlayMotionClassName}`

const closeButtonClassName = cn(
  buttonVariants({ variant: 'ghost', size: 'icon-sm' }),
  '-mr-1 text-muted-foreground hover:bg-muted/70 hover:text-foreground',
)

export interface ResponsivePanelProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  headerActions?: ReactNode
  closeLabel?: string
  pending?: boolean
  height?: 'content' | 'viewport'
  className?: string
  contentClassName?: string
  finalFocus?: RefObject<HTMLElement | null> | (() => HTMLElement | null)
  onOpenChangeComplete?: (open: boolean) => void
  onCloseRequest?: () => boolean | void
}

interface PanelContentProps {
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  headerActions?: ReactNode
  close: ReactNode
  contentClassName?: string
}

function PanelContent({
  title,
  description,
  children,
  footer,
  headerActions,
  close,
  contentClassName,
}: PanelContentProps) {
  return (
    <>
      <header
        data-slot="responsive-panel-header"
        className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-3 border-b border-border px-5 py-4"
      >
        <div className="min-w-0">
          {title}
          {description}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {headerActions}
          {close}
        </div>
      </header>
      <div
        data-slot="responsive-panel-content"
        className={cn('min-h-0 flex-1 overflow-y-auto px-5 py-4', contentClassName)}
      >
        {children}
      </div>
      {footer ? (
        <footer
          data-slot="responsive-panel-footer"
          className="flex shrink-0 flex-col-reverse items-stretch gap-2 border-t border-border px-5 py-3 sm:flex-row sm:items-center sm:justify-end"
        >
          {footer}
        </footer>
      ) : null}
    </>
  )
}

function useLandingPreviewFrame() {
  const hostRef = useRef<HTMLSpanElement>(null)
  const [container, setContainer] = useState<HTMLElement | null>(null)
  useLayoutEffect(() => {
    const frame = hostRef.current?.closest('[data-slot="landing-live-frame"]')
    setContainer(frame instanceof HTMLElement ? frame : null)
  }, [])
  return { hostRef, container }
}

export function ResponsivePanel({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  headerActions,
  closeLabel = copy.ui.close,
  pending = false,
  height = 'content',
  className,
  contentClassName,
  finalFocus,
  onOpenChangeComplete,
  onCloseRequest,
}: ResponsivePanelProps) {
  const { visible, exiting, handleOpenChange, handleOpenChangeComplete } = usePanelMotion(
    open,
    onOpenChange,
    pending,
    onCloseRequest,
  )
  const snapshotRef = useRef({ title, description, children, footer, headerActions, closeLabel })
  if (open && !exiting) {
    snapshotRef.current = { title, description, children, footer, headerActions, closeLabel }
  }
  const view = snapshotRef.current
  const { hostRef, container: landingContainer } = useLandingPreviewFrame()
  useModalScrollLock((open || exiting) && !landingContainer)
  const overlayContainer = useOverlayPortalContainer()
  const portalContainer = landingContainer ?? overlayContainer
  const isDesktop = useMediaQuery(DESKTOP_PANEL_QUERY)
  const preferredMode = isDesktop ? 'dialog' : 'drawer'
  const [panelMode, setPanelMode] = useState<'dialog' | 'drawer'>(() => (
    typeof window !== 'undefined' && window.matchMedia(DESKTOP_PANEL_QUERY).matches ? 'dialog' : 'drawer'
  ))
  const appearedRef = useRef(false)

  useEffect(() => {
    if (visible) appearedRef.current = true
  }, [visible])

  useEffect(() => {
    if (!open) setPanelMode(preferredMode)
  }, [open, preferredMode])

  const handleComplete = (nextOpen: boolean) => {
    if (nextOpen) {
      appearedRef.current = true
      onOpenChangeComplete?.(true)
      return
    }
    if (!appearedRef.current) return
    handleOpenChangeComplete(false)
    onOpenChangeComplete?.(false)
  }

  if (panelMode === 'dialog') {
    return (
      <Dialog.Root open={visible} onOpenChange={handleOpenChange} onOpenChangeComplete={handleComplete}>
        <span ref={hostRef} hidden />
        <Dialog.Portal container={portalContainer ?? undefined}>
          <Dialog.Backdrop
            data-slot="responsive-panel-backdrop"
            className={backdropClassName}
          />
          <Dialog.Viewport
            data-slot="responsive-panel-viewport"
            className="fixed inset-0 z-50 grid place-items-center overflow-hidden overscroll-none p-4 md:p-8"
          >
            <Dialog.Popup
              data-slot="responsive-panel-dialog"
              finalFocus={finalFocus}
              aria-busy={pending || undefined}
              className={cn(
                'isolate flex max-h-[calc(100dvh-4rem)] w-full max-w-[90rem] flex-col overflow-hidden rounded-2xl border border-border bg-card text-foreground shadow-[var(--design-shadow-elevated)] outline-none focus-visible:ring-2 focus-visible:ring-ring',
                dialogMotionClassName,
                height === 'viewport' && 'h-[calc(100dvh-4rem)]',
                className,
              )}
            >
              <PanelContent
                title={
                  <Dialog.Title className="text-base font-semibold leading-6">
                    {view.title}
                  </Dialog.Title>
                }
                description={view.description ? (
                  <Dialog.Description className="mt-1 text-sm text-muted-foreground">
                    {view.description}
                  </Dialog.Description>
                ) : undefined}
                footer={view.footer}
                headerActions={view.headerActions}
                close={
                  <Dialog.Close
                    data-slot="responsive-panel-close"
                    type="button"
                    aria-label={view.closeLabel}
                    title={view.closeLabel}
                    disabled={pending}
                    className={closeButtonClassName}
                  >
                    <PhX aria-hidden="true" />
                  </Dialog.Close>
                }
                contentClassName={contentClassName}
              >
                {view.children}
              </PanelContent>
            </Dialog.Popup>
          </Dialog.Viewport>
        </Dialog.Portal>
      </Dialog.Root>
    )
  }

  return (
    <Drawer.Root
      open={visible}
      onOpenChange={handleOpenChange}
      onOpenChangeComplete={handleComplete}
      swipeDirection="down"
    >
      <span ref={hostRef} hidden />
      <Drawer.Portal container={portalContainer ?? undefined}>
        <Drawer.Backdrop
          data-slot="responsive-panel-backdrop"
          className={backdropClassName}
        />
        <Drawer.Viewport className="pointer-events-none fixed inset-0 z-50 flex items-end justify-center overscroll-contain">
          <Drawer.Popup
            data-slot="responsive-panel-drawer"
            finalFocus={finalFocus}
            aria-busy={pending || undefined}
            className={cn(
              'pointer-events-auto isolate flex max-h-[90dvh] w-full flex-col overflow-hidden rounded-t-2xl border border-b-0 border-border bg-card text-foreground shadow-[var(--design-shadow-dialog)] outline-none focus-visible:ring-2 focus-visible:ring-ring',
              drawerMotionClassName,
              height === 'viewport' && 'h-[90dvh]',
              className,
            )}
          >
            <Drawer.Content className="flex min-h-0 flex-1 flex-col">
              <PanelContent
                title={
                  <Drawer.Title className="text-base font-semibold leading-6">
                    {view.title}
                  </Drawer.Title>
                }
                description={view.description ? (
                  <Drawer.Description className="mt-1 text-sm text-muted-foreground">
                    {view.description}
                  </Drawer.Description>
                ) : undefined}
                footer={view.footer}
                headerActions={view.headerActions}
                close={
                  <Drawer.Close
                    data-slot="responsive-panel-close"
                    type="button"
                    aria-label={view.closeLabel}
                    title={view.closeLabel}
                    disabled={pending}
                    className={closeButtonClassName}
                  >
                    <PhX aria-hidden="true" />
                  </Drawer.Close>
                }
                contentClassName={contentClassName}
              >
                {view.children}
              </PanelContent>
            </Drawer.Content>
          </Drawer.Popup>
        </Drawer.Viewport>
      </Drawer.Portal>
    </Drawer.Root>
  )
}
