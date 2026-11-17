import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

type Capability = (locallyAvailable?: boolean) => boolean

type DomainState = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
}

export type RequirementsOperationAccess = DomainState & {
  canViewPool: Capability
  canCreateRequirement: Capability
  canUpdateRequirement: Capability
  canBatchMoveRequirement: Capability
  canDeleteRequirement: Capability
  canScheduleRequirement: Capability
  canValidateDocument: Capability
  canUploadDesignAsset: Capability
  canAssignRoleOwners: Capability
  canViewMaterials: Capability
  canUploadMaterial: Capability
  canDownloadMaterial: Capability
  canDeleteMaterial: Capability
}

export function useRequirementsOperationAccess(): RequirementsOperationAccess {
  const operations = useOperationCapabilities('requirements')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewPool: (local = true) => operations.allows(PM_ACTION.RequirementsPoolView, local),
    canCreateRequirement: (local = true) => operations.allows(PM_ACTION.RequirementsPoolCreate, local),
    canUpdateRequirement: (local = true) => operations.allows(PM_ACTION.RequirementsPoolUpdate, local),
    canBatchMoveRequirement: (local = true) => operations.allows(PM_ACTION.RequirementsPoolBatchMove, local),
    canDeleteRequirement: (local = true) => operations.allows(PM_ACTION.RequirementsPoolDelete, local),
    canScheduleRequirement: (local = true) => operations.allows(PM_ACTION.RequirementsPoolSchedule, local),
    canValidateDocument: (local = true) => operations.allows(PM_ACTION.RequirementsPoolValidateDocument, local),
    canUploadDesignAsset: (local = true) => operations.allows(PM_ACTION.RequirementsPoolUploadDesignAsset, local),
    canAssignRoleOwners: (local = true) => operations.allows(PM_ACTION.RequirementsRoleOwnersAssign, local),
    canViewMaterials: (local = true) => operations.allows(PM_ACTION.RequirementsMaterialsView, local),
    canUploadMaterial: (local = true) => operations.allows(PM_ACTION.RequirementsMaterialsUpload, local),
    canDownloadMaterial: (local = true) => operations.allows(PM_ACTION.RequirementsMaterialsDownload, local),
    canDeleteMaterial: (local = true) => operations.allows(PM_ACTION.RequirementsMaterialsDelete, local),
  }), [operations.allows, operations.retry, operations.status])
}

export type TestingOperationAccess = DomainState & {
  canViewLibrary: Capability
  canUploadLibraryItem: Capability
  canViewVersionReports: Capability
  canUploadVersionReport: Capability
  canLinkVersionReport: Capability
  canDownloadVersionReport: Capability
}

export function useTestingOperationAccess(): TestingOperationAccess {
  const operations = useOperationCapabilities('testing')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewLibrary: (local = true) => operations.allows(PM_ACTION.TestingLibraryView, local),
    canUploadLibraryItem: (local = true) => operations.allows(PM_ACTION.TestingLibraryUpload, local),
    canViewVersionReports: (local = true) => operations.allows(PM_ACTION.TestingVersionReportsView, local),
    canUploadVersionReport: (local = true) => operations.allows(PM_ACTION.TestingVersionReportsUpload, local),
    canLinkVersionReport: (local = true) => operations.allows(PM_ACTION.TestingVersionReportsLink, local),
    canDownloadVersionReport: (local = true) => operations.allows(PM_ACTION.TestingVersionReportsDownload, local),
  }), [operations.allows, operations.retry, operations.status])
}

export type WorkflowOperationAccess = DomainState & {
  canViewTemplates: Capability
  canCreateTemplate: Capability
  canUpdateTemplate: Capability
  canValidateTemplate: Capability
  canPublishTemplate: Capability
  canViewInstances: Capability
  canStartInstance: Capability
  canActOnInstance: Capability
  canUpdateNodePlans: Capability
  canViewTestCaseMaterials: Capability
  canUploadTestCaseMaterial: Capability
  canGenerateTestCaseMaterial: Capability
  canDownloadTestCaseMaterial: Capability
  canViewMaterialProjections: Capability
  canViewOpinions: Capability
  canCreateOpinion: Capability
  canUpdateOpinion: Capability
  canUploadOpinionImage: Capability
}

