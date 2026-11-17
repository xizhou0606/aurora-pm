import { useEffect, useRef, useState } from 'react'

export const overlayMotionClassName = 'pm-overlay-motion'
export const dialogMotionClassName = 'pm-dialog-motion'
export const drawerMotionClassName = 'pm-drawer-motion'

export function useAppearOpen(open: boolean) {
  const [entered, setEntered] = useState(!open)

  useEffect(() => {
    if (!open) {
      setEntered(true)
      return
    }
    if (entered) return
    let inner = 0
    const outer = requestAnimationFrame(() => {
      inner = requestAnimationFrame(() => setEntered(true))
    })
    return () => {
      cancelAnimationFrame(outer)
      cancelAnimationFrame(inner)
    }
  }, [open, entered])

  return open && entered
}

export function usePanelMotion(
  open: boolean,
  onOpenChange: (open: boolean) => void,
  pending = false,
  onCloseRequest?: () => boolean | void,
) {
  const [exiting, setExiting] = useState(false)
  const exitingRef = useRef(false)
  const onOpenChangeRef = useRef(onOpenChange)
  const onCloseRequestRef = useRef(onCloseRequest)
  onOpenChangeRef.current = onOpenChange
  onCloseRequestRef.current = onCloseRequest

  useEffect(() => {
    if (open) return
    exitingRef.current = false
    setExiting(false)
  }, [open])

  const visible = useAppearOpen(open && !exiting)

  const handleOpenChange = (nextOpen: boolean) => {
    if (pending && !nextOpen) return
    if (nextOpen) {
      exitingRef.current = false
      setExiting(false)
      onOpenChangeRef.current(true)
      return
    }
    if (exitingRef.current) return
    if (onCloseRequestRef.current?.() === false) return
    exitingRef.current = true
    setExiting(true)
  }

  const handleOpenChangeComplete = (nextOpen: boolean) => {
    if (nextOpen || !exitingRef.current) return
    onOpenChangeRef.current(false)
  }

  return { visible, exiting, handleOpenChange, handleOpenChangeComplete }
}
