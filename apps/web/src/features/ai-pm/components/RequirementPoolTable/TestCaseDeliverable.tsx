import { useEffect, useRef, useState } from 'react'
import { PhDownloadSimple, PhEye, PhCircleNotch } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import {
  Button,
  ResponsivePanel,
  notifyOperationError,
  operationFeedback,
} from '@/shared/ui'
import { downloadPlatformFile, downloadRequirementMaterial, downloadWorkflowNodeArtifact, useRequirementMaterialMutations } from '../../api/api'
import { useRequirementsOperationAccess } from '../../hooks/useOperationAccess'
import { saveBlobAsFile } from '../../utils/workflow-artifact-file'
import {
  DeliverableActions,
  DeliverableFilePicker,
  DeliverableUploadExtra,
  deliverableDisabledActionClass,
  TEST_CASE_UPLOAD_ACCEPT,
  type DeliverablePresentation,
} from './DeliverableComponents'

const poolCopy = copy.aiPM.requirementPool
const MAX_TEXT_PREVIEW_BYTES = 1024 * 1024

export function poolTestCaseArtifacts(requirement: {
  deliverable_overview?: {
    test_case_artifact?: {
      id: number; instance_id: number; node_id: string; kind: string; name: string;
      content_type: string; size: number; url?: string; requirement_id?: number;
      ai_run_id?: number | null; created_at: string;
    } | null
    test_case_artifacts?: Array<{
      id: number; instance_id: number; node_id: string; kind: string; name: string;
      content_type: string; size: number; url?: string; requirement_id?: number;
      ai_run_id?: number | null; created_at: string;
    }>
  }
}) {
  const overview = requirement.deliverable_overview
  if (overview?.test_case_artifacts?.length) return overview.test_case_artifacts
  return overview?.test_case_artifact ? [overview.test_case_artifact] : []
}

async function blobForPoolArtifact(artifact: { id: number; url?: string; requirement_id?: number }) {
  if (artifact.requirement_id) {
    return downloadRequirementMaterial(artifact.requirement_id, artifact.id)
  }
  if (artifact.url) {
    const blob = await downloadPlatformFile(artifact.url)
    if (blob) return blob
  }
  return downloadWorkflowNodeArtifact(artifact.id)
}

function isPreviewableTestCase(artifact: NonNullable<ReturnType<typeof poolTestCaseArtifacts>[number]>) {
  return Boolean((
    (artifact.content_type || '').toLowerCase().startsWith('text/')
    || artifact.name.toLowerCase().endsWith('.csv')
  ) && artifact.size <= MAX_TEXT_PREVIEW_BYTES)
}

