import { PhRobot, PhClock, PhClockCounterClockwise, PhPath, PhUser } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AlertBanner, ResponsivePanel, StatusPill } from '@/shared/ui'
import { formatWorkflowTime, historyActionLabel, historyTone, historyStatusLabel } from './WorkflowSubjectLauncher/utils'

const subjectCopy = copy.aiPM.workflowSubject

interface HistoryPanelParams {
  open: boolean
  onOpenChange: (open: boolean) => void
  history: any[]
}

export function WorkflowInstanceControlHistoryPanel({ open, onOpenChange, history }: HistoryPanelParams) {
  return (
    <ResponsivePanel open={open} onOpenChange={onOpenChange}
      title={subjectCopy.historyTitle} description={subjectCopy.historyDescription}
      closeLabel={copy.ui.close} className="max-w-xl"
      headerActions={<StatusPill tone="neutral">{subjectCopy.historyCount(history.length)}</StatusPill>}
    >
      {history.length ? (
        <div className="grid gap-0.5">
          {history.map((item, index) => (
            <div key={item.id} className="relative grid grid-cols-[2rem_minmax(0,1fr)] gap-3 pb-4 last:pb-0">
              {index < history.length - 1 ? <span aria-hidden="true" className="absolute top-8 bottom-0 left-3.75 w-px bg-border" /> : null}
              <span className={`relative z-10 grid size-8 place-items-center rounded-full border ${item.status === 'rejected' || item.status === 'failed' ? 'border-red-200 dark:border-red-800 bg-red-50 text-red-600 dark:text-red-400' : item.operator_type === 'ai' ? 'border-primary/30 bg-primary/10 text-primary' : 'border-border bg-muted text-muted-foreground'}`}>
                {item.operator_type === 'ai' ? <PhRobot className="size-3.5" /> : item.operator_type === 'user' ? <PhUser className="size-3.5" /> : <PhPath className="size-3.5" />}
              </span>
              <article className="min-w-0 rounded-lg border border-border bg-card p-3">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="min-w-0"><strong className="block truncate text-sm text-foreground">{item.node_name}</strong><span className="mt-0.5 block text-xs text-muted-foreground">{historyActionLabel(item.action)}</span></div>
                  <StatusPill tone={historyTone(item.status)}>{historyStatusLabel(item.status)}</StatusPill>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground"><span className="inline-flex items-center gap-1"><PhUser className="size-3" />{item.operator_name}</span><span className="inline-flex items-center gap-1"><PhClock className="size-3" />{formatWorkflowTime(item.occurred_at)}</span></div>
                {item.target_node_name ? <p className="mt-2 mb-0 text-xs font-medium text-amber-700 dark:text-amber-400">{subjectCopy.historyRollbackTarget(item.target_node_name)}</p> : null}
                {item.comment ? <p className="mt-2 mb-0 rounded-md bg-muted px-2.5 py-2 text-xs leading-5 text-muted-foreground">{item.comment}</p> : null}
                {item.error_message ? <AlertBanner tone="error" className="mt-2">{item.error_message}</AlertBanner> : null}
              </article>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid min-h-40 place-items-center rounded-lg border border-dashed border-border bg-muted/60 p-6 text-center">
          <div><PhClockCounterClockwise className="mx-auto mb-2 size-6 text-muted-foreground" /><p className="m-0 text-sm text-muted-foreground">{subjectCopy.historyEmpty}</p></div>
        </div>
      )}
    </ResponsivePanel>
  )
}