import { Detail } from '../components'
import type { QualityEnvironment } from '../components/environment'
import { useQualityDetailWorkflow, type QualityScopeState } from '../hooks'
import { qualityDetailPath } from '../model'
import type { QualityRouteState } from '../model/types'

export function DetailPage({
  environment,
  route,
  scope,
}: {
  environment: QualityEnvironment
  route: QualityRouteState
  scope: QualityScopeState
}) {
  const workflow = useQualityDetailWorkflow(environment, route)
  return (
    <Detail
        route={route}
        sharePath={route.bugId ? qualityDetailPath(route, route.bugId, route.mode) : undefined}
        state={workflow.detail}
        scope={scope}
        users={workflow.users}
        sceneOptions={workflow.sceneOptions}
        sceneOptionMutations={workflow.sceneOptionMutations}
        activities={workflow.activities}
        comments={workflow.comments}
        currentUserId={environment.userId}
        comment={workflow.comment}
        deleteComment={workflow.deleteComment}
        canViewComments={workflow.canViewComments}
        canCreateComment={workflow.canCreateComment}
        canDeleteComment={workflow.canDeleteComment}
        canUpdate={workflow.canUpdate}
        canDelete={workflow.canDelete}
        canUpload={workflow.canUpload}
        lifecycle={workflow.lifecycle.model}
        edit={workflow.edit}
        assign={workflow.assign}
        resolve={workflow.resolve}
        returnBug={workflow.returnBug}
        remove={workflow.remove}
        upload={workflow.upload}
        assignOpen={workflow.overlay.assignOpen}
        resolveOpen={workflow.overlay.resolveOpen}
        returnOpen={workflow.overlay.returnOpen}
        previewUrl={workflow.overlay.previewUrl}
        onAssignOpenChange={workflow.overlay.setAssignOpen}
        onResolveOpenChange={workflow.overlay.setResolveOpen}
        onReturnOpenChange={workflow.overlay.setReturnOpen}
        onPreviewChange={workflow.overlay.setPreviewUrl}
        onLifecycleAction={workflow.lifecycle.run}
        onModeChange={(mode) => void workflow.setMode(mode)}
        onOpenRequirement={(requirementId) => {
          void environment.navigate(`/workspace/requirements/${requirementId}`)
        }}
        onClose={() => void workflow.close()}
    />
  )
}
