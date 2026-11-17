import { useCallback, useEffect, useRef, useState } from 'react'
import { PhLink, PhProhibit, PhDownloadSimple, PhEye, PhFileXls, PhCircleNotch, PhArrowsClockwise, PhMagicWand } from '@/shared/ui/icons'

import { AiLoading, AiLoadingOverlay, Button, IconBadge, Input, LazyFileUploader, ResponsivePanel, notifyOperationError, operationFeedback } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { useAICapabilities, useAIMutations, useAIRun, useAIRuns, useWorkflowMutations, useWorkflowTestCaseArtifacts } from '../api/api'
import { aiRunErrorMessage } from '../utils/run-error-message'
import type { WorkflowInstance } from '../model/types'
import { isHttpURL, previewWorkflowNodeArtifact, saveWorkflowNodeArtifact } from '../utils/workflow-artifact-file'
import { buildTestCaseCSV, TestCasePreview } from './WorkflowTestCasePreview'
import { useAIRunsOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'

export { buildTestCaseCSV } from './WorkflowTestCasePreview'

const subjectCopy = copy.aiPM.workflowSubject

export function isTestCaseWorkflowNode(node?: { type: string; name: string; config?: Record<string, unknown> }) {
  return Boolean(node && (node.type === 'human_task' || node.type === 'form') && (
    node.config?.test_case_artifact === true || node.name.includes('测试用例') || node.name.includes('测试案例')
  ))
}

export function testCaseGenerationStartedAt(
  run?: { status?: string; started_at?: string; updated_at?: string; created_at?: string },
  fallback = 0,
) {
  const values = run?.status === 'queued'
    ? [run.updated_at, run.created_at]
    : [run?.started_at, run?.updated_at, run?.created_at]
  for (const value of values) {
    if (!value) continue
    const parsed = Date.parse(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function downloadCSV(contents: string, filename: string) {
  const url = URL.createObjectURL(new Blob([contents], { type: 'text/csv;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

export function WorkflowTestCaseWorkbench({ workspaceId, instance, node, readOnly = false }: {
  workspaceId: number
  instance: Pick<WorkflowInstance, 'id' | 'subject_type' | 'subject_id'>
  node: WorkflowInstance['timeline_nodes'][number]
  readOnly?: boolean
}) {
  'use no memo'
  const [runId, setRunId] = useState(0)
  const [autoUploadInFlightRunId, setAutoUploadInFlightRunId] = useState(0)
  const [autoUploadErrorRunId, setAutoUploadErrorRunId] = useState(0)
  const [generationStartedAt, setGenerationStartedAt] = useState(0)
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [testCaseLink, setTestCaseLink] = useState('')
  const uploadAttemptedRunId = useRef(0)
  const capabilities = useAICapabilities(workspaceId)
  const workflowAccess = useWorkflowOperationAccess()
  const aiAccess = useAIRunsOperationAccess()
  const ai = useAIMutations(workspaceId)
  // The selected run has its own faster status query below. Keep the list as
  // discovery/cache state so one active run does not create duplicate polling.
  const subjectRuns = useAIRuns(workspaceId, instance.subject_type, instance.subject_id, { pollActive: false })
  const workflow = useWorkflowMutations(workspaceId)
  const artifacts = useWorkflowTestCaseArtifacts(workspaceId, instance.id, node.id)
  const persistedGeneratedArtifact = artifacts.data?.find((artifact) => artifact.kind === 'ai_generated')
  const latestDeliveryRun = subjectRuns.data?.rows.find((candidate) => (
    candidate.capability === 'qa.test_cases'
    && candidate.test_case_delivery?.instance_id === instance.id
    && candidate.test_case_delivery.node_id === node.id
  ))
  const latestRunCannotRestoreArtifact = Boolean(
    persistedGeneratedArtifact?.ai_run_id && latestDeliveryRun && ['failed', 'cancelled'].includes(latestDeliveryRun.status),
  )
  const restoredRunId = latestRunCannotRestoreArtifact
    ? persistedGeneratedArtifact?.ai_run_id
    : latestDeliveryRun?.id || persistedGeneratedArtifact?.ai_run_id
  const displayedRunId = runId || restoredRunId || 0
  const run = useAIRun(workspaceId, displayedRunId)
  // Disabled/changed TanStack queries may briefly retain the previous run.
  // Only render data that belongs to the selected current or persisted run.
  const currentRun = run.data?.id === displayedRunId ? run.data : undefined
  const testCases = currentRun?.output?.testCases ?? []
  const testCaseDesign = currentRun?.output?.testCaseDesign
  const selectedStatus = currentRun?.status ?? (runId ? 'queued' : latestDeliveryRun?.status)
  const running = Boolean(displayedRunId && selectedStatus && ['queued', 'running'].includes(selectedStatus))
  const failed = currentRun?.status === 'failed'
  const cancelled = currentRun?.status === 'cancelled'
  const reconnecting = Boolean(running && run.isError)
  const generating = ai.create.isPending || running
  const elapsedRun = currentRun ?? (
    latestDeliveryRun?.id === displayedRunId ? latestDeliveryRun : undefined
  )
  const elapsedStartedAt = testCaseGenerationStartedAt(elapsedRun, generationStartedAt)
  const filename = subjectCopy.testCaseFileName(instance.id)
  const csv = testCases.length ? buildTestCaseCSV(testCases, testCaseDesign) : ''
  const generatedArtifact = (
    workflow.materializeTestCaseArtifact.data?.kind === 'ai_generated'
    && (!currentRun || workflow.materializeTestCaseArtifact.data.ai_run_id === currentRun.id)
      ? workflow.materializeTestCaseArtifact.data
      : undefined
  ) ?? artifacts.data?.find((artifact) => (
    artifact.kind === 'ai_generated' && (!currentRun || artifact.ai_run_id === currentRun.id)
  ))
  const completedWithoutCases = Boolean(
    currentRun?.status === 'completed' && !testCases.length && !generatedArtifact,
  )
  const manualArtifacts = artifacts.data?.filter((artifact) => artifact.kind !== 'ai_generated') ?? []
  const canUploadMaterial = workflowAccess.canUploadTestCaseMaterial(!readOnly)
  const trimmedTestCaseLink = testCaseLink.trim()
  const testCaseLinkValid = !trimmedTestCaseLink || isHttpURL(trimmedTestCaseLink)
  const saveTestCaseLink = () => {
    if (!trimmedTestCaseLink || !isHttpURL(trimmedTestCaseLink)) return
    void workflow.uploadTestCaseArtifact
      .mutateAsync({ instanceId: instance.id, nodeId: node.id, url: trimmedTestCaseLink })
      .then(() => setTestCaseLink(''))
  }
  const autoUploadComplete = Boolean(currentRun?.id && generatedArtifact)
  const autoUploading = Boolean(currentRun?.id && autoUploadInFlightRunId === currentRun.id)
  const autoUploadFailed = Boolean(currentRun?.id && !generatedArtifact && autoUploadErrorRunId === currentRun.id)
  const downloadGenerated = () => {
    if (generatedArtifact) {
      void saveWorkflowNodeArtifact(generatedArtifact).catch((error) => {
        notifyOperationError(operationFeedback('', subjectCopy.testCaseDownloadFailed), error)
      })
      return
    }
    if (csv) downloadCSV(csv, filename)
  }
  const generate = () => {
    if (!workflowAccess.canGenerateTestCaseMaterial(!readOnly) || !aiAccess.canCreateRun()) return
    setRunId(0)
    setAutoUploadInFlightRunId(0)
    setAutoUploadErrorRunId(0)
    uploadAttemptedRunId.current = 0
    workflow.materializeTestCaseArtifact.reset()
    setGenerationStartedAt(Date.now())
    setElapsedSeconds(0)
    ai.create.mutate({
      capability: 'qa.test_cases', subject_type: instance.subject_type, subject_id: instance.subject_id,
      test_case_delivery: { instance_id: instance.id, node_id: node.id },
    }, {
      onSuccess: (created) => {
        setRunId(created.id)
      },
      onError: () => setGenerationStartedAt(0),
    })
  }
  const materializeGenerated = useCallback((aiRunId: number) => {
    if (!workflowAccess.canGenerateTestCaseMaterial(!readOnly)) return
    uploadAttemptedRunId.current = aiRunId
    setAutoUploadInFlightRunId(aiRunId)
    setAutoUploadErrorRunId(0)
    workflow.materializeTestCaseArtifact.mutate({
      instanceId: instance.id, nodeId: node.id, aiRunId,
    }, {
      onSuccess: () => {
        setAutoUploadInFlightRunId(0)
      },
      onError: () => {
        setAutoUploadInFlightRunId(0)
        setAutoUploadErrorRunId(aiRunId)
      },
    })
  }, [instance.id, node.id, readOnly, workflow.materializeTestCaseArtifact])

  useEffect(() => {
    if (!workflowAccess.canGenerateTestCaseMaterial(!readOnly) || !currentRun || currentRun.status !== 'completed' || !testCases.length || generatedArtifact) return
    if (uploadAttemptedRunId.current === currentRun.id) return
    materializeGenerated(currentRun.id)
  }, [currentRun, generatedArtifact, materializeGenerated, readOnly, testCases.length])

  useEffect(() => {
    if (!autoUploadFailed) return
    const timer = window.setInterval(() => void artifacts.refetch(), 5_000)
    return () => window.clearInterval(timer)
  }, [artifacts.refetch, autoUploadFailed])

  useEffect(() => {
    if (!generating || !elapsedStartedAt) return
    const update = () => setElapsedSeconds(Math.max(0, Math.floor((Date.now() - elapsedStartedAt) / 1000)))
    update()
    const timer = window.setInterval(update, 1000)
    return () => window.clearInterval(timer)
  }, [elapsedStartedAt, generating])

  return <section className="relative grid gap-3 rounded-lg border border-violet-200 dark:border-violet-800 bg-violet-50/40 dark:bg-violet-950/40 p-3" aria-label={subjectCopy.testCaseTitle}>
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex min-w-0 items-start gap-2.5">
        <IconBadge icon={PhFileXls} tone="violet" />
        <div><strong className="text-sm text-foreground">{subjectCopy.testCaseTitle}</strong><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.testCaseDescription}</p></div>
      </div>
    </div>

    <div className="flex flex-wrap gap-2">
      {workflowAccess.canGenerateTestCaseMaterial(!readOnly) && aiAccess.canCreateRun() ? <Button type="button" size="sm" disabled={!capabilities.data?.enabled || generating} onClick={generate}>
        {generating ? <AiLoading size={16} label={null} /> : <PhMagicWand />}
        {generating ? subjectCopy.testCaseGenerating : subjectCopy.testCaseGenerate}
      </Button> : null}
      {running && aiAccess.canCancelRun(!readOnly) ? <Button type="button" size="sm" variant="outline" className="bg-card" disabled={ai.cancel.isPending} onClick={() => ai.cancel.mutate(displayedRunId, { onSuccess: () => void run.refetch() })}>
        {ai.cancel.isPending ? <PhCircleNotch className="animate-spin" /> : <PhProhibit />}
        {ai.cancel.isPending ? subjectCopy.testCaseCancelling : subjectCopy.testCaseCancel}
      </Button> : null}
      {!generating && testCases.length ? <Button
        type="button"
        size="icon-sm"
        variant="outline"
        className="bg-card"
        aria-label={subjectCopy.testCasePreview}
        title={subjectCopy.testCasePreview}
        onClick={() => setPreviewOpen(true)}
      ><PhEye /></Button> : null}
      {!generating && workflowAccess.canDownloadTestCaseMaterial(Boolean(testCases.length || generatedArtifact)) ? <Button
        type="button"
        size="icon-sm"
        variant="outline"
        className="bg-card"
        aria-label={subjectCopy.testCaseDownload}
        title={subjectCopy.testCaseDownload}
        onClick={downloadGenerated}
      ><PhDownloadSimple /></Button> : null}
      {testCases.length && autoUploadFailed ? <Button
        type="button"
        size="icon-sm"
        variant="outline"
        className="bg-card"
        aria-label={subjectCopy.testCaseAutoUploadRetry}
        title={subjectCopy.testCaseAutoUploadRetry}
        disabled={!workflowAccess.canGenerateTestCaseMaterial(!readOnly)}
        onClick={() => currentRun && materializeGenerated(currentRun.id)}
      ><PhArrowsClockwise /></Button> : null}
    </div>
    {capabilities.data && !capabilities.data.enabled ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.testCaseAIUnavailable}</p> : null}
    {reconnecting ? <p role="status" className="m-0 text-xs text-amber-700 dark:text-amber-400">{subjectCopy.testCaseStatusRetrying}</p> : null}
    {!generating && (ai.create.isError || failed) ? <p role="alert" className="m-0 text-xs text-destructive">{currentRun?.error_message ? aiRunErrorMessage(currentRun.error_message) : subjectCopy.testCaseGenerateFailed}</p> : null}
    {cancelled ? <p role="status" className="m-0 text-xs text-muted-foreground">{subjectCopy.testCaseGenerateCancelled}</p> : null}
    {completedWithoutCases ? <p role="status" className="m-0 text-xs text-amber-700 dark:text-amber-400">{subjectCopy.testCaseGenerateEmpty}</p> : null}
    {testCases.length && autoUploading ? <p role="status" className="m-0 flex items-center gap-1.5 text-xs text-violet-700 dark:text-violet-300"><PhCircleNotch className="size-3.5 animate-spin" />{subjectCopy.testCaseAutoUploading}</p> : null}
    {testCases.length && autoUploadComplete ? <p role="status" className="m-0 text-xs text-emerald-700 dark:text-emerald-400">{subjectCopy.testCaseAutoUploaded}</p> : null}
    {testCases.length && autoUploadFailed ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.testCaseAutoUploadFailed}</p> : null}
    {generating ? <AiLoadingOverlay label={subjectCopy.testCaseGeneratingWithElapsed(subjectCopy.testCaseGeneratingElapsed(elapsedSeconds))} /> : null}

    <LazyFileUploader
      label={subjectCopy.testCaseUploadLabel}
      inputLabel={subjectCopy.testCaseUploadInput}
      hint={subjectCopy.testCaseUploadHint}
      accept=".csv,.xls,.xlsx,.pdf,text/csv,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf"
      maxBytes={20 * 1024 * 1024}
      items={manualArtifacts}
      multiple
      disabled={!canUploadMaterial}
      pending={workflow.uploadTestCaseArtifact.isPending}
      error={workflow.uploadTestCaseArtifact.isError && !autoUploadFailed ? subjectCopy.testCaseUploadFailed : undefined}
      onUpload={(file) => workflow.uploadTestCaseArtifact.mutateAsync({ instanceId: instance.id, nodeId: node.id, file })}
      onPreview={(item) => {
        void previewWorkflowNodeArtifact(item).catch((error) => {
          notifyOperationError(operationFeedback('', subjectCopy.testCasePreviewFailed), error)
        })
      }}
    />
    {canUploadMaterial ? <label className="grid gap-1.5 text-xs font-medium text-foreground">
      <span className="flex items-center gap-1.5"><PhLink className="size-3.5 text-violet-600 dark:text-violet-300" />{subjectCopy.testCaseLinkLabel}</span>
      <Input
        name="workflow-test-case-link"
        type="url"
        aria-label={subjectCopy.testCaseLinkLabel}
        aria-invalid={!testCaseLinkValid || undefined}
        value={testCaseLink}
        placeholder={subjectCopy.testCaseLinkPlaceholder}
        onChange={(event) => setTestCaseLink(event.target.value)}
      />
    </label> : null}
    {!testCaseLinkValid ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.designLinkInvalid}</p> : trimmedTestCaseLink ? (
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="justify-self-start bg-card"
        disabled={workflow.uploadTestCaseArtifact.isPending}
        onClick={saveTestCaseLink}
      >{copy.aiPM.common.save}</Button>
    ) : null}
    <ResponsivePanel
      open={previewOpen}
      onOpenChange={setPreviewOpen}
      title={subjectCopy.testCasePreviewTitle}
      description={subjectCopy.testCasePreviewDescription(testCases.length)}
      className="max-w-4xl"
      contentClassName="bg-muted/60"
    >
      <TestCasePreview testCases={testCases} design={testCaseDesign} />
    </ResponsivePanel>
  </section>
}
