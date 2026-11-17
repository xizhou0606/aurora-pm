import { useState, useEffect } from 'react'
import { PhProhibit, PhClock } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AiLoading, Button } from '@/shared/ui'
import type { WorkflowAIRunState } from '../components/WorkflowAIAssistant'

const workflowCopy = copy.aiPM.workflow

export function CanvasAILoading({ state, cancelling, onCancel }: { state: WorkflowAIRunState; cancelling: boolean; onCancel: () => void }) {
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    setElapsed(0)
    const timer = window.setInterval(() => setElapsed((value) => value + 1), 1000)
    return () => window.clearInterval(timer)
  }, [state.runId])
  const title = state.phase === 'saving' ? workflowCopy.aiCanvasSaving : state.phase === 'starting' ? workflowCopy.aiCanvasStarting : state.phase === 'queued' ? workflowCopy.aiCanvasQueued : workflowCopy.aiCanvasGenerating
  const stage = state.phase === 'saving' ? workflowCopy.aiStageSaving : state.phase === 'queued' ? workflowCopy.aiStageQueued : elapsed < 8 ? workflowCopy.aiStageReading : elapsed < 20 ? workflowCopy.aiStageStructuring : workflowCopy.aiStageStillWorking
  return <div className="fixed inset-0 z-30 grid place-items-center bg-background/75 px-6" role="dialog" aria-labelledby="workflow-ai-loading-title">
    <div className="flex w-full max-w-sm flex-col items-center text-center">
      <AiLoading size={72} className="mx-auto" />
      <h2 id="workflow-ai-loading-title" className="mt-4 mb-0 text-base font-semibold">{title}</h2>
      <p className="mt-2 mb-0 text-xs leading-5 text-muted-foreground">{workflowCopy.aiCanvasBusyHint}</p>
      <div aria-live="polite" className="mx-auto mt-4 flex w-fit items-center gap-2 rounded-full bg-muted px-3 py-1.5 text-xs text-foreground"><AiLoading size={16} label={null} />{stage}</div>
      <div className="mt-4 h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full w-2/3 animate-pulse rounded-full bg-primary" /></div>
      <div className="mt-3 flex items-center justify-center gap-1.5 text-xs text-muted-foreground"><PhClock className="size-3.5" />{workflowCopy.aiElapsed(elapsed)}{state.runId ? ` · ${workflowCopy.aiRun(state.runId)}` : ''}</div>
      {elapsed >= 30 ? <p className="mt-3 mb-0 rounded-md bg-amber-50 dark:bg-amber-950/40 px-3 py-2 text-xs leading-5 text-amber-800">{workflowCopy.aiSlowHint}</p> : null}
      {state.runId && state.phase !== 'saving' ? <Button className="mt-4" size="sm" variant="outline" disabled={cancelling} onClick={onCancel}><PhProhibit />{cancelling ? workflowCopy.aiCancelling : workflowCopy.aiCancelRun}</Button> : null}
    </div>
  </div>
}