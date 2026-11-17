import { useGSAP } from '@gsap/react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useEffect, useRef, useState } from 'react'
import { scheduleLandingIdle } from '../utils/idle'
import { canUseLandingMotion, landingHeroShot } from '../utils/story'
import { LandingWorkbenchFrame, preloadLandingLive } from './LandingScreen'

gsap.registerPlugin(useGSAP, ScrollTrigger)

export function LandingProductShot() {
  const shotRef = useRef<HTMLDivElement>(null)
  const [inView, setInView] = useState(false)
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    const target = shotRef.current
    if (!target || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => {
      setInView(entries.some((entry) => entry.isIntersecting))
    }, { rootMargin: '240px 0px' })
    observer.observe(target)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!inView) {
      setMounted(false)
      return
    }
    preloadLandingLive(landingHeroShot)
    return scheduleLandingIdle(() => setMounted(true))
  }, [inView])

  useGSAP(() => {
    const target = shotRef.current
    if (!target || !canUseLandingMotion()) return
    gsap.fromTo(target, { y: 28 }, {
      ease: 'none',
      scrollTrigger: {
        end: 'bottom top',
        invalidateOnRefresh: true,
        scrub: 0.8,
        start: 'top 88%',
        trigger: target,
      },
      y: -36,
    })
  }, { scope: shotRef })

  return (
    <div className="landing-product-shot" ref={shotRef}>
      <LandingWorkbenchFrame scene={landingHeroShot} mounted={mounted} />
    </div>
  )
}
