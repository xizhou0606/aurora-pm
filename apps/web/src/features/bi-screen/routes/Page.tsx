import { Screen } from '../components'
import {
  EnvironmentProvider,
  useInjectedEnvironment,
  usePMEnvironment,
  type Environment,
} from '../components/environment'

export function BiScreenPage({
  onRuntimeFailure,
}: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const injected = useInjectedEnvironment()
  return injected
    ? <EnvironmentScreen environment={injected} onRuntimeFailure={onRuntimeFailure} />
    : <PMEnvironmentScreen onRuntimeFailure={onRuntimeFailure} />
}

function PMEnvironmentScreen({ onRuntimeFailure }: {
  onRuntimeFailure?: (error: unknown) => void
}) {
  const environment = usePMEnvironment()
  return <EnvironmentScreen environment={environment} onRuntimeFailure={onRuntimeFailure} />
}

function EnvironmentScreen({
  environment,
  onRuntimeFailure,
}: {
  environment: Environment
  onRuntimeFailure?: (error: unknown) => void
}) {
  return (
    <EnvironmentProvider value={environment}>
      <Screen environment={environment} onRuntimeFailure={onRuntimeFailure} />
    </EnvironmentProvider>
  )
}
