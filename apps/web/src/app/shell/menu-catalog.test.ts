import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { NATIVE_MODULE_ROUTES } from '@/app/native-modules'

import { HOST_NAVIGATION_CATALOG, MENU_CATALOG } from './menu-catalog'
import { SHELL_NAVIGATION_GROUPS, SHELL_NAVIGATION_ITEMS } from './registry'

describe('menu catalog', () => {
  it('exposes every business menu to the host and keeps system menus PM-only', () => {
    expect(HOST_NAVIGATION_CATALOG.map((item) => item.hostKey)).toEqual([
      'inbox',
      'bi-screen',
      'reporting',
      'projects',
      'requirements',
      'documents',
      'work-items',
      'design-management',
      'quality-management',
      'testing',
      'workflows',
      'ai-runs',
    ])
    expect(MENU_CATALOG.filter((item) => item.group === 'system').every((item) => !item.exposeToHost)).toBe(true)
  })

  it('builds standalone navigation from the same catalog', () => {
    expect(SHELL_NAVIGATION_ITEMS.map((item) => item.path)).toEqual(
      MENU_CATALOG.filter((item) => item.visible).map((item) => item.path),
    )
    expect(SHELL_NAVIGATION_GROUPS.map((group) => group.id)).toEqual([
      'workbench',
      'delivery',
      'assurance',
      'intelligence',
      'system',
    ])
    expect(SHELL_NAVIGATION_ITEMS.map((item) => item.label)).toContain(copy.app.navigation.requirements)
    expect(SHELL_NAVIGATION_ITEMS.map((item) => item.label)).toContain(copy.app.navigation.documents)
    expect(NATIVE_MODULE_ROUTES.every((route) => MENU_CATALOG.some((item) => item.id === route.id && item.path === route.path))).toBe(true)
  })
})
