import {
  useInjectedWorkItemsEnvironment,
  usePMWorkItemsEnvironment,
} from '../hooks/useEnvironment'
import { WorkItemsRoute } from './WorkItemsRoute'

export function WorkItemsPage({
  onRuntimeFailure,
}: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const injected = useInjectedWorkItemsEnvironment()
  return injected
    ? <WorkItemsRoute environment={injected} onRuntimeFailure={onRuntimeFailure} />
    : <PMWorkItemsPage onRuntimeFailure={onRuntimeFailure} />
}

function PMWorkItemsPage({
  onRuntimeFailure,
}: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const environment = usePMWorkItemsEnvironment()
  return <WorkItemsRoute environment={environment} onRuntimeFailure={onRuntimeFailure} />
}