export function useWorkflowOperationAccess(): WorkflowOperationAccess {
  const operations = useOperationCapabilities('workflow-management')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewTemplates: (local = true) => operations.allows(PM_ACTION.WorkflowsTemplatesView, local),
    canCreateTemplate: (local = true) => operations.allows(PM_ACTION.WorkflowsTemplatesCreate, local),
    canUpdateTemplate: (local = true) => operations.allows(PM_ACTION.WorkflowsTemplatesUpdate, local),
    canValidateTemplate: (local = true) => operations.allows(PM_ACTION.WorkflowsTemplatesValidate, local),
    canPublishTemplate: (local = true) => operations.allows(PM_ACTION.WorkflowsTemplatesPublish, local),
    canViewInstances: (local = true) => operations.allows(PM_ACTION.WorkflowsInstancesView, local),
    canStartInstance: (local = true) => operations.allows(PM_ACTION.WorkflowsInstancesStart, local),
    canActOnInstance: (local = true) => operations.allows(PM_ACTION.WorkflowsInstancesAct, local),
    canUpdateNodePlans: (local = true) => operations.allows(PM_ACTION.WorkflowsNodePlansUpdate, local),
    canViewTestCaseMaterials: (local = true) => operations.allows(PM_ACTION.WorkflowsTestCaseMaterialsView, local),
    canUploadTestCaseMaterial: (local = true) => operations.allows(PM_ACTION.WorkflowsTestCaseMaterialsUpload, local),
    canGenerateTestCaseMaterial: (local = true) => operations.allows(PM_ACTION.WorkflowsTestCaseMaterialsAiGenerate, local),
    canDownloadTestCaseMaterial: (local = true) => operations.allows(PM_ACTION.WorkflowsTestCaseMaterialsDownload, local),
    canViewMaterialProjections: (local = true) => operations.allows(PM_ACTION.WorkflowsMaterialProjectionsView, local),
    canViewOpinions: (local = true) => operations.allows(PM_ACTION.WorkflowsOpinionsView, local),
    canCreateOpinion: (local = true) => operations.allows(PM_ACTION.WorkflowsOpinionsCreate, local),
    canUpdateOpinion: (local = true) => operations.allows(PM_ACTION.WorkflowsOpinionsUpdate, local),
    canUploadOpinionImage: (local = true) => operations.allows(PM_ACTION.WorkflowsOpinionsUploadImage, local),
  }), [operations.allows, operations.retry, operations.status])
}

export type AIRunsOperationAccess = DomainState & {
  canViewCapabilities: Capability
  canViewRuns: Capability
  canCreateRun: Capability
  canApplyRun: Capability
  canSubmitFeedback: Capability
  canRetryRun: Capability
  canCancelRun: Capability
}

export function useAIRunsOperationAccess(): AIRunsOperationAccess {
  const operations = useOperationCapabilities('ai-runs')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewCapabilities: (local = true) => operations.allows(PM_ACTION.AiRunsCapabilitiesView, local),
    canViewRuns: (local = true) => operations.allows(PM_ACTION.AiRunsRunsView, local),
    canCreateRun: (local = true) => operations.allows(PM_ACTION.AiRunsRunsCreate, local),
    canApplyRun: (local = true) => operations.allows(PM_ACTION.AiRunsRunsApply, local),
    canSubmitFeedback: (local = true) => operations.allows(PM_ACTION.AiRunsRunsFeedback, local),
    canRetryRun: (local = true) => operations.allows(PM_ACTION.AiRunsRunsRetry, local),
    canCancelRun: (local = true) => operations.allows(PM_ACTION.AiRunsRunsCancel, local),
  }), [operations.allows, operations.retry, operations.status])
}

// Workflow task-plan nodes surface the tasks that already exist under the
// requirement; reading them goes through the same governed action as the
// work-items task table.
export type TaskTableViewAccess = DomainState & {
  canViewTaskTable: Capability
}

export function useTaskTableViewAccess(): TaskTableViewAccess {
  const operations = useOperationCapabilities('work-items')
  return useMemo(() => ({
    status: operations.status,
    retry: operations.retry,
    canViewTaskTable: (local = true) => operations.allows(PM_ACTION.WorkItemsTaskTableView, local),
  }), [operations.allows, operations.retry, operations.status])
}
