import { PhArrowsClockwise } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AlertBanner, Button, ResponsivePanel, Skeleton } from '@/shared/ui'

const subjectCopy = copy.aiPM.workflowSubject

export function WorkflowInstanceControlLoading({
  chrome, initialNodeName, onDismiss,
}: {
  chrome: string; initialNodeName?: string; onDismiss?: () => void
}) {
  if (chrome === 'dialog') {
    return (
      <div>
        <ResponsivePanel key="workflow-node-action" open
          onOpenChange={(nextOpen) => { if (!nextOpen) onDismiss?.() }}
          title={initialNodeName ? subjectCopy.todoTitle(initialNodeName) : subjectCopy.currentTodo}
          closeLabel={copy.aiPM.common.cancel} className="max-w-6xl"
        >
          <div className="grid gap-3">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-2 w-full" />
            <Skeleton className="h-24 w-full" />
          </div>
        </ResponsivePanel>
      </div>
    )
  }
  return (
    <div className="grid gap-3">
      <Skeleton className="h-4 w-40" /><Skeleton className="h-2 w-full" /><Skeleton className="h-24 w-full" />
    </div>
  )
}

export function WorkflowInstanceControlError({
  chrome, initialNodeName, onDismiss, refetch,
}: {
  chrome: string; initialNodeName?: string; onDismiss?: () => void
  refetch: () => void
}) {
  if (chrome === 'dialog') {
    return (
      <div>
        <ResponsivePanel key="workflow-node-action" open
          onOpenChange={(nextOpen) => { if (!nextOpen) onDismiss?.() }}
          title={initialNodeName ? subjectCopy.todoTitle(initialNodeName) : subjectCopy.currentTodo}
          closeLabel={copy.aiPM.common.cancel} className="max-w-6xl"
        >
          <AlertBanner tone="error" action={<Button size="xs" variant="outline" onClick={() => void refetch()}><PhArrowsClockwise />{copy.ui.retry}</Button>}>{subjectCopy.loadFailed}</AlertBanner>
        </ResponsivePanel>
      </div>
    )
  }
  return (
    <AlertBanner tone="error" action={<Button size="xs" variant="outline" onClick={() => void refetch()}><PhArrowsClockwise />{copy.ui.retry}</Button>}>{subjectCopy.loadFailed}</AlertBanner>
  )
}