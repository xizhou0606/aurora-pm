import { lazy, Suspense } from 'react'

import type { LandingWorkbenchId } from '../screens/workbench'
import { LandingWorkbenchFallback } from '../screens/workbench'
import { landingScenes } from '../utils/story'

const liveLoaders = {
  inbox: () => import('../live/InboxLive').then((m) => ({ default: m.InboxLive })),
  tasks: () => import('../live/TasksLive').then((m) => ({ default: m.TasksLive })),
  workflow: () => import('../live/WorkflowLive').then((m) => ({ default: m.WorkflowLive })),
  command: () => import('../live/CommandLive').then((m) => ({ default: m.CommandLive })),
  quality: () => import('../live/QualityLive').then((m) => ({ default: m.QualityLive })),
  'requirement-detail': () => import('../live/RequirementDetailLive').then((m) => ({ default: m.RequirementDetailLive })),
  'requirement-todo': () => import('../live/RequirementTodoLive').then((m) => ({ default: m.RequirementTodoLive })),
} as const

const liveScenes = {
  inbox: lazy(liveLoaders.inbox),
  tasks: lazy(liveLoaders.tasks),
  workflow: lazy(liveLoaders.workflow),
  command: lazy(liveLoaders.command),
  quality: lazy(liveLoaders.quality),
  'requirement-detail': lazy(liveLoaders['requirement-detail']),
  'requirement-todo': lazy(liveLoaders['requirement-todo']),
} as const

export function preloadLandingLive(scene: LandingWorkbenchId) {
  void liveLoaders[scene]()
}

export function LandingProductFrame({
  activeIndex = 0,
  mounted = true,
}: {
  activeIndex?: number
  mounted?: boolean
}) {
  const scene = landingScenes[activeIndex]?.id ?? 'inbox'
  return <LandingWorkbenchFrame scene={scene} mounted={mounted} />
}

export function LandingWorkbenchFrame({
  mounted = true,
  scene,
}: {
  mounted?: boolean
  scene: LandingWorkbenchId
}) {
  if (!mounted) return <LandingWorkbenchFallback scene={scene} />
  const Live = liveScenes[scene]
  return (
    <Suspense fallback={<LandingWorkbenchFallback scene={scene} />}>
      <Live />
    </Suspense>
  )
}
