import { useEffect, useRef, useState } from 'react'

/**
 * Plays entrance animations on first viewport entry instead of on mount, so
 * panels below the fold still get their reveal when the user scrolls to them.
 * Reduced-motion handling stays in the global CSS (durations collapse to 1ms).
 */
export function useRevealOnScroll<T extends HTMLElement>() {
  const ref = useRef<T>(null)
  const [revealed, setRevealed] = useState(false)
  useEffect(() => {
    const element = ref.current
    if (!element || typeof IntersectionObserver === 'undefined') {
      setRevealed(true)
      return
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting && entry.intersectionRatio > 0)) {
        setRevealed(true)
        observer.disconnect()
      }
    })
    observer.observe(element)
    return () => observer.disconnect()
  }, [])
  return { ref, revealed }
}
