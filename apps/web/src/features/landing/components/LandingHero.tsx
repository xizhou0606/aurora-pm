import { useGSAP } from '@gsap/react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useRef } from 'react'
import { copy } from '@/shared/copy'
import { BrandMark } from '@/shared/ui'
import { canUseLandingMotion } from '../utils/story'

gsap.registerPlugin(useGSAP, ScrollTrigger)

const trailingPunct = /^(.*?)([，。！？、；：]+)$/

function withHangingPunct(line: string) {
  const match = line.match(trailingPunct)
  if (!match?.[1] || !match[2]) return line
  return <>{match[1]}<span className="landing-hero-punct">{match[2]}</span></>
}

export function LandingHero({ workspaceHref }: { workspaceHref: string }) {
  const heroRef = useRef<HTMLElement>(null)

  useGSAP(() => {
    if (!canUseLandingMotion()) return
    const hero = heroRef.current
    if (!hero) return
    const lines = hero.querySelectorAll('[data-hero-line]')

    gsap.from(lines, {
      autoAlpha: 0,
      duration: 0.7,
      ease: 'power3.out',
      stagger: 0.1,
      y: 16,
    })

    gsap.to(hero, {
      autoAlpha: 0.18,
      ease: 'none',
      scrollTrigger: {
        end: 'bottom top',
        scrub: 0.5,
        start: 'top top',
        trigger: hero,
      },
      y: -72,
    })
  }, { scope: heroRef })

  return (
    <section className="landing-hero" ref={heroRef}>
      <p className="landing-hero-kicker">
        <BrandMark className="landing-hero-mark" size={20} />
        <span>{copy.landing.heroEyebrow}</span>
      </p>
      <h1>
        {copy.landing.heroTitleLines.map((line) => (
          <span data-hero-line="" key={line}>{withHangingPunct(line)}</span>
        ))}
      </h1>
      <p className="landing-hero-lead">
        {copy.landing.heroLeadLines.map((line) => (
          <span key={line}>{withHangingPunct(line)}</span>
        ))}
      </p>
      <div className="landing-hero-actions">
        <a className="landing-primary" href={workspaceHref}>{copy.landing.enterWorkspace}</a>
        <a className="landing-ghost" href="/login">{copy.landing.loginPrompt}</a>
      </div>
    </section>
  )
}
