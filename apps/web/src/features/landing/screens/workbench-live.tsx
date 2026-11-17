import { useEffect, useMemo, type ReactNode } from 'react'

import { Standalone } from '@/app/shell/Standalone'
import { seedActionPermissions } from '@/features/access-control'
import { copy } from '@/shared/copy'
import { PM_ACTION_CODES } from '@/generated/access-catalog'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import {
  LANDING_USER_LABEL,
  LANDING_WORKSPACE_ID,
  LANDING_WORKSPACE_LABEL,
} from '../live/fixtures'
import {
  LANDING_REQUIREMENT_ID,
  LANDING_TEMPLATE_ID,
} from '../live/ai-pm-client'
import type { LandingWorkbenchId } from './workbench'
import { LandingWorkbenchChrome } from './workbench'

// 落地预览的数据是冻结 fixture，但权限门禁读取真实登录态：未登录访客的
// action-permissions 请求必然 401，会把质量管理等入口 fail-close 成
// "当前模块不可用"。预览壳统一注入全量授权（与测试基座同语义），仅影响
// 本标签页的 fixture 预览，不产生任何服务端信任。
function usePreviewActionPermissions() {
  const revision = useSessionStore((state) => state.revision)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  useMemo(() => seedActionPermissions(PM_ACTION_CODES), [])
  useEffect(() => {
    seedActionPermissions(PM_ACTION_CODES)
  }, [revision, workspaceId])
}

const scenePath: Record<LandingWorkbenchId, string> = {
  inbox: '/workspace/inbox',
  tasks: '/workspace/work-items',
  workflow: `/workspace/pm/workflows/${LANDING_TEMPLATE_ID}`,
  command: '/workspace/bi-screen',
  quality: '/workspace/quality-management',
  'requirement-detail': `/workspace/requirements/${LANDING_REQUIREMENT_ID}`,
  'requirement-todo': `/workspace/requirements/${LANDING_REQUIREMENT_ID}`,
}

function ignoreNavigation() {}
function ignoreWorkspaceChange() {}
function ignoreLogout() {}

export function LandingWorkbench({
  scene,
  children,
}: {
  scene: LandingWorkbenchId
  children: ReactNode
}) {
  usePreviewActionPermissions()
  return (
    <LandingWorkbenchChrome>
      <Standalone
        className="h-full min-h-0"
        currentPath={scenePath[scene]}
        mcpCliHref="/workspace/mcp-cli"
        onLogout={ignoreLogout}
        onProfile={ignoreNavigation}
        onNavigate={ignoreNavigation}
        onWorkspaceChange={ignoreWorkspaceChange}
        productName={copy.app.productName}
        shortName={copy.app.shortName}
        userLabel={LANDING_USER_LABEL}
        workspaceDisabled
        workspaceId={LANDING_WORKSPACE_ID}
        workspaceOptions={[{ id: LANDING_WORKSPACE_ID, label: LANDING_WORKSPACE_LABEL }]}
      >
        {children}
      </Standalone>
    </LandingWorkbenchChrome>
  )
}
