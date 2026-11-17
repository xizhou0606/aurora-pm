import { afterEach, describe, expect, it } from 'bun:test'
import {
  applyPMBrand,
  getPMRuntimeConfig,
  optionalPMRedirect,
  safePMRedirect,
  type PMRuntimeConfig,
} from './runtime'

afterEach(() => {
  window.__RUNTIME_CONFIG__ = {}
  document.title = ''
  document.documentElement.removeAttribute('style')
  document.querySelectorAll('link[rel="icon"]').forEach((node) => node.remove())
})

describe('PM runtime config', () => {
  it('does not expose migration flags in the native-only runtime', () => {
    const config = getPMRuntimeConfig()
    expect(config).not.toHaveProperty('nativeModuleModes')
    expect(config).not.toHaveProperty('externalAppUrl')
    expect(config.hostApiBaseUrl).toBe(
      (import.meta.env.VITE_PM_HOST_API_BASE_URL || window.location.origin).replace(/\/+$/, ''),
    )
    expect(config.logoUrl).toBe('/favicon.svg')
  })

  it('normalizes runtime URL overrides and rejects unsafe color values', () => {
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: ' https://api.example.com/// ',
      PM_HOST_API_BASE_URL: 'https://host.example.com/',
      PM_EXTERNAL_APP_URL: 'https://one2all.example.com///',
      PM_NATIVE_INBOX_MODE: 'iframe',
      PM_NATIVE_BI_SCREEN_MODE: 'iframe',
      PM_NATIVE_WORK_ITEMS_MODE: 'unexpected',
      PM_PRODUCT_NAME: 'Delivery PM',
      PM_APP_VERSION: '2.3.4',
      PM_VERSION_SUMMARY: '["新增版本摘要","修复登录问题"]',
      PM_RELEASE_DATE: '2026-09-01',
      PM_PRODUCT_SHORT_NAME: 'DPM',
      PM_PRIMARY_COLOR: 'url(javascript:alert(1))',
      PM_LOGIN_BACKGROUND_COLOR: 'rgb(1, 2, 3)',
      PM_TIMEZONE: 'Asia/Shanghai',
      PM_FEISHU_LOGIN_ENABLED: 'true',
    }

    expect(getPMRuntimeConfig()).toMatchObject({
      apiBaseUrl: 'https://api.example.com',
      hostApiBaseUrl: 'https://host.example.com',
      productName: 'Delivery PM',
      appVersion: '2.3.4',
      versionSummary: ['新增版本摘要', '修复登录问题'],
      releaseDate: '2026-09-01',
      shortName: 'DPM',
      primaryColor: '#2563eb',
      loginBackgroundColor: 'rgb(1, 2, 3)',
      timezone: 'Asia/Shanghai',
    })
    expect(getPMRuntimeConfig()).not.toHaveProperty('feishuLoginEnabled')
  })

  it('applies only the documented title and brand variables', () => {
    const config = {
      productName: 'Delivery PM',
      primaryColor: '#123456',
      loginBackgroundColor: '#f1f2f3',
    } as PMRuntimeConfig

    applyPMBrand(config)

    expect(document.title).toBe('Delivery PM')
    expect(document.documentElement.style.getPropertyValue('--app-theme-primary')).toBe('#123456')
    expect(document.documentElement.style.getPropertyValue('--design-color-primary')).toBe('#123456')
    expect(document.documentElement.style.getPropertyValue('--pm-login-background')).toBe('#f1f2f3')
    const favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(favicon?.href).toMatch(/\/favicon\.svg$/)
    expect(favicon?.type).toBe('image/svg+xml')
  })

  it('keeps redirects inside PM and standalone workspace routes', () => {
    expect(safePMRedirect('/workspace/projects/9?tab=members')).toBe(
      '/workspace/projects/9?tab=members',
    )
    expect(safePMRedirect('/workspace/work-items?view=list')).toBe(
      '/workspace/work-items?view=list',
    )
    expect(safePMRedirect('/workspace/work-items/31?from=inbox')).toBe(
      '/workspace/work-items/31?from=inbox',
    )
    expect(safePMRedirect('/workspace/quality-management/bugs/new')).toBe(
      '/workspace/quality-management/bugs/new',
    )
    expect(safePMRedirect('/workspace/quality-management/bugs/8?mode=edit')).toBe(
      '/workspace/quality-management/bugs/8?mode=edit',
    )
    expect(safePMRedirect('/workspace/requirements/8')).toBe('/workspace/requirements/8')
    expect(safePMRedirect('/workspace/design-management?status=completed')).toBe(
      '/workspace/design-management?status=completed',
    )
    expect(safePMRedirect('/workspace/design-management/reviews/new?requirement_id=8')).toBe(
      '/workspace/design-management/reviews/new?requirement_id=8',
    )
    expect(safePMRedirect('/workspace/design-management/reviews/22222222-2222-4222-8222-222222222222')).toBe(
      '/workspace/design-management/reviews/22222222-2222-4222-8222-222222222222',
    )
    expect(safePMRedirect('/workspace/design-management/mobile-collectors')).toBe(
      '/workspace/design-management/mobile-collectors',
    )
    expect(safePMRedirect('/workspace/pm/workflows/3')).toBe('/workspace/pm/workflows/3')
    expect(safePMRedirect('/workspace/pm/ai-runs')).toBe('/workspace/pm/ai-runs')
    expect(safePMRedirect('/workspace/mcp-cli')).toBe('/workspace/mcp-cli')
    expect(safePMRedirect('/workspace/mcp-cli/authorize?request_id=req_123')).toBe(
      '/workspace/mcp-cli/authorize?request_id=req_123',
    )
    expect(safePMRedirect('/workspace/mcp-cli/device?user_code=ABCD-EFGH')).toBe(
      '/workspace/mcp-cli/device?user_code=ABCD-EFGH',
    )
    expect(safePMRedirect(
      '/workspace/mcp-cli/authorize?response_type=code&client_id=pm-cli&redirect_uri=http%3A%2F%2F127.0.0.1%3A43123%2Fcallback&code_challenge=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&code_challenge_method=S256&state=opaque&resource=http%3A%2F%2Flocalhost%3A8081%2Fapi%2Fv1%2Ftool-runtime&scope=pm%3Atools+offline_access&channel=cli',
    )).toContain('code_challenge_method=S256')
    expect(safePMRedirect('https://evil.example/workspace/projects')).toBe('/workspace/projects')
    expect(safePMRedirect('//evil.example/workspace/projects')).toBe('/workspace/projects')
    expect(safePMRedirect('/workspace/projects/../../admin')).toBe('/workspace/projects')
    expect(safePMRedirect('/workspace/projects/%2e%2e/%2e%2e/admin')).toBe('/workspace/projects')
    expect(safePMRedirect('/workspace/projects\\..\\admin')).toBe('/workspace/projects')
    expect(safePMRedirect('/admin')).toBe('/workspace/projects')
    expect(optionalPMRedirect('/workspace/projects')).toBeUndefined()
    expect(optionalPMRedirect('/admin')).toBeUndefined()
    expect(optionalPMRedirect('/workspace/projects/9?tab=members')).toBe(
      '/workspace/projects/9?tab=members',
    )
  })

  it('removes credential-like values from otherwise valid redirects', () => {
    expect(
      safePMRedirect(
        '/workspace/projects/9?tab=members&token=secret&access_token=secret-2',
      ),
    ).toBe('/workspace/projects/9?tab=members')
    expect(
      safePMRedirect('/workspace/work-items?view=list#jwt=secret'),
    ).toBe('/workspace/work-items?view=list')
    expect(
      safePMRedirect('/workspace/projects/9?PASSWORD=secret&from_item=31'),
    ).toBe('/workspace/projects/9?from_item=31')
    expect(
      safePMRedirect('/workspace/projects/9?login_code=one-time-code&from_item=31'),
    ).toBe('/workspace/projects/9?from_item=31')
  })
})
