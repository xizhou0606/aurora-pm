import { useMemo } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { Button, EmptyState, OperationalPage } from '@/shared/ui'
import {
  useInjectedQualityEnvironment,
  usePMQualityEnvironment,
  type QualityEnvironment,
} from '../components/environment'
import { useQualityOperationAccess, useQualityScopeWorkflow } from '../hooks'
import { qualityListPath } from '../model/route'
import type { QualityRouteState } from '../model/types'
import { CreatePage } from './CreatePage'
import { DetailPage } from './DetailPage'
import { ListPage } from './ListPage'

export function QualityManagementPage() {
  const injected = useInjectedQualityEnvironment()
  return injected
    ? <QualityContent environment={injected} />
    : <PMQualityPage />
}

function PMQualityPage() {
  const environment = usePMQualityEnvironment()
  return <QualityContent environment={environment} />
}

function QualityContent({ environment }: { environment: QualityEnvironment }) {
  const access = useQualityOperationAccess()
  const canEnter = access.canViewBugs || access.canCreateBug || access.canViewDashboard
  const accessEnvironment = useMemo<QualityEnvironment>(() => ({
    ...environment,
    allowed: environment.allowed && access.status === 'ready' && canEnter,
  }), [access.status, canEnter, environment])
  const { route, scope, queryEnvironment, clearInvalidScope } = useQualityScopeWorkflow(accessEnvironment)
  const listPath = qualityListPath(route)
  const listRoute = useMemo<QualityRouteState>(() => ({
    ...route,
    kind: 'list',
    bugId: undefined,
    mode: 'view',
    sourceExecutionId: undefined,
    sourceTestId: undefined,
    actualResult: undefined,
  }), [listPath])
  const listEnvironment = useMemo<QualityEnvironment>(() => ({
    client: queryEnvironment.client,
    workspaceId: queryEnvironment.workspaceId,
    userId: queryEnvironment.userId,
    sessionRevision: queryEnvironment.sessionRevision,
    pathname: '/workspace/quality-management',
    search: {},
    allowed: queryEnvironment.allowed,
    navigate: queryEnvironment.navigate,
  }), [
    queryEnvironment.allowed,
    queryEnvironment.client,
    queryEnvironment.navigate,
    queryEnvironment.sessionRevision,
    queryEnvironment.userId,
    queryEnvironment.workspaceId,
  ])
  const listScope = useMemo(() => scope, [
    scope.status,
    scope.status === 'ready' ? scope.data : undefined,
    scope.status === 'unavailable' ? scope.reason : undefined,
    scope.status === 'error' ? scope.retry : undefined,
  ])

  if (!environment.allowed || access.status !== 'ready' || !canEnter) {
    return (
      <OperationalPage title={QUALITY_COPY.page.title}>
        <EmptyState
          role="alert"
          title={QUALITY_COPY.page.unavailableTitle}
        />
      </OperationalPage>
    )
  }

  if (scope.status === 'unavailable') {
    return (
      <OperationalPage title={QUALITY_COPY.page.title}>
        <EmptyState
          role="alert"
          title={QUALITY_COPY.page.workspaceUnavailableTitle}
        />
      </OperationalPage>
    )
  }

  if (route.invalidScope) {
    return (
      <OperationalPage title={QUALITY_COPY.page.title}>
        <EmptyState
          role="alert"
          title={QUALITY_COPY.page.invalidScopeTitle}
          action={(
            <Button type="button" variant="outline" onClick={() => void clearInvalidScope()}>
              {QUALITY_COPY.page.resetInvalidScope}
            </Button>
          )}
        />
      </OperationalPage>
    )
  }

  return (
    <>
      <ListPage environment={listEnvironment} route={listRoute} scope={listScope} />
      {route.kind === 'create' ? (
        <CreatePage environment={queryEnvironment} route={route} scope={scope} />
      ) : route.kind === 'detail' ? (
        <DetailPage environment={queryEnvironment} route={route} scope={scope} />
      ) : null}
    </>
  )
}
