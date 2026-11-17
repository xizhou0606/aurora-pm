import { useGSAP } from '@gsap/react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import Lenis from 'lenis'
import 'lenis/dist/lenis.css'
import { canUseLandingMotion } from '../utils/story'

gsap.registerPlugin(useGSAP, ScrollTrigger)

let landingLenis: Lenis | null = null

export function scrollLandingY(top: number) {
  if (landingLenis) {
    landingLenis.scrollTo(top, { lerp: 0.07 })
    return
  }
  window.scrollTo({ behavior: 'smooth', top })
}

export function scrollLandingTo(target: HTMLElement) {
  if (landingLenis) {
    landingLenis.scrollTo(target, { lerp: 0.07 })
    return
  }
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

export function useLandingSmooth() {
  useGSAP(() => {
    if (!canUseLandingMotion()) return

    const lenis = new Lenis({
      autoRaf: false,
      lerp: 0.075,
      syncTouch: true,
      touchMultiplier: 1.1,
      wheelMultiplier: 1,
    })
    landingLenis = lenis
    lenis.on('scroll', ScrollTrigger.update)
    ScrollTrigger.config({ ignoreMobileResize: true })

    const tick = (time: number) => {
      if (document.hidden) return
      lenis.raf(time * 1000)
    }
    gsap.ticker.add(tick)
    gsap.ticker.lagSmoothing(0)

    return () => {
      gsap.ticker.remove(tick)
      gsap.ticker.lagSmoothing(500)
      landingLenis = null
      lenis.destroy()
    }
  })
}
