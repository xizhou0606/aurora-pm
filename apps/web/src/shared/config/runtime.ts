import { copy } from '@/shared/copy'

export type PMRuntimeConfig = {
  apiBaseUrl: string
  hostApiBaseUrl: string
  productName: string
  appVersion: string
  versionSummary: readonly string[]
  releaseDate: string
  shortName: string
  logoUrl: string
  primaryColor: string
  loginTitle: string
  loginSubtitle: string
  loginBackgroundColor: string
  loginBackgroundImage: string
  timezone: string
}

export const PM_BUSINESS_MODULE_IDS = [
  'inbox',
  'workItems',
  'biScreen',
  'quality',
  'reporting',
] as const

export type PMBusinessModuleId = (typeof PM_BUSINESS_MODULE_IDS)[number]

declare global {
  interface Window {
    __RUNTIME_CONFIG__?: Record<string, unknown>
  }
}

function runtimeValue(key: string, fallback = ''): string {
  const value = window.__RUNTIME_CONFIG__?.[key]
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

function safeColor(value: string, fallback: string): string {
  return /^(#[0-9a-f]{3,8}|rgb[a]?\([^)]+\)|hsl[a]?\([^)]+\))$/i.test(value)
    ? value
    : fallback
}

function versionSummary(value: string): string[] {
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) {
      return parsed
        .map((item) => (typeof item === 'string' ? item.trim() : ''))
        .filter(Boolean)
    }
  } catch {
    // Fall back to a line-separated deployment value.
  }
  return value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
}

export function getPMRuntimeConfig(): PMRuntimeConfig {
  const productName = runtimeValue(
    'PM_PRODUCT_NAME',
    import.meta.env.VITE_PM_PRODUCT_NAME || copy.app.productName,
  )
  return {
    apiBaseUrl: runtimeValue(
      'PM_API_BASE_URL',
      runtimeValue('VITE_API_BASE_URL', import.meta.env.VITE_PM_API_BASE_URL || ''),
    ).replace(/\/+$/, ''),
    hostApiBaseUrl: runtimeValue(
      'PM_HOST_API_BASE_URL',
      import.meta.env.VITE_PM_HOST_API_BASE_URL || window.location.origin,
    ).replace(/\/+$/, ''),
    productName,
    appVersion: runtimeValue(
      'PM_APP_VERSION',
      import.meta.env.VITE_PM_APP_VERSION || 'dev',
    ),
    versionSummary: versionSummary(runtimeValue(
      'PM_VERSION_SUMMARY',
      import.meta.env.VITE_PM_VERSION_SUMMARY || '[]',
    )),
    releaseDate: runtimeValue(
      'PM_RELEASE_DATE',
      import.meta.env.VITE_PM_RELEASE_DATE || '',
    ),
    shortName: runtimeValue('PM_PRODUCT_SHORT_NAME', copy.app.shortName),
    logoUrl: runtimeValue('PM_LOGO_URL', '/favicon.svg'),
    primaryColor: safeColor(runtimeValue('PM_PRIMARY_COLOR', '#2563eb'), '#2563eb'),
    loginTitle: runtimeValue('PM_LOGIN_TITLE', copy.auth.loginTitle(productName)),
    loginSubtitle: runtimeValue(
      'PM_LOGIN_SUBTITLE',
      copy.auth.loginSubtitle,
    ),
    loginBackgroundColor: safeColor(
      runtimeValue('PM_LOGIN_BACKGROUND_COLOR', '#eef2f7'),
      '#eef2f7',
    ),
    loginBackgroundImage: runtimeValue('PM_LOGIN_BACKGROUND_IMAGE'),
    timezone: runtimeValue(
      'PM_TIMEZONE',
      Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Shanghai',
    ),
  }
}

const BRAND_FAVICON_HREF = '/favicon.svg'

function applyBrandFavicon() {
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.type = 'image/svg+xml'
  link.href = BRAND_FAVICON_HREF
}

export function applyPMBrand(config = getPMRuntimeConfig()) {
  document.title = config.productName
  document.documentElement.style.setProperty('--app-theme-primary', config.primaryColor)
  document.documentElement.style.setProperty('--design-color-primary', config.primaryColor)
  document.documentElement.style.setProperty('--pm-login-background', config.loginBackgroundColor)
  applyBrandFavicon()
}

export function isSafePMWorkspacePath(pathname: string): boolean {
  return (
    pathname === '/workspace/projects' ||
    pathname.startsWith('/workspace/projects/') ||
    pathname === '/workspace/mcp-cli' ||
    pathname === '/workspace/mcp-cli/authorize' ||
    pathname === '/workspace/mcp-cli/device' ||
    pathname === '/workspace/inbox' ||
    /^\/workspace\/work-items(?:\/\d+)?$/.test(pathname) ||
    pathname === '/workspace/bi-screen' ||
    pathname === '/workspace/quality-management' ||
    pathname === '/workspace/quality-management/bugs/new' ||
    /^\/workspace\/quality-management\/bugs\/\d+$/.test(pathname) ||
    pathname === '/workspace/design-management' ||
    pathname === '/workspace/design-management/reviews/new' ||
    /^\/workspace\/design-management\/reviews\/[0-9a-f-]+$/i.test(pathname) ||
    pathname === '/workspace/design-management/mobile-collectors' ||
    pathname === '/workspace/requirements' ||
    /^\/workspace\/requirements\/\d+$/.test(pathname) ||
    pathname === '/workspace/pm/workflows' ||
    /^\/workspace\/pm\/workflows\/\d+$/.test(pathname) ||
    pathname === '/workspace/pm/ai-runs' ||
    pathname === '/workspace/pm/host-navigation' ||
    pathname === '/workspace/testing'
  )
}

export function safePMRedirect(raw: unknown, fallback = '/workspace/projects'): string {
  const value = typeof raw === 'string' ? raw.trim() : ''
  const path = value.startsWith('/') && !value.startsWith('//') && !value.includes('\\')
    ? value
    : fallback
  const url = new URL(path, window.location.origin)
  const pathname = url.pathname
  if (!isSafePMWorkspacePath(pathname)) return fallback

  const sensitiveKeys = /^(access_token|authorization|id_token|jwt|login_code|password|refresh_token|token)$/i
  for (const key of [...url.searchParams.keys()]) {
    if (sensitiveKeys.test(key)) url.searchParams.delete(key)
  }
  if (/(?:access_token|authorization|id_token|jwt|login_code|password|refresh_token|token)=/i.test(url.hash)) {
    url.hash = ''
  }
  return `${url.pathname}${url.search}${url.hash}`
}

export function optionalPMRedirect(raw: unknown, fallback = '/workspace/projects'): string | undefined {
  const redirect = safePMRedirect(raw, fallback)
  return redirect === fallback ? undefined : redirect
}
