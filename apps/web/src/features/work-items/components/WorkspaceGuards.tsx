import { WORK_ITEMS_COPY } from '@/shared/copy'
import { AlertBanner, Button, EmptyState, LoadingSkeleton, OperationalPage } from '@/shared/ui'

export function WorkspacePermissionLoading() {
  return (
    <OperationalPage title={WORK_ITEMS_COPY.page.title}>
      <LoadingSkeleton rows={6} label={WORK_ITEMS_COPY.page.permissionLoading} />
    </OperationalPage>
  )
}

export function WorkspacePermissionError({ onRetry }: { onRetry: () => void }) {
  return (
    <OperationalPage title={WORK_ITEMS_COPY.page.title}>
      <div className="flex flex-col items-center gap-3">
        <EmptyState
          role="alert"
          title={WORK_ITEMS_COPY.page.permissionErrorTitle}
          action={(
            <Button type="button" variant="outline" onClick={onRetry}>
              {WORK_ITEMS_COPY.page.retryPermissions}
            </Button>
          )}
        />
        <AlertBanner tone="error" className="w-full max-w-md">
          {WORK_ITEMS_COPY.page.permissionErrorDescription}
        </AlertBanner>
      </div>
    </OperationalPage>
  )
}

export function WorkspaceNotAllowed({ workspaceId }: { workspaceId: number }) {
  return (
    <OperationalPage title={WORK_ITEMS_COPY.page.title}>
      <div className="flex flex-col items-center gap-3">
        <EmptyState
          role="alert"
          title={WORK_ITEMS_COPY.page.unavailableTitle}
        />
        <AlertBanner tone="error" className="w-full max-w-md">
          {WORK_ITEMS_COPY.page.unavailableDescription}
        </AlertBanner>
      </div>
    </OperationalPage>
  )
}

export function WorkspaceInvalidScope({ onClearLegacyScope }: { onClearLegacyScope: () => void }) {
  return (
    <OperationalPage title={WORK_ITEMS_COPY.page.title}>
      <div className="flex flex-col items-center gap-3">
        <EmptyState
          role="alert"
          title={WORK_ITEMS_COPY.page.invalidScopeTitle}
          action={(
            <Button type="button" variant="outline" onClick={onClearLegacyScope}>
              {WORK_ITEMS_COPY.page.resetInvalidScope}
            </Button>
          )}
        />
        <AlertBanner tone="warning" className="w-full max-w-md">
          {WORK_ITEMS_COPY.page.invalidScopeDescription}
        </AlertBanner>
      </div>
    </OperationalPage>
  )
}
