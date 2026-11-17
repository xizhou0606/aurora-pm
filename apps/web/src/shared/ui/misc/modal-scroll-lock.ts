import { useEffect } from 'react'

let lockCount = 0
let previousBodyOverflow = ''
let previousDocumentOverflow = ''

function acquireModalScrollLock() {
  if (lockCount === 0) {
    previousBodyOverflow = document.body.style.overflow
    previousDocumentOverflow = document.documentElement.style.overflow
    document.body.style.overflow = 'hidden'
    document.documentElement.style.overflow = 'hidden'
    document.body.dataset.modalScrollLock = 'true'
  }
  lockCount += 1
}

function releaseModalScrollLock() {
  lockCount = Math.max(0, lockCount - 1)
  if (lockCount > 0) return
  document.body.style.overflow = previousBodyOverflow
  document.documentElement.style.overflow = previousDocumentOverflow
  delete document.body.dataset.modalScrollLock
}

export function useModalScrollLock(open: boolean) {
  useEffect(() => {
    if (!open) return
    acquireModalScrollLock()
    return releaseModalScrollLock
  }, [open])
}
