import catalog from './catalog.json'

export type MenuGroupId = 'workbench' | 'delivery' | 'assurance' | 'intelligence' | 'system'

export type MenuShellId =
  | 'inbox'
  | 'biScreen'
  | 'projects'
  | 'requirements'
  | 'documents'
  | 'workItems'
  | 'designReviews'
  | 'quality'
  | 'reporting'
  | 'testing'
  | 'workflows'
  | 'ai-runs'
  | 'workspace-management'
  | 'access-management'
  | 'scm-integration'
  | 'host-navigation'

export type MenuCatalogItem = {
  id: MenuShellId
  hostKey: string
  label: string
  path: string
  group: MenuGroupId
  exposeToHost: boolean
  visible: boolean
  hostIcon: string
  sortOrder: number
  testId: string
  matchPrefix?: boolean
}

type RawMenuCatalogItem = {
  id: MenuShellId
  hostKey: string
  label: string
  path: string
  icon: string
  group: MenuGroupId
  exposeToHost: boolean
  visible?: boolean
  sortOrder: number
  testId: string
  matchPrefix?: boolean
}

export const MENU_GROUP_ORDER: readonly MenuGroupId[] = [
  'workbench',
  'delivery',
  'assurance',
  'intelligence',
  'system',
]

export const MENU_CATALOG: readonly MenuCatalogItem[] = (catalog as RawMenuCatalogItem[]).map((item) => ({
  id: item.id,
  hostKey: item.hostKey,
  label: item.label,
  path: item.path,
  group: item.group,
  exposeToHost: item.exposeToHost,
  visible: item.visible !== false,
  hostIcon: item.icon,
  sortOrder: item.sortOrder,
  testId: item.testId,
  matchPrefix: item.matchPrefix,
}))

export const HOST_NAVIGATION_CATALOG = MENU_CATALOG.filter((item) => item.visible && item.exposeToHost)