export function TestCaseDeliverable({
  requirement,
  presentation = 'chip',
}: {
  requirement: {
    id: number
    deliverable_overview?: Parameters<typeof poolTestCaseArtifacts>[0]['deliverable_overview']
  }
  presentation?: DeliverablePresentation
}) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const mutations = useRequirementMaterialMutations(workspaceId)
  const access = useRequirementsOperationAccess()
  const [activeAction, setActiveAction] = useState<'preview' | 'download' | null>(null)
  const [busyArtifactId, setBusyArtifactId] = useState(0)
  const [previewData, setPreviewData] = useState<{ artifactId: number; name: string; text: string } | null>(null)
  const actionTokenRef = useRef(0)
  const artifacts = poolTestCaseArtifacts(requirement)
  const artifactKey = artifacts.map((artifact) => artifact.id).join(',')
  const label = poolCopy.deliverableTestCases
  useEffect(() => {
    actionTokenRef.current += 1
    setActiveAction(null)
    setBusyArtifactId(0)
    setPreviewData(null)
  }, [artifactKey])
  const preview = async (artifact: (typeof artifacts)[number]) => {
    if (!access.canDownloadMaterial(isPreviewableTestCase(artifact) && !activeAction)) return
    const actionToken = ++actionTokenRef.current
    setActiveAction('preview')
    setBusyArtifactId(artifact.id)
    try {
      const blob = await blobForPoolArtifact(artifact)
      const text = await blob.text()
      if (actionToken === actionTokenRef.current) {
        setPreviewData({ artifactId: artifact.id, name: artifact.name, text })
      }
    } catch (error) {
      if (actionToken === actionTokenRef.current) {
        notifyOperationError(operationFeedback('', poolCopy.deliverablePreviewFailed(label)), error)
      }
    } finally {
      if (actionToken === actionTokenRef.current) {
        setActiveAction(null)
        setBusyArtifactId(0)
      }
    }
  }
  const download = async (artifact: (typeof artifacts)[number]) => {
    if (!access.canDownloadMaterial(!activeAction)) return
    const actionToken = ++actionTokenRef.current
    setActiveAction('download')
    setBusyArtifactId(artifact.id)
    try {
      saveBlobAsFile(await blobForPoolArtifact(artifact), artifact.name)
    } catch (error) {
      notifyOperationError(operationFeedback('', poolCopy.deliverableDownloadFailed(label)), error)
    } finally {
      if (actionToken === actionTokenRef.current) {
        setActiveAction(null)
        setBusyArtifactId(0)
      }
    }
  }
  const upload = (file: File) => {
    void mutations.upload.mutateAsync({ requirementId: requirement.id, kind: 'test_case', file, source: 'requirement' })
  }
  const uploadExtras = (close: () => void, open: () => void) => access.canUploadMaterial() ? (
    <DeliverableUploadExtra
      label={label}
      pending={mutations.upload.isPending || Boolean(activeAction)}
      onPick={() => { close(); open() }}
    />
  ) : null
  if (!artifacts.length) {
    if (!access.canUploadMaterial()) return null
    return (
      <div className="flex w-full justify-center">
        <DeliverableFilePicker accept={TEST_CASE_UPLOAD_ACCEPT} disabled={mutations.upload.isPending} onPick={upload}>
          {(open) => (
            <DeliverableActions
              kind="test-cases"
              label={label}
              presentation={presentation}
              triggerLabel={poolCopy.deliverableUpload(label)}
              className="grid min-w-56 gap-1 rounded-md p-1.5"
            >
              {(close) => uploadExtras(close, open)}
            </DeliverableActions>
          )}
        </DeliverableFilePicker>
      </div>
    )
  }
  return (
    <>
      <div className="flex w-full min-w-0 items-center justify-center">
        <DeliverableFilePicker accept={TEST_CASE_UPLOAD_ACCEPT} disabled={mutations.upload.isPending} onPick={upload}>
          {(open) => (
        <DeliverableActions kind="test-cases" label={label} presentation={presentation} className="grid min-w-72 max-w-sm gap-1 rounded-md p-1.5">
          {(close) => (
            <>
              {artifacts.map((artifact) => {
                const previewable = isPreviewableTestCase(artifact)
                const previewLabel = poolCopy.deliverablePreviewFile(label, artifact.name)
                const downloadLabel = poolCopy.deliverableDownloadFile(label, artifact.name)
                return (
                  <div key={artifact.id} className="flex min-w-0 items-center gap-1">
                    <span className="min-w-0 flex-1 truncate px-1 text-xs text-muted-foreground" title={artifact.name}>{artifact.name}</span>
                    {access.canDownloadMaterial(previewable) ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        disabled={Boolean(activeAction)}
                        aria-label={previewLabel}
                        title={previewLabel}
                        className="text-muted-foreground hover:bg-card hover:text-foreground"
                        onClick={() => { close(); void preview(artifact) }}
                      >
                        {activeAction === 'preview' && busyArtifactId === artifact.id ? <PhCircleNotch aria-hidden="true" className="animate-spin" /> : <PhEye aria-hidden="true" />}
                      </Button>
                    ) : (
                      <span
                        role="button"
                        aria-disabled="true"
                        aria-label={poolCopy.deliverablePreviewUnavailable(label)}
                        title={poolCopy.deliverablePreviewUnavailable(label)}
                        className={deliverableDisabledActionClass}
                      >
                        <PhEye aria-hidden="true" />
                      </span>
                    )}
                    {access.canDownloadMaterial() ? <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      disabled={Boolean(activeAction)}
                      aria-label={downloadLabel}
                      title={downloadLabel}
                      className="text-muted-foreground hover:bg-card hover:text-foreground"
                      onClick={() => { close(); void download(artifact) }}
                    >
                      {activeAction === 'download' && busyArtifactId === artifact.id ? <PhCircleNotch aria-hidden="true" className="animate-spin" /> : <PhDownloadSimple aria-hidden="true" />}
                    </Button> : null}
                  </div>
                )
              })}
              {uploadExtras(close, open)}
            </>
          )}
        </DeliverableActions>
          )}
        </DeliverableFilePicker>
      </div>
      <ResponsivePanel
        open={Boolean(previewData)}
        onOpenChange={(nextOpen) => { if (!nextOpen) setPreviewData(null) }}
        title={poolCopy.deliverablePreview(label)}
        description={previewData?.name || artifacts[0]?.name}
        className="max-w-3xl"
        contentClassName="bg-muted/70"
      >
        <pre className="m-0 min-h-48 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-card p-4 text-xs leading-6 text-foreground">{previewData?.text || ''}</pre>
      </ResponsivePanel>
    </>
  )
}