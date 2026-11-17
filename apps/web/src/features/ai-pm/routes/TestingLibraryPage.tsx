import { useEffect, useMemo, useState } from 'react'
import { PhArrowCounterClockwise } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import type { PMScopeSelection } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  Button,
  AdaptiveFilterBar,
  Input,
  LazyFileUploader,
  OperationalPage,
  PMScopeCascader,
  PMScopeCascaderField,
  SearchInput,
  ResponsivePanel,
  ViewTabs,
  type PMScopeCascaderLabels,
  type PMScopeCascaderState,
} from '@/shared/ui'
import { useRequirementScope, useTestingLibrary, useTestingLibraryMutations } from '../api/api'
import { TestingLibraryCards } from '../components/TestingLibraryCards'
import type { TestingLibraryKind } from '../model/types'
import { isHttpURL } from '../utils/workflow-artifact-file'
import { useTestingOperationAccess } from '../hooks/useOperationAccess'

const libraryCopy = copy.aiPM.testingLibrary
const poolCopy = copy.aiPM.requirementPool
const emptyUploadScope: PMScopeSelection = { projectSetIds: [], projectIds: [], requirementIds: [] }
const scopeLabels = {
  label: libraryCopy.scopeFilter,
  hierarchyHint: poolCopy.scopeHierarchyHint,
  projectSet: poolCopy.projectLevel,
  project: poolCopy.planUnitLevel,
  requirement: poolCopy.requirement,
  selectProjectSetFirst: poolCopy.chooseProjectFirst,
  selectProjectFirst: poolCopy.chooseProjectFirst,
} satisfies PMScopeCascaderLabels
const requirementFieldLabels = {
  ...scopeLabels,
  label: libraryCopy.chooseRequirement,
} satisfies PMScopeCascaderLabels

type LibraryFilters = {
  kind: TestingLibraryKind
  q: string
  projectId: number
  planUnit: string
}

const emptyFilters: LibraryFilters = {
  kind: 'test_case',
  q: '',
  projectId: 0,
  planUnit: '',
}

