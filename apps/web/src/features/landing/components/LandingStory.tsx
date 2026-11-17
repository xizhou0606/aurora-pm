import { useGSAP } from '@gsap/react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useCallback, useEffect, useRef, useState } from 'react'
import { copy } from '@/shared/copy'
import { scrollLandingY } from './LandingSmooth'
import { LandingCopyPanel } from './LandingSceneView'
import { LandingProductFrame, preloadLandingLive } from './LandingScreen'
import {
  canUseLandingMotion,
  landingScenes,
  shotEnterFrom,
  shotLeaveTo,
  shotRestTravel,
  shouldMountLandingScene,
  shouldPrepareLandingScene,
} from '../utils/story'

gsap.registerPlugin(useGSAP, ScrollTrigger)

function useWideStage() {
  const [wide, setWide] = useState(() => (
    typeof window !== 'undefined'
    && window.matchMedia?.('(min-width: 1024px)')?.matches === true
  ))

  useEffect(() => {
    if (!window.matchMedia) return
    const media = window.matchMedia('(min-width: 1024px)')
    const sync = () => setWide(media.matches)
    sync()
    media.addEventListener('change', sync)
    return () => media.removeEventListener('change', sync)
  }, [])

  return wide
}

function refreshStoryPin() {
  requestAnimationFrame(() => ScrollTrigger.refresh())
}

