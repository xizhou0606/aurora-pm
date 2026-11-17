import {
  PhFileText,
  PhFlowArrow,
  PhSparkle,
  PhUserCircleCheck,
} from '@/shared/ui/icons'
import { copy } from '@/shared/copy'

const highlightIcons = {
  workflow: PhFlowArrow,
  'ai-in-node': PhSparkle,
  owners: PhUserCircleCheck,
  deliverables: PhFileText,
} as const

export function LandingHighlights() {
  return (
    <section
      aria-label={copy.landing.highlightsLabel}
      className="landing-highlights"
      id="product-highlights"
    >
      <div className="landing-highlights-intro">
        <p className="landing-eyebrow">{copy.landing.highlightsLabel}</p>
        <h2>{copy.landing.highlightsTitle}</h2>
        <p className="landing-copy-body">{copy.landing.highlightsLead}</p>
      </div>
      <ul className="landing-highlights-grid">
        {copy.landing.highlights.map((highlight) => {
          const Icon = highlightIcons[highlight.id as keyof typeof highlightIcons]
          return (
            <li className="landing-highlight" key={highlight.id}>
              <Icon aria-hidden="true" className="landing-highlight-icon" />
              <h3>{highlight.title}</h3>
              <p>{highlight.body}</p>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