export function TestingLibraryPage() {
  'use no memo'
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const access = useTestingOperationAccess()
  const [filters, setFilters] = useState<LibraryFilters>(emptyFilters)
  const [casePanelOpen, setCasePanelOpen] = useState(false)
  const [uploadScope, setUploadScope] = useState<PMScopeSelection>(emptyUploadScope)
  const [reportLink, setReportLink] = useState('')
  const scope = useRequirementScope(workspaceId)
  const selectedProject = useMemo(() => (
    scope.data?.projectSets.find((item) => item.id === filters.projectId)
  ), [filters.projectId, scope.data?.projectSets])
  const selectedPlanUnit = useMemo(() => {
    const id = Number(filters.planUnit)
    if (!Number.isSafeInteger(id) || id <= 0 || !selectedProject) return undefined
    return scope.data?.projects.find((item) => item.id === id && item.parentId === selectedProject.id)
  }, [filters.planUnit, scope.data?.projects, selectedProject])
  const query = useTestingLibrary(workspaceId, {
    kind: filters.kind,
    q: filters.q.trim() || undefined,
    projectId: selectedProject?.id,
    planUnitId: selectedPlanUnit?.id,
  })
  const mutations = useTestingLibraryMutations(workspaceId)
  const uploadRequirementId = uploadScope.requirementIds[0] ?? 0
  const uploadRequirement = scope.data?.requirements.find((item) => item.id === uploadRequirementId)
  const trimmedLink = reportLink.trim()
  const linkValid = !trimmedLink || isHttpURL(trimmedLink)
  const canViewCases = access.canViewLibrary()
  const canViewReports = access.canViewVersionReports()
  const canUploadReportFile = access.canUploadVersionReport()
  const canLinkReport = access.canLinkVersionReport()
  const canUploadCase = uploadRequirementId > 0
  const reportScopeAvailable = Boolean(selectedPlanUnit)
  const scopeState: PMScopeCascaderState = scope.isPending
    ? { status: 'loading' }
    : scope.isError || !scope.data
      ? { status: 'error', onRetry: () => void scope.refetch() }
      : { status: 'ready', options: scope.data }
  const filterScope: PMScopeSelection = {
    projectSetIds: selectedProject ? [selectedProject.id] : [],
    projectIds: selectedPlanUnit ? [selectedPlanUnit.id] : [],
    requirementIds: [],
  }
  const hasFilters = Boolean(filters.q || selectedProject || selectedPlanUnit)
  const openCasePanel = () => {
    setUploadScope({
      projectSetIds: selectedProject ? [selectedProject.id] : [],
      projectIds: selectedPlanUnit ? [selectedPlanUnit.id] : [],
      requirementIds: [],
    })
    setCasePanelOpen(true)
  }
  useEffect(() => {
    if (access.status !== 'ready') return
    if (filters.kind === 'test_case' && !canViewCases && canViewReports) {
      setFilters((current) => ({ ...current, kind: 'test_report' }))
    } else if (filters.kind === 'test_report' && !canViewReports && canViewCases) {
      setFilters((current) => ({ ...current, kind: 'test_case' }))
    }
  }, [access.status, canViewCases, canViewReports, filters.kind])
  if (access.status !== 'ready' || (!canViewCases && !canViewReports)) return null
  return (
    <OperationalPage
      title={libraryCopy.title}
      actions={(
        <PMScopeCascader
          idPrefix="testing-library-scope"
          className="w-48 max-w-full"
          state={scopeState}
          labels={scopeLabels}
          value={filterScope}
          summary={selectedProject
            ? poolCopy.scopeSummary(selectedProject.label, selectedPlanUnit?.label ?? poolCopy.allSchedules)
            : libraryCopy.scopeAll}
          levels="project-set-project"
          selectionMode="single-requirement"
          presentation="control"
          onChange={(selection) => {
            setFilters((current) => ({
              ...current,
              projectId: selection.projectSetIds[0] ?? 0,
              planUnit: selection.projectIds[0] ? String(selection.projectIds[0]) : '',
            }))
          }}
        />
      )}
    >
      <div className="grid min-h-full min-w-0 content-start gap-4 p-3 sm:p-5">
        <AdaptiveFilterBar
          aria-label={libraryCopy.filters}
          controls={[
            { id: 'search', label: libraryCopy.search, placement: 'search', node: (
              <SearchInput
                  name="testing-search"
                  aria-label={libraryCopy.search}
                  placeholder={libraryCopy.searchPlaceholder}
                  value={filters.q}
                  onChange={(event) => setFilters((current) => ({ ...current, q: event.target.value }))}
                />
            ) },

          ]}
          leading={(<ViewTabs
                density="compact"
                label={libraryCopy.typeFilter}
                value={filters.kind}
                onValueChange={(value) => {
                  if (value === 'test_report') setCasePanelOpen(false)
                  setFilters((current) => ({
                    ...current,
                    kind: value === 'test_report' ? 'test_report' : 'test_case',
                  }))
                }}
                tabs={[
                  ...(canViewCases ? [{ id: 'test_case' as const, label: libraryCopy.testCases }] : []),
                  ...(canViewReports ? [{ id: 'test_report' as const, label: libraryCopy.testReports }] : []),
                ]}
              />)}
          endActions={(
            <>
              {filters.kind === 'test_case' && access.canUploadLibraryItem() ? (
                <Button type="button" size="sm" onClick={openCasePanel}>
                  {libraryCopy.uploadCase}
                </Button>
              ) : null}
              {hasFilters ? (
                <Button type="button" variant="ghost" size="icon-sm" aria-label={libraryCopy.reset} onClick={() => {
                  setFilters((current) => ({ ...emptyFilters, kind: current.kind }))
                }}>
                  <PhArrowCounterClockwise />
                </Button>
              ) : null}
            </>
          )}
        />
        {filters.kind === 'test_report' && (canUploadReportFile || canLinkReport) ? (
          <section className="grid gap-3 rounded-lg border border-border bg-card p-4">
            {!reportScopeAvailable ? <p className="m-0 text-xs text-muted-foreground">{libraryCopy.choosePlanUnitRequired}</p> : null}
            {canUploadReportFile ? (
              <LazyFileUploader
                label={libraryCopy.uploadReport}
                inputLabel={libraryCopy.uploadReport}
                hint={libraryCopy.uploadReportHint}
                accept=".pdf,.doc,.docx,.xlsx,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                maxBytes={20 * 1024 * 1024}
                multiple
                disabled={!reportScopeAvailable}
                pending={mutations.create.isPending}
                onUpload={(file) => mutations.create.mutateAsync({ kind: 'test_report', planUnitId: selectedPlanUnit?.id, file })}
              />
            ) : null}
            {canLinkReport && reportScopeAvailable ? (
              <>
                <label className="grid gap-1.5 text-xs font-medium text-foreground">
                  <span>{libraryCopy.reportLink}</span>
                  <Input
                    name="testing-report-link"
                    type="url"
                    aria-label={libraryCopy.reportLink}
                    aria-invalid={!linkValid || undefined}
                    value={reportLink}
                    placeholder={libraryCopy.reportLinkPlaceholder}
                    onChange={(event) => setReportLink(event.target.value)}
                  />
                </label>
                {trimmedLink && linkValid ? (
                  <Button
                    size="sm"
                    variant="outline"
                    className="justify-self-start"
                    disabled={mutations.create.isPending}
                    onClick={() => {
                      void mutations.create.mutateAsync({ kind: 'test_report', planUnitId: selectedPlanUnit?.id, url: trimmedLink }).then(() => setReportLink(''))
                    }}
                  >
                    {libraryCopy.saveReportLink}
                  </Button>
                ) : null}
              </>
            ) : null}
          </section>
        ) : null}
        <TestingLibraryCards
          rows={query.data ?? []}
          pending={query.isPending}
          error={query.isError}
          kind={filters.kind}
          canOpen={filters.kind !== 'test_report' || access.canDownloadVersionReport()}
          onRetry={() => void query.refetch()}
        />
      </div>
      <ResponsivePanel
        open={casePanelOpen && access.canUploadLibraryItem()}
        onOpenChange={setCasePanelOpen}
        title={libraryCopy.uploadCase}
        description={libraryCopy.uploadCaseDescription}
        className="max-w-lg"
      >
        <div className="grid gap-3">
          <PMScopeCascaderField
            idPrefix="testing-library-upload-requirement"
            state={scopeState}
            labels={requirementFieldLabels}
            value={uploadScope}
            summary={uploadRequirement?.label ?? libraryCopy.chooseRequirementPlaceholder}
            selectionMode="single-requirement"
            onChange={setUploadScope}
          />
          {!canUploadCase ? <p className="m-0 text-xs text-muted-foreground">{libraryCopy.chooseRequirementRequired}</p> : null}
          <LazyFileUploader
            label={libraryCopy.uploadCase}
            inputLabel={libraryCopy.uploadCase}
            hint={libraryCopy.uploadCaseHint}
            accept=".csv,.xls,.xlsx,.pdf,text/csv,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/pdf"
            maxBytes={20 * 1024 * 1024}
            multiple
            disabled={!access.canUploadLibraryItem(canUploadCase)}
            pending={mutations.create.isPending}
            onUpload={(file) => mutations.create.mutateAsync({ kind: 'test_case', requirementId: uploadRequirementId, file }).then(() => setCasePanelOpen(false))}
          />
        </div>
      </ResponsivePanel>
    </OperationalPage>
  )
}
