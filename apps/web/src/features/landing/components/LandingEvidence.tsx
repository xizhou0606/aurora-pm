import { useEffect, useRef, useState } from 'react'
import { copy } from '@/shared/copy'
import { LandingWorkbenchFrame, preloadLandingLive } from './LandingScreen'
import type { LandingWorkbenchId } from '../screens/workbench'
import { landingEvidenceShots } from '../utils/story'

function LandingEvidenceFrame({ scene }: { scene: LandingWorkbenchId }) {
  const frameRef = useRef<HTMLElement>(null)
  const [near, setNear] = useState(false)

  useEffect(() => {
    const target = frameRef.current
    if (!target || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => {
      const visible = entries.some((entry) => entry.isIntersecting)
      setNear(visible)
      if (visible) preloadLandingLive(scene)
    }, { rootMargin: '160px 0px', threshold: 0.08 })
    observer.observe(target)
    return () => observer.disconnect()
  }, [scene])

  return (
    <figure className="landing-evidence-frame" ref={frameRef}>
      <LandingWorkbenchFrame mounted={near} scene={scene} />
    </figure>
  )
}

export function LandingEvidence() {
  return (
    <section
      aria-label={copy.landing.evidenceLabel}
      className="landing-evidence"
      id="product-evidence"
    >
      <ul className="landing-evidence-rows">
        {copy.landing.evidenceShots.map((item, index) => {
          const shot = landingEvidenceShots.find((entry) => entry.id === item.id)
          if (!shot) return null
          return (
            <li
              className="landing-evidence-row"
              data-side={index % 2 === 0 ? 'copy-left' : 'shot-left'}
              key={item.id}
            >
              <div className="landing-evidence-copy">
                <p className="landing-eyebrow">{item.rail}</p>
                <h2>{item.title}</h2>
                <p className="landing-copy-body">{item.body}</p>
              </div>
              <LandingEvidenceFrame scene={shot.id} />
            </li>
          )
        })}
      </ul>
    </section>
  )
}