export function LandingStory({ workspaceHref }: { workspaceHref: string }) {
  const sectionRef = useRef<HTMLElement>(null)
  const triggerRef = useRef<ScrollTrigger | null>(null)
  const labelTimes = useRef<number[]>([])
  const activeIndexRef = useRef(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const [storyReady, setStoryReady] = useState(false)
  const wide = useWideStage()

  const syncActive = useCallback((index: number) => {
    if (activeIndexRef.current === index) return
    activeIndexRef.current = index
    setActiveIndex(index)
  }, [])

  const seek = useCallback((index: number) => {
    landingScenes.forEach((scene, sceneIndex) => {
      if (shouldPrepareLandingScene(sceneIndex, index)) preloadLandingLive(scene.id)
    })
    const trigger = triggerRef.current
    const time = labelTimes.current[index]
    if (!trigger || time === undefined) return
    const duration = trigger.animation?.duration() ?? 1
    scrollLandingY(trigger.start + (time / duration) * (trigger.end - trigger.start))
  }, [])

  useGSAP(() => {
    if (!canUseLandingMotion()) return
    const root = sectionRef.current
    if (!root) return
    const mm = gsap.matchMedia()
    mm.add('(min-width: 1024px)', () => {
      const scenes = [...root.querySelectorAll<HTMLElement>('[data-scene]')]
      const fill = root.querySelector<HTMLElement>('[data-fill]')
      const progress = root.querySelector<HTMLElement>('[data-progress]')
      if (scenes.length !== landingScenes.length) return

      const copyOf = (scene: HTMLElement) => [...scene.querySelectorAll<HTMLElement>('[data-reveal="copy"]')]
      const shotOf = (scene: HTMLElement) => scene.querySelector<HTMLElement>('[data-reveal="image"]')
      const rest = shotRestTravel()

      gsap.set(scenes, { autoAlpha: 0, scale: 1, x: 0, y: 0 })
      gsap.set(scenes[0], { autoAlpha: 1 })
      gsap.set(copyOf(scenes[0]), { autoAlpha: 0, y: 18 })
      scenes.forEach((scene, index) => {
        const shot = shotOf(scene)
        if (!shot) return
        gsap.set(shot, index === 0 ? rest : shotEnterFrom(landingScenes[index].motion))
        if (index > 0) gsap.set(copyOf(scene), { autoAlpha: 0, y: 18 })
      })
      gsap.set(fill, { scaleY: 0, transformOrigin: '50% 0' })

      const timeline = gsap.timeline({
        defaults: { ease: 'power2.inOut' },
        scrollTrigger: {
          anticipatePin: 1,
          end: () => `+=${window.innerHeight * 6.2}`,
          fastScrollEnd: false,
          invalidateOnRefresh: true,
          pin: true,
          pinSpacing: true,
          pinType: 'fixed',
          scrub: 0.4,
          start: 'top top',
          trigger: root,
          onToggle: (self) => {
            if (progress) progress.dataset.visible = self.isActive ? 'true' : 'false'
          },
          onUpdate: (self) => {
            const timeline = self.animation as gsap.core.Timeline | undefined
            const name = timeline?.currentLabel() ?? 'ch0'
            const index = Number(String(name).replace('ch', ''))
            if (Number.isNaN(index)) return
            syncActive(index)
          },
        },
      })
      triggerRef.current = timeline.scrollTrigger ?? null
      labelTimes.current = []

      landingScenes.forEach((scene, index) => {
        timeline.addLabel(`ch${index}`)
        labelTimes.current[index] = timeline.duration()
        if (index > 0) {
          const previousShot = shotOf(scenes[index - 1])
          const nextShot = shotOf(scenes[index])
          timeline.to(scenes[index - 1], { autoAlpha: 0, duration: 0.9 })
          timeline.to(scenes[index], { autoAlpha: 1, duration: 0.9 }, '<')
          if (previousShot) {
            timeline.to(previousShot, { ...shotLeaveTo(landingScenes[index - 1].motion), duration: 1.15, ease: 'power3.in' }, '<')
          }
          if (nextShot) {
            timeline.to(nextShot, { ...rest, duration: 1.15, ease: 'power3.out' }, '<')
          }
        }
        const dwellAt = timeline.duration()
        timeline.to(copyOf(scenes[index]), {
          autoAlpha: 1,
          duration: 0.18,
          ease: 'power2.out',
          stagger: 0.06,
          y: 0,
        })
        timeline.to({}, { duration: 0.6 }, dwellAt)
      })

      timeline.to(fill, { duration: timeline.duration(), ease: 'none', scaleY: 1 }, 0)
      refreshStoryPin()
    })
    return () => mm.revert()
  }, { dependencies: [syncActive, wide], scope: sectionRef })

  useEffect(() => {
    const root = sectionRef.current
    if (!root || typeof IntersectionObserver === 'undefined') return
    const preload = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return
      landingScenes.forEach((scene, index) => {
        if (shouldPrepareLandingScene(index, activeIndexRef.current)) preloadLandingLive(scene.id)
      })
    }, { rootMargin: '480px 0px' })
    const mount = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) setStoryReady(true)
    }, { rootMargin: '160px 0px' })
    preload.observe(root)
    mount.observe(root)
    return () => {
      preload.disconnect()
      mount.disconnect()
    }
  }, [])

  useEffect(() => {
    if (!storyReady) return
    landingScenes.forEach((scene, index) => {
      if (shouldPrepareLandingScene(index, activeIndex)) preloadLandingLive(scene.id)
    })
  }, [activeIndex, storyReady])

  useEffect(() => {
    if (wide) return
    const root = sectionRef.current
    if (!root || typeof IntersectionObserver === 'undefined') return
    const articles = [...root.querySelectorAll<HTMLElement>('[data-mobile-scene]')]
    const observer = new IntersectionObserver((entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((left, right) => right.intersectionRatio - left.intersectionRatio)[0]
      const index = Number(visible?.target.getAttribute('data-mobile-scene'))
      if (Number.isInteger(index)) syncActive(index)
    }, { threshold: [0.35, 0.6] })
    articles.forEach((article) => observer.observe(article))
    return () => observer.disconnect()
  }, [syncActive, wide])

  return (
    <section
      aria-label={copy.landing.storyLabel}
      className="landing-story"
      id="product-story"
      ref={sectionRef}
    >
      <nav
        aria-label={copy.landing.chapterNavigation}
        className="landing-progress"
        data-progress=""
        data-visible="false"
      >
        <div className="landing-chain">
          <span className="landing-chain-line" aria-hidden="true">
            <i data-fill="" />
          </span>
          <div className="landing-chain-stops">
            {copy.landing.chapters.map((chapter, index) => (
              <button
                aria-current={index === activeIndex ? 'step' : undefined}
                data-active={index === activeIndex ? 'true' : 'false'}
                data-stop=""
                key={chapter.id}
                onClick={() => seek(index)}
                type="button"
              >
                <span aria-hidden="true" />
                {chapter.rail}
              </button>
            ))}
          </div>
        </div>
      </nav>
      <div className="landing-stage">
        {wide ? copy.landing.chapters.map((chapter, index) => (
          <article className="landing-scene" data-scene="" key={chapter.id}>
            <span aria-hidden="true" className="landing-scene-edge" />
            <div className="landing-chapter-copy">
              <LandingCopyPanel index={index} workspaceHref={workspaceHref} />
            </div>
            <div className="landing-scene-shot" data-reveal="image">
              <LandingProductFrame
                activeIndex={index}
                mounted={storyReady && shouldMountLandingScene(index, activeIndex)}
              />
            </div>
          </article>
        )) : null}
      </div>
      <div className="landing-mobile-chapters">
        {wide ? null : copy.landing.chapters.map((chapter, index) => (
          <article
            className="landing-mobile-chapter"
            data-mobile-scene={index}
            key={chapter.id}
          >
            <LandingCopyPanel index={index} workspaceHref={workspaceHref} />
            <LandingProductFrame
              activeIndex={index}
              mounted={storyReady && shouldMountLandingScene(index, activeIndex)}
            />
          </article>
        ))}
      </div>
    </section>
  )
}
