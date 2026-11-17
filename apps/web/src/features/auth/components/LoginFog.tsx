import { useEffect, useRef } from 'react'

type FogTheme = 'light' | 'dark'

type VantaEffect = {
  destroy: () => void
  setOptions?: (options: Record<string, unknown>) => void
  renderer?: {
    setPixelRatio: (ratio: number) => void
  }
}

type VantaFogApi = {
  FOG: (options: Record<string, unknown>) => VantaEffect
}

declare global {
  interface Window {
    VANTA?: VantaFogApi
  }
}

// 本地 vendored 资源（public/vendor/）：外网 CDN 不稳定，曾出现 30s+ 挂起，
// 会导致登录页/落地页雾效长时间空白。
const THREE_SRC = '/vendor/three.r134.min.js'
const VANTA_SRC = '/vendor/vanta.fog.min.js'

const fogDrivers = new WeakMap<Element, (zoom: number, speed: number) => void>()

const fogMotion = {
  speed: 2.25,
  zoom: 0.34,
  scale: 1,
  scaleMobile: 1,
} as const

const palettes: Record<FogTheme, Record<string, number | boolean>> = {
  light: {
    highlightColor: 0xfff4f8,
    midtoneColor: 0x5e74d6,
    lowlightColor: 0x2c356e,
    baseColor: 0xb8c2e8,
    blurFactor: 0.34,
    ...fogMotion,
  },
  dark: {
    highlightColor: 0xc9cedd,
    midtoneColor: 0x4f5ab8,
    lowlightColor: 0x1a1f32,
    baseColor: 0x14161c,
    blurFactor: 0.34,
    ...fogMotion,
  },
}

export function driveFogMotion(root: ParentNode, zoom: number, speed: number) {
  const host = root.querySelector('[data-slot="login-fog"]')
  if (host) fogDrivers.get(host)?.(zoom, speed)
}

function loadScript(src: string, marker: string) {
  return new Promise<void>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(
      `script[data-pm-login-fog="${marker}"]`,
    )
    if (existing) {
      if (existing.dataset.ready === '1') {
        resolve()
        return
      }
      existing.addEventListener('load', () => resolve(), { once: true })
      existing.addEventListener('error', () => reject(new Error(src)), { once: true })
      return
    }

    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.dataset.pmLoginFog = marker
    script.onload = () => {
      script.dataset.ready = '1'
      resolve()
    }
    script.onerror = () => reject(new Error(src))
    document.head.appendChild(script)
  })
}

function fogPixelRatio() {
  const dpr = window.devicePixelRatio || 1
  if (window.matchMedia?.('(max-width: 1024px)')?.matches) return 1
  return Math.min(dpr, 1.2)
}

function fogSpeed(theme: FogTheme, scrollLinked: boolean) {
  return scrollLinked ? 3.1 : Number(palettes[theme].speed)
}

export function LoginFog({
  theme,
  scrollLinked = false,
  conserve = false,
}: {
  theme: FogTheme
  scrollLinked?: boolean
  conserve?: boolean
}) {
  const hostRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    if (typeof navigator !== 'undefined' && /jsdom/i.test(navigator.userAgent)) return
    if (window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches) return

    let cancelled = false
    let effect: VantaEffect | undefined

    const syncVisibility = () => {
      effect?.setOptions?.({
        speed: document.hidden ? 0 : fogSpeed(theme, scrollLinked),
      })
    }

    void (async () => {
      try {
        await loadScript(THREE_SRC, 'three')
        await loadScript(VANTA_SRC, 'vanta-fog')
        if (cancelled || !window.VANTA?.FOG) return
        const created = window.VANTA.FOG({
          el: host,
          mouseControls: !scrollLinked && !conserve,
          touchControls: !scrollLinked && !conserve,
          gyroControls: false,
          minHeight: 200,
          minWidth: 200,
          ...palettes[theme],
          ...(scrollLinked ? { speed: 3.1, zoom: 0.28 } : {}),
        })
        if (cancelled) {
          created.destroy()
          return
        }
        effect = created
        if (conserve) {
          created.renderer?.setPixelRatio(fogPixelRatio())
        }
        fogDrivers.set(host, (zoom, speed) => {
          effect?.setOptions?.({ zoom, speed })
        })
        document.addEventListener('visibilitychange', syncVisibility)
        syncVisibility()
      } catch {
        effect = undefined
      }
    })()

    return () => {
      cancelled = true
      document.removeEventListener('visibilitychange', syncVisibility)
      fogDrivers.delete(host)
      effect?.destroy()
    }
  }, [theme, scrollLinked, conserve])

  return <div ref={hostRef} data-slot="login-fog" className="absolute inset-0" aria-hidden="true" />
}
