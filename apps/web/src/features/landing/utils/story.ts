export const landingScenes = [
  { id: 'inbox', motion: 'iris' },
  { id: 'tasks', motion: 'slide' },
  { id: 'workflow', motion: 'wipe' },
  { id: 'command', motion: 'dissolve' },
  { id: 'quality', motion: 'reveal' },
] as const

export const landingHeroShot = 'workflow' as const

export type LandingScene = (typeof landingScenes)[number]
export type LandingMotion = LandingScene['motion']
export type LandingTheme = 'light' | 'dark'

export type ShotEnter = {
  opacity: number
  xPercent: number
  yPercent: number
}

const REST: ShotEnter = { opacity: 1, xPercent: 0, yPercent: 0 }

export function canUseLandingMotion() {
  return typeof window !== 'undefined'
    && !/jsdom/i.test(navigator.userAgent)
    && !(window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches)
}

export function chapterPad(index: number) {
  return String(index + 1).padStart(2, '0')
}

export function chapterProgress(index: number, total = landingScenes.length) {
  return (index + 1) / total
}

export type ShotTravel = {
  rotate: number
  scale: number
  xPercent: number
  yPercent: number
  y: number
}

const SHOT_REST: ShotTravel = { rotate: 0, scale: 1, xPercent: 0, yPercent: 0, y: 0 }

export function shotEnterFrom(motion: LandingMotion = 'iris'): ShotTravel {
  if (motion === 'slide') return { rotate: 2.2, scale: 1.16, xPercent: 48, yPercent: 0, y: 36 }
  if (motion === 'wipe') return { rotate: 0, scale: 1.12, xPercent: 0, yPercent: 46, y: 0 }
  if (motion === 'reveal') return { rotate: -2.2, scale: 1.16, xPercent: -48, yPercent: 0, y: 28 }
  if (motion === 'dissolve') return { rotate: 0, scale: 1.28, xPercent: 0, yPercent: 0, y: 18 }
  return { rotate: 0, scale: 1.32, xPercent: 0, yPercent: 0, y: 64 }
}

export function shotLeaveTo(motion: LandingMotion = 'iris'): ShotTravel {
  if (motion === 'slide') return { rotate: -1.6, scale: 0.82, xPercent: -22, yPercent: 0, y: -72 }
  if (motion === 'wipe') return { rotate: 0, scale: 0.84, xPercent: 0, yPercent: -34, y: 0 }
  if (motion === 'reveal') return { rotate: 1.6, scale: 0.82, xPercent: 22, yPercent: 0, y: -64 }
  if (motion === 'dissolve') return { rotate: 0, scale: 0.8, xPercent: 0, yPercent: 0, y: -20 }
  return { rotate: 0, scale: 0.78, xPercent: 0, yPercent: 0, y: -80 }
}

export function shotRestTravel() {
  return SHOT_REST
}

export function shotEnter(_motion: LandingMotion): ShotEnter {
  return { opacity: 0, xPercent: 0, yPercent: 0 }
}

export const landingEvidenceShots = [
  { id: 'requirement-detail' },
  { id: 'requirement-todo' },
] as const

export type LandingEvidenceShot = (typeof landingEvidenceShots)[number]

export const landingTeamMarks = [
  '/landing/team-aurora.png',
  '/landing/team-ui.png',
  '/landing/team-product.png',
  '/landing/team-delivery.png',
  '/landing/team-quality.png',
] as const

export function shotRest() {
  return REST
}

export function shouldMountLandingScene(index: number, activeIndex: number) {
  return Math.abs(index - activeIndex) <= 1
}

export function shouldPrepareLandingScene(index: number, activeIndex: number) {
  return Math.abs(index - activeIndex) <= 1
}
