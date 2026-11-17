import { useState } from 'react'
import { PhCaretDown, PhList, PhX } from '@/shared/ui/icons'
import { LoginBrand, LoginFog } from '@/features/auth'
import { copy } from '@/shared/copy'
import { ThemeAppearanceControl, useAppearance } from '@/shared/ui'
import { LandingHero } from '../components/LandingHero'
import { LandingProductShot } from '../components/LandingProductShot'
import { useLandingSmooth } from '../components/LandingSmooth'
import { LandingEvidence } from '../components/LandingEvidence'
import { LandingHighlights } from '../components/LandingHighlights'
import { LandingStory } from '../components/LandingStory'
import { LandingUsedBy } from '../components/LandingUsedBy'
import './landing.css'

export function LandingPage() {
  useLandingSmooth()
  const [menuOpen, setMenuOpen] = useState(false)
  const { resolved: fogTheme } = useAppearance()
  const workspaceHref = '/login'

  return (
    <div
      className="landing-page"
      data-fog-theme={fogTheme}
      data-landing-menu={menuOpen ? 'open' : 'closed'}
    >
      <div className="landing-fog" aria-hidden="true">
        <LoginFog conserve theme={fogTheme} />
        <div data-slot="login-iris" />
        <div data-slot="login-veil" />
        <div data-slot="login-grain" />
      </div>
      <a className="landing-skip" href="#product-story">{copy.landing.skipToStory}</a>
      <header className="landing-top">
        <LoginBrand />
        <div className="landing-top-actions">
          <a className="landing-login" href="/login">{copy.landing.login}</a>
          <ThemeAppearanceControl
            labels={{
              menu: copy.landing.themeMenu,
              light: copy.landing.themeLight,
              dark: copy.landing.themeDark,
              system: copy.landing.themeSystem,
            }}
            triggerClassName="text-(--login-theme-fg) hover:bg-(--login-theme-hover) focus-visible:ring-2 focus-visible:ring-(--login-theme-ring)"
          />
          <button
            aria-controls="landing-nav"
            aria-expanded={menuOpen}
            aria-label={menuOpen ? copy.landing.closeNavigation : copy.landing.openNavigation}
            className="landing-menu-toggle"
            onClick={() => setMenuOpen((open) => !open)}
            type="button"
          >
            {menuOpen ? <PhX /> : <PhList />}
          </button>
        </div>
      </header>
      <nav aria-label={copy.landing.primaryNavigation} className="landing-nav" id="landing-nav">
        <a href="#product-highlights">{copy.landing.navigation.capabilities}</a>
        <a href="#product-story">{copy.landing.navigation.solutions}</a>
        <a href="#product-story">{copy.landing.navigation.security}</a>
        <a href="#product-story">{copy.landing.navigation.documentation}</a>
      </nav>
      <main aria-label={copy.landing.homeLabel}>
        <LandingHero workspaceHref={workspaceHref} />
        <LandingProductShot />
        <a className="landing-scroll-cue" href="#product-story">
          <PhCaretDown />
          {copy.landing.scrollCue}
        </a>
        <LandingStory workspaceHref={workspaceHref} />
        <LandingHighlights />
        <LandingEvidence />
        <LandingUsedBy />
      </main>
      <footer className="landing-footer">{copy.landing.footer}</footer>
    </div>
  )
}
