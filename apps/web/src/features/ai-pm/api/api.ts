// Re-export barrel — all imports from '../api/api' continue to work.
// Domain splits: pool.ts, ai.ts, workflow-queries.ts, materials.ts, workflow-mutations.ts.

export {
  useRequirementPool,
  useRequirementCover,
  useRequirementScope,
  useRequirement,
  useProjects,
  usePlanUnits,
  prefetchRequirement,
  useRequirementMutations,
  keepPreviousData,
} from './pool'

export {
  useAICapabilities,
  useAIRuns,
  useAIRun,
  useAIMutations,
  useCancelAIRun,
} from './ai'

export {
  useWorkflows,
  useRequirementWorkflowFilterNodes,
  useRequirementTemplateVersions,
  usePlanUnitIntakeRequirements,
  useWorkflow,
  useWorkflowAssignees,
  prefetchWorkflowInstance,
  useWorkflowInstance,
  useWorkflowInstances,
  useWorkflowTestCaseArtifacts,
  useWorkflowTestReportArtifacts,
  useTestingLibrary,
  useWorkflowNodeOpinions,
} from './workflow-queries'

export {
  downloadWorkflowNodeArtifact,
  downloadRequirementMaterial,
  downloadPlatformFile,
  useRequirementMaterials,
  useWorkflowMaterialProjections,
  useRequirementMaterialMutations,
  useTestingLibraryMutations,
} from './materials'

export {
  useWorkflowMutations,
} from './workflow-mutations'
