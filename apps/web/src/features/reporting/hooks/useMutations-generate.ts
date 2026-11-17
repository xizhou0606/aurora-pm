import { useMutation } from '@tanstack/react-query'
import { useCallback } from 'react'

import { OPERATION_FEEDBACK_COPY, REPORTING_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'

import { normalizeExportedDocument, normalizeStartResult } from '../api/normalizers'
import type { ExportedDocument, ReportTargetKind, StartReportResult } from '../model/types'
import { requireReportingOperation } from './useOperationAccess'
import type { ReportingEnvironment } from '../components/environment'

export type ReportGenerateInput = {
  kind: ReportTargetKind
  targetId: number
  type: string
  anchorDate: string
}

export function useReportGenerateMutation(
  environment: ReportingEnvironment,
  canGenerateMember: boolean,
  canGenerateGroup: boolean,
  onSuccess: (result: StartReportResult, input: ReportGenerateInput) => void,
) {
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.reporting.generate),
    mutationFn: async (input: ReportGenerateInput) => {
      requireReportingOperation(input.kind === 'group' ? canGenerateGroup : canGenerateMember)
      const body = { type: input.type, date: input.anchorDate || undefined }
      const raw = input.kind === 'group'
        ? await environment.client.groupGenerate(input.targetId, body)
        : input.kind === 'department'
          ? await environment.client.departmentGenerate(input.targetId, body)
          : await environment.client.memberGenerate(input.targetId, body)
      return normalizeStartResult(raw)
    },
    onSuccess,
  })

  const generate = useCallback((input: ReportGenerateInput) => {
    mutation.mutate(input)
  }, [mutation])

  return {
    generate,
    reset: mutation.reset,
    status: mutation.status,
    isPending: mutation.isPending,
    errorTitle: REPORTING_COPY.report.failedTitle,
  }
}

export type FeishuExportInput = {
  // runId=0 exports the facts-only report (no completed AI run required).
  runId: number
  ctx: string
  facts?: {
    targetKind: ReportTargetKind
    targetId: number
    type: string
    anchorDate: string
  }
}

export function useFeishuExportMutation(
  environment: ReportingEnvironment,
  canPublish: boolean,
  onSuccess: (document: ExportedDocument, input: FeishuExportInput) => void,
) {
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.reporting.feishuExport),
    mutationFn: async (input: FeishuExportInput) => {
      requireReportingOperation(canPublish)
      return normalizeExportedDocument(
        await environment.client.exportFeishuDoc(input.runId, input.facts),
      )
    },
    onSuccess,
  })

  const exportToFeishu = useCallback((input: FeishuExportInput) => {
    mutation.mutate(input)
  }, [mutation])

  return {
    exportToFeishu,
    isPending: mutation.isPending,
  }
}
