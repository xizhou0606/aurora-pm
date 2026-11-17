import { useSyncExternalStore } from 'react'

function subscribeToFullscreenChange(onStoreChange: () => void) {
  document.addEventListener('fullscreenchange', onStoreChange)
  return () => document.removeEventListener('fullscreenchange', onStoreChange)
}

function getFullscreenElement() {
  return document.fullscreenElement instanceof HTMLElement
    ? document.fullscreenElement
    : null
}

function getServerFullscreenElement() {
  return null
}

/**
 * Keeps floating UI inside the browser fullscreen top layer.
 * Portals mounted under document.body are not visible while a descendant is fullscreen.
 */
export function useOverlayPortalContainer() {
  return useSyncExternalStore(
    subscribeToFullscreenChange,
    getFullscreenElement,
    getServerFullscreenElement,
  )
}
