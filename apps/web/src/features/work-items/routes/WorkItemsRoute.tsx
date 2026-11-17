import type { WorkItemsEnvironment } from '../model/environment'
import { WorkItemsWorkspace } from '../components/Workspace'

export function WorkItemsRoute({
  environment,
  onRuntimeFailure,
}: {
  environment: WorkItemsEnvironment
  onRuntimeFailure?: (error: unknown) => void
}) {
  return (
    <WorkItemsWorkspace
      environment={environment}
      onRuntimeFailure={onRuntimeFailure}
    />
  )
}
