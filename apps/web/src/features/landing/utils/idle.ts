export function scheduleLandingIdle(work: () => void, timeout = 280) {
  if (typeof window === 'undefined') return () => {}
  if (typeof window.requestIdleCallback === 'function') {
    const id = window.requestIdleCallback(() => work(), { timeout })
    return () => window.cancelIdleCallback(id)
  }
  const id = window.setTimeout(work, 0)
  return () => window.clearTimeout(id)
}
