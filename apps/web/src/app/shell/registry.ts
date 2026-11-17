import {
  type IconComponent,
  PhBookOpenText, PhBookOpenTextFill,
  PhBuildings, PhBuildingsFill,
  PhBriefcase, PhBriefcaseFill,
  PhClipboardText, PhClipboardTextFill,
  PhFileText, PhFileTextFill,
  PhFileMagnifyingGlass,
  PhChartBar, PhChartBarFill,
  PhTray, PhTrayFill,
  PhListChecks, PhListChecksFill,
  PhSlidersHorizontal, PhSlidersHorizontalFill,
  PhShieldCheck, PhShieldCheckFill,
  PhSparkle, PhSparkleFill,
  PhNotebook, PhNotebookFill,
  PhUsers, PhUsersFill,
  PhGitBranch,
  PhFlowArrow, PhFlowArrowFill,
} from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { nativeModuleRoute } from '@/app/native-modules'

import { MENU_CATALOG, MENU_GROUP_ORDER, type MenuCatalogItem, type MenuGroupId, type MenuShellId } from './menu-catalog'

export type ShellNavigationItemId = MenuShellId

// Sidebar icons: Regular 16px, inheriting text colour; the solid Fill weight
// is reserved for the active item (weight-as-state, docs/ICON_SYSTEM.md).
export type ShellNavigationIcon = {
  regular: IconComponent
  fill: IconComponent
}

export type ShellNavigationItem = {
  id: ShellNavigationItemId
  path: string
  label: string
  icon: ShellNavigationIcon
  testId: string
  active: (path: string) => boolean
}

export type ShellNavigationGroup = {
  id: MenuGroupId
  label: string
  items: readonly ShellNavigationItem[]
}

const menuIcons: Record<ShellNavigationItemId, ShellNavigationIcon> = {
  inbox: { regular: PhTray, fill: PhTrayFill },
  biScreen: { regular: PhChartBar, fill: PhChartBarFill },
  projects: { regular: PhBriefcase, fill: PhBriefcaseFill },
  requirements: { regular: PhFileText, fill: PhFileTextFill },
  documents: { regular: PhBookOpenText, fill: PhBookOpenTextFill },
  workItems: { regular: PhListChecks, fill: PhListChecksFill },
  designReviews: { regular: PhFileMagnifyingGlass, fill: PhFileMagnifyingGlass },
  quality: { regular: PhShieldCheck, fill: PhShieldCheckFill },
  reporting: { regular: PhNotebook, fill: PhNotebookFill },
  testing: { regular: PhClipboardText, fill: PhClipboardTextFill },
  workflows: { regular: PhFlowArrow, fill: PhFlowArrowFill },
  'ai-runs': { regular: PhSparkle, fill: PhSparkleFill },
  'workspace-management': { regular: PhBuildings, fill: PhBuildingsFill },
  'access-management': { regular: PhUsers, fill: PhUsersFill },
  'scm-integration': { regular: PhGitBranch, fill: PhGitBranch },
  'host-navigation': { regular: PhSlidersHorizontal, fill: PhSlidersHorizontalFill },
}

const menuLabels: Record<ShellNavigationItemId, string> = {
  inbox: copy.app.navigation.inbox,
  biScreen: copy.app.navigation.biScreen,
  projects: copy.app.navigation.projects,
  requirements: copy.app.navigation.requirements,
  documents: copy.app.navigation.documents,
  workItems: copy.app.navigation.workItems,
  designReviews: copy.app.navigation.designReviews,
  quality: copy.app.navigation.quality,
  reporting: copy.app.navigation.reporting,
  testing: copy.app.navigation.testing,
  workflows: copy.app.navigation.workflows,
  'ai-runs': copy.app.navigation.aiRuns,
  'workspace-management': copy.app.navigation.workspaceManagement,
  'access-management': copy.app.navigation.accessManagement,
  'scm-integration': copy.app.navigation.scmIntegration,
  'host-navigation': copy.app.navigation.hostNavigation,
}

function catalogPath(item: MenuCatalogItem): string {
  if (
    item.id === 'inbox'
    || item.id === 'workItems'
    || item.id === 'biScreen'
    || item.id === 'quality'
    || item.id === 'reporting'
  ) {
    return nativeModuleRoute(item.id).path
  }
  return item.path
}

function toShellItem(item: MenuCatalogItem): ShellNavigationItem {
  const path = catalogPath(item)
  return {
    id: item.id,
    path,
    label: menuLabels[item.id],
    icon: menuIcons[item.id],
    testId: item.testId,
    active: (current) => current === path || (item.matchPrefix === true && current.startsWith(`${path}/`)),
  }
}

export const SHELL_NAVIGATION_GROUPS: readonly ShellNavigationGroup[] = MENU_GROUP_ORDER.map((groupId) => ({
  id: groupId,
  label: copy.app.shell.navigationGroups[groupId],
  items: MENU_CATALOG.filter((item) => item.visible && item.group === groupId).map(toShellItem),
}))

export const SHELL_NAVIGATION_ITEMS: readonly ShellNavigationItem[] =
  SHELL_NAVIGATION_GROUPS.flatMap((group) => group.items)

export function normalizedShellPath(path: string): string {
  return String(path || '').split('?')[0].split('#')[0]
}
