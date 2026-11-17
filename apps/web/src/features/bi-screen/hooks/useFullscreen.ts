import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type RefObject,
} from 'react'

export function useFullscreen(root: RefObject<HTMLElement | null>) {
  const [fallback, setFallback] = useState(false)
  const [native, setNative] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const active = fallback || native

  useEffect(() => {
    const sync = () => {
      const nextNative = document.fullscreenElement === root.current
      setNative(nextNative)
      if (nextNative) setFallback(false)
    }
    document.addEventListener('fullscreenchange', sync)
    return () => {
      document.removeEventListener('fullscreenchange', sync)
      document.body.classList.remove('overflow-hidden')
      if (document.fullscreenElement === root.current) {
        void document.exitFullscreen().catch(() => undefined)
      }
    }
  }, [root])

  useEffect(() => {
    document.body.classList.toggle('overflow-hidden', fallback)
    return () => document.body.classList.remove('overflow-hidden')
  }, [fallback])

  useEffect(() => {
    if (!fallback) return
    const exitOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      setFallback(false)
      triggerRef.current?.focus()
    }
    document.addEventListener('keydown', exitOnEscape)
    return () => document.removeEventListener('keydown', exitOnEscape)
  }, [fallback])

  const toggle = useCallback(async () => {
    const element = root.current
    if (!element) return
    if (fallback) {
      setFallback(false)
      return
    }
    try {
      if (document.fullscreenElement === element) {
        await document.exitFullscreen()
      } else if (element.requestFullscreen) {
        await element.requestFullscreen()
      } else {
        setFallback(true)
      }
    } catch {
      setFallback(true)
    }
  }, [fallback, root])

  return { active, fallback, toggle, triggerRef }
}
