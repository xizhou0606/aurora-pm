import { InboxWorkspace } from '../components'
import {
  useInboxRuntimeEnvironment,
  useInjectedInboxEnvironment,
  usePMInboxEnvironment,
  type InboxEnvironment,
} from '../components/environment'

export function InboxPage({
  onRuntimeFailure,
}: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const injected = useInjectedInboxEnvironment()
  return injected
    ? <InboxPageEntry environment={injected} onRuntimeFailure={onRuntimeFailure} />
    : <PMInboxPage onRuntimeFailure={onRuntimeFailure} />
}

function PMInboxPage({
  onRuntimeFailure,
}: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const environment = usePMInboxEnvironment()
  return <InboxPageEntry environment={environment} onRuntimeFailure={onRuntimeFailure} />
}

function InboxPageEntry({
  environment: baseEnvironment,
  onRuntimeFailure,
}: {
  environment: InboxEnvironment
  onRuntimeFailure?: (error: unknown) => void
}) {
  const environment = useInboxRuntimeEnvironment(baseEnvironment, onRuntimeFailure)
  return <InboxWorkspace environment={environment} />
}
