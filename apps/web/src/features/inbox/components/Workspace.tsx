import type { ReactNode } from 'react'

import { INBOX_COPY } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton, OperationalPage, ScopeCascader } from '@/shared/ui'
import type { InboxEnvironment } from '../components/environment'
import {
  INBOX_PAGE_SIZE,
  useInboxScreenModel,
} from '../hooks'
import { InboxScreen } from './Screen'

function Boundary({ children }: { children: ReactNode }) {
  return <div className="box-border min-w-0 p-3 min-[721px]:p-4">{children}</div>
}

export function InboxWorkspace({ environment }: { environment: InboxEnvironment }) {
  const model = useInboxScreenModel(environment)

  if (model.access.status === 'permission_loading') {
    return (
      <OperationalPage title={INBOX_COPY.page.title}>
        <Boundary>
          <LoadingSkeleton rows={6} label={INBOX_COPY.summary.loading} />
        </Boundary>
      </OperationalPage>
    )
  }

  if (model.access.status === 'permission_error') {
    return (
      <OperationalPage title={INBOX_COPY.page.title}>
        <Boundary>
          <EmptyState
            role="alert"
            title={INBOX_COPY.page.unavailableTitle}
            action={<Button type="button" variant="outline" onClick={model.access.retry}>{INBOX_COPY.actions.retry}</Button>}
          />
        </Boundary>
      </OperationalPage>
    )
  }

  if (model.access.status === 'denied') {
    return (
      <OperationalPage title={INBOX_COPY.page.title}>
        <Boundary>
          <EmptyState
            role="alert"
            title={INBOX_COPY.page.unavailableTitle}
          />
        </Boundary>
      </OperationalPage>
    )
  }

  if (model.access.status === 'invalid_scope') {
    return (
      <OperationalPage
        title={INBOX_COPY.page.title}
        context={(
          <ScopeCascader
            state={{ status: 'ready' }}
            label={INBOX_COPY.scope.label}
            summary={INBOX_COPY.scope.invalid}
          >
            <span />
          </ScopeCascader>
        )}
      >
        <Boundary>
          <EmptyState
            role="alert"
            title={INBOX_COPY.page.invalidScopeTitle}
            action={(
              <Button type="button" variant="outline" onClick={model.clearInvalidScope}>
                {INBOX_COPY.page.resetInvalidScope}
              </Button>
            )}
          />
        </Boundary>
      </OperationalPage>
    )
  }

  return (
    <InboxScreen
      refresh={model.refresh}
      canReadBugs={model.canReadBugs}
      scope={{ state: model.scope, value: model.route, onChange: model.setScope }}
      filter={{
        draft: model.search.draft,
        total: model.metrics.total,
        onDraftChange: model.search.setDraft,
        onSubmit: model.search.submit,
      }}
      workflow={{
        state: model.workflow,
        page: model.route.workflowPage,
        pageSize: INBOX_PAGE_SIZE,
        attentionCount: model.metrics.workflowAttention,
        onPageChange: (page) => model.setFeedPage('workflow', page),
        onOpen: model.openWorkflow,
      }}
      task={{
        state: model.task,
        page: model.route.taskPage,
        pageSize: INBOX_PAGE_SIZE,
        attentionCount: model.metrics.taskAttention,
        onPageChange: (page) => model.setFeedPage('task', page),
        onOpen: model.openTask,
      }}
      bug={model.canReadBugs ? {
        state: model.bug,
        page: model.route.bugPage,
        pageSize: INBOX_PAGE_SIZE,
        attentionCount: model.metrics.bugAttention,
        onPageChange: (page) => model.setFeedPage('bug', page),
        onOpen: model.openBug,
      } : undefined}

    />
  )
}
