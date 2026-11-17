import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import {
  canUseLandingMotion,
  chapterPad,
  chapterProgress,
  landingEvidenceShots,
  landingHeroShot,
  landingScenes,
  landingTeamMarks,
  shotEnterFrom,
  shotLeaveTo,
  shotRest,
  shotRestTravel,
  shouldMountLandingScene,
  shouldPrepareLandingScene,
} from '../utils/story'

describe('landing story', () => {
  it('keeps five live workbench scenes', () => {
    expect(landingScenes.map((scene) => scene.id)).toEqual([
      'inbox',
      'tasks',
      'workflow',
      'command',
      'quality',
    ])
    expect(landingScenes.map((scene) => scene.motion)).toEqual([
      'iris',
      'slide',
      'wipe',
      'dissolve',
      'reveal',
    ])
  })

  it('pads chapter numbers for the stacked story', () => {
    expect(chapterPad(0)).toBe('01')
    expect(chapterPad(4)).toBe('05')
    expect(chapterProgress(0)).toBe(0.2)
    expect(chapterProgress(4)).toBe(1)
  })

  it('moves the product shot farther than the copy', () => {
    expect(shotEnterFrom('slide').xPercent).toBeGreaterThan(40)
    expect(shotEnterFrom('wipe').yPercent).toBeGreaterThan(40)
    expect(shotEnterFrom('reveal').xPercent).toBeLessThan(-40)
    expect(shotEnterFrom('dissolve').scale).toBeGreaterThan(1.2)
    expect(shotLeaveTo('slide').y).toBeLessThan(-50)
    expect(shotRestTravel()).toEqual({ rotate: 0, scale: 1, xPercent: 0, yPercent: 0, y: 0 })
    expect(shotRest()).toEqual({ opacity: 1, xPercent: 0, yPercent: 0 })
  })

  it('keeps the first product shot off the later command chapter', () => {
    expect(landingHeroShot).toBe('workflow')
    expect(landingHeroShot).not.toBe('command')
    expect(landingScenes.map((scene) => scene.id)).toContain(landingHeroShot)
  })

  it('renders html workbench frames instead of screenshots', () => {
    expect(landingScenes.every((scene) => !('image' in scene))).toBe(true)
    expect(landingEvidenceShots.map((shot) => shot.id)).toEqual([
      'requirement-detail',
      'requirement-todo',
    ])
    expect(copy.app.productName).toBe('PM 项目管理')
  })

  it('mounts the previous, current, and next chapters', () => {
    expect(shouldMountLandingScene(0, 0)).toBe(true)
    expect(shouldMountLandingScene(1, 0)).toBe(true)
    expect(shouldMountLandingScene(2, 0)).toBe(false)
    expect(shouldMountLandingScene(3, 3)).toBe(true)
    expect(shouldMountLandingScene(2, 3)).toBe(true)
    expect(shouldMountLandingScene(4, 3)).toBe(true)
    expect(shouldMountLandingScene(1, 3)).toBe(false)
    expect(shouldPrepareLandingScene(2, 3)).toBe(true)
    expect(shouldPrepareLandingScene(4, 3)).toBe(true)
    expect(shouldPrepareLandingScene(1, 3)).toBe(false)
  })

  it('disables scroll theater in jsdom', () => {
    const originalUserAgent = Object.getOwnPropertyDescriptor(navigator, 'userAgent')
    Object.defineProperty(navigator, 'userAgent', { configurable: true, value: 'Mozilla/5.0 jsdom/26.1.0' })
    try {
      expect(canUseLandingMotion()).toBe(false)
    } finally {
      if (originalUserAgent) Object.defineProperty(navigator, 'userAgent', originalUserAgent)
      else Reflect.deleteProperty(navigator, 'userAgent')
    }
  })

  it('opens the hero with a short workflow title', () => {
    expect(copy.landing.heroTitleLines.join('')).toBe(copy.landing.heroTitle)
    expect(copy.landing.heroLeadLines.join('')).toBe(copy.landing.heroLead)
    expect(copy.landing.heroEyebrow).toBe('项目交付工作台')
  })

  it('keeps four product highlights after the story', () => {
    expect(copy.landing.highlights.map((item) => item.id)).toEqual([
      'workflow',
      'ai-in-node',
      'owners',
      'deliverables',
    ])
  })

  it('places live requirement evidence after the highlights', () => {
    expect(landingEvidenceShots.map((shot) => shot.id)).toEqual([
      'requirement-detail',
      'requirement-todo',
    ])
    expect(copy.landing.evidenceShots.map((shot) => shot.id)).toEqual([
      'requirement-detail',
      'requirement-todo',
    ])
    expect(copy.landing.evidenceShots.map((shot) => shot.rail)).toEqual([
      '需求详情',
      '当前节点',
    ])
  })

  it('keeps a looping used-by strip after the story', () => {
    expect(landingTeamMarks).toHaveLength(5)
    expect(copy.landing.usedByTeams).toEqual([
      '极光矩阵测试团队',
      'UI团队',
      '产品团队',
      '交付团队',
      '质量保障团队',
    ])
  })
})
