import { useState } from 'react'
import { PhFileXls, PhImage, PhLink } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { Button, IconBadge, Input, LazyFileUploader, notifyOperationError, operationFeedback } from '@/shared/ui'
import { useRequirementMaterialMutations, useRequirementMaterials } from '../api/api'
import type { RequirementMaterial, WorkflowNodeArtifact } from '../model/types'
import { isHttpURL, previewWorkflowNodeArtifact } from '../utils/workflow-artifact-file'
import { designMaterialAttachURL } from '../utils/workflow-node-materials'
import { useRequirementsOperationAccess } from '../hooks/useOperationAccess'

const detailCopy = copy.aiPM.requirementDetail
const subjectCopy = copy.aiPM.workflowSubject
const commonCopy = copy.aiPM.common

function asArtifact(row: RequirementMaterial): WorkflowNodeArtifact {
  return {
    id: row.id,
    instance_id: 0,
    node_id: '',
    kind: row.artifact_kind === 'ai_generated' ? 'ai_generated' : 'uploaded',
    name: row.name,
    content_type: row.content_type,
    size: row.size,
    url: designMaterialAttachURL(row) || row.url,
    source: row.content_type === 'text/uri-list' ? 'link' : 'file',
    ai_run_id: row.ai_run_id,
    uploader_user_id: row.uploader_user_id,
    created_at: row.created_at,
  }
}

export function RequirementMaterialsPanel({ workspaceId, requirementId }: { workspaceId: number; requirementId: number }) {
  const designs = useRequirementMaterials(workspaceId, requirementId, 'design')
  const cases = useRequirementMaterials(workspaceId, requirementId, 'test_case')
  const mutations = useRequirementMaterialMutations(workspaceId)
  const access = useRequirementsOperationAccess()
  const [designLink, setDesignLink] = useState('')
  const [caseLink, setCaseLink] = useState('')
  const trimmedLink = designLink.trim()
  const linkValid = !trimmedLink || isHttpURL(trimmedLink)
  const trimmedCaseLink = caseLink.trim()
  const caseLinkValid = !trimmedCaseLink || isHttpURL(trimmedCaseLink)
  const preview = (row: RequirementMaterial) => {
    void previewWorkflowNodeArtifact(asArtifact(row)).catch((error) => {
      notifyOperationError(operationFeedback('', detailCopy.deliverablesTestCaseOpenFailed), error)
    })
  }
  if (!access.canViewMaterials()) return null
  return (
    <div className="grid gap-3">
      <p className="m-0 text-xs leading-5 text-muted-foreground">{detailCopy.deliverablesEarlyHint}</p>
      <section className="grid gap-2 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/50 dark:bg-blue-950/40 p-3">
        <div className="flex items-start gap-2.5">
          <IconBadge icon={PhImage} tone="blue" />
          <strong className="text-xs text-foreground">{detailCopy.deliverablesDesign}</strong>
        </div>
        <LazyFileUploader
          label={detailCopy.deliverablesUploadDesign}
          inputLabel={detailCopy.deliverablesUploadDesign}
          hint={subjectCopy.designImageHint}
          accept="image/png,image/jpeg,image/webp,image/gif,.png,.jpg,.jpeg,.webp,.gif"
          maxBytes={20 * 1024 * 1024}
          items={(designs.data ?? []).map((row) => ({ ...asArtifact(row), url: row.url || `pending:${row.id}` }))}
          multiple
          disabled={!access.canUploadMaterial()}
          pending={mutations.upload.isPending}
          error={mutations.upload.isError ? subjectCopy.designImageUploadFailed : undefined}
          onUpload={(file) => mutations.upload.mutateAsync({ requirementId, kind: 'design', file, source: 'requirement' })}
          onPreview={(item) => {
            const row = (designs.data ?? []).find((material) => material.id === item.id)
            if (row) preview(row)
          }}
        />
        {access.canUploadMaterial() ? <label className="grid gap-1.5 text-xs font-medium text-foreground">
          <span className="flex items-center gap-1.5"><PhLink className="size-3.5 text-blue-600" />{detailCopy.deliverablesDesignLink}</span>
          <Input
            name="requirement-design-link"
            type="url"
            aria-label={detailCopy.deliverablesDesignLink}
            aria-invalid={!linkValid || undefined}
            value={designLink}
            placeholder={detailCopy.deliverablesDesignLinkPlaceholder}
            onChange={(event) => setDesignLink(event.target.value)}
          />
        </label> : null}
        {!linkValid ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.designLinkInvalid}</p> : trimmedLink ? (
          <Button
            size="sm"
            variant="outline"
            className="justify-self-start bg-card"
            disabled={mutations.upload.isPending}
            onClick={() => {
              void mutations.upload.mutateAsync({ requirementId, kind: 'design', url: trimmedLink, source: 'requirement' }).then(() => setDesignLink(''))
            }}
          >
            {commonCopy.save}
          </Button>
        ) : null}
      </section>
      <section className="grid gap-2 rounded-lg border border-border bg-muted/30 p-3">
        <div className="flex items-start gap-2.5">
          <IconBadge icon={PhFileXls} tone="violet" />
          <strong className="text-xs text-foreground">{detailCopy.deliverablesTestCases}</strong>
        </div>
        <LazyFileUploader
          label={detailCopy.deliverablesUploadTestCase}
          inputLabel={detailCopy.deliverablesUploadTestCase}
          hint={subjectCopy.testCaseUploadHint}
          accept=".csv,.xls,.xlsx,.pdf,text/csv,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf"
          maxBytes={20 * 1024 * 1024}
          items={(cases.data ?? []).map((row) => ({ ...asArtifact(row), url: row.url || `pending:${row.id}` }))}
          multiple
          disabled={!access.canUploadMaterial()}
          pending={mutations.upload.isPending}
          error={mutations.upload.isError ? subjectCopy.testCaseUploadFailed : undefined}
          onUpload={(file) => mutations.upload.mutateAsync({ requirementId, kind: 'test_case', file, source: 'requirement' })}
          onPreview={(item) => {
            const row = (cases.data ?? []).find((material) => material.id === item.id)
            if (row) preview(row)
          }}
        />
        {access.canUploadMaterial() ? <label className="grid gap-1.5 text-xs font-medium text-foreground">
          <span className="flex items-center gap-1.5"><PhLink className="size-3.5 text-violet-600 dark:text-violet-300" />{subjectCopy.testCaseLinkLabel}</span>
          <Input
            name="requirement-test-case-link"
            type="url"
            aria-label={subjectCopy.testCaseLinkLabel}
            aria-invalid={!caseLinkValid || undefined}
            value={caseLink}
            placeholder={subjectCopy.testCaseLinkPlaceholder}
            onChange={(event) => setCaseLink(event.target.value)}
          />
        </label> : null}
        {!caseLinkValid ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.designLinkInvalid}</p> : trimmedCaseLink ? (
          <Button
            size="sm"
            variant="outline"
            className="justify-self-start bg-card"
            disabled={mutations.upload.isPending}
            onClick={() => {
              void mutations.upload.mutateAsync({ requirementId, kind: 'test_case', url: trimmedCaseLink, source: 'requirement' }).then(() => setCaseLink(''))
            }}
          >
            {commonCopy.save}
          </Button>
        ) : null}
      </section>
    </div>
  )
}
