import { useMemo, type ReactNode } from 'react'
import { PhCaretDown } from '@/shared/ui/icons'
import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { applyTableColumnPreference, type TableColumnPreference } from '@/shared/model/table-column-preferences'
import {
  Button,
  ClearControlButton,
  AdaptiveFilterBar,
  MultiUserSelect,
  PMScopeCascader,
  Popover,
  PopoverContent,
  PopoverTrigger,
  SearchInput,
  Select,
  SelectContent,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
  type PMScopeCascaderLabels,
  type PMScopeCascaderState,
} from '@/shared/ui'
import type { UserSelectOption } from '@/shared/ui/user/user-select-option'
import { useWorkflowAssignees } from '../api/api'
import {
  REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
  REQUIREMENT_POOL_NODE_COMPLETED,
  REQUIREMENT_POOL_NODE_IDLE,
  REQUIREMENT_POOL_UNSCHEDULED,
  applyRequirementPoolScopeSelection,
  requirementPoolFilterHasValue,
  requirementPoolScopeSelection,
  toggleRequirementPoolUnscheduled,
  type RequirementPoolFilterControlId,
  type RequirementPoolFilterValue,
} from '../utils/requirement-pool-filter-fields'
import type { WorkflowAssignee } from '../model/types'

const poolCopy = copy.aiPM.requirementPool
const emptyScopeOptions: PMScopeOptions = { projectSets: [], projects: [], requirements: [] }
const scopeLabels = {
  label: poolCopy.scopeFilter, hierarchyHint: poolCopy.scopeHierarchyHint,
  projectSet: poolCopy.projectLevel, project: poolCopy.planUnitLevel,
  requirement: poolCopy.requirement,
  selectProjectSetFirst: poolCopy.chooseProjectFirst, selectProjectFirst: poolCopy.chooseProjectFirst,
} satisfies PMScopeCascaderLabels
const defaultControlPreference: TableColumnPreference<RequirementPoolFilterControlId> = {
  order: REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
  hidden: [],
}
const roleFilterIds = ['role-product', 'role-ui', 'role-frontend', 'role-backend', 'role-tester'] as const

export type RequirementPoolNodeOption = { id: string; name: string }

type RequirementPoolFilterBarProps = {
  value: RequirementPoolFilterValue
  nodeOptions: readonly RequirementPoolNodeOption[]
  people: readonly WorkflowAssignee[]
  peoplePending?: boolean
  controlPreference?: TableColumnPreference<RequirementPoolFilterControlId>
  actions?: ReactNode
  onChange: (value: RequirementPoolFilterValue) => void
  onReset: () => void
}

function scopeOptionsOf(state: PMScopeCascaderState): PMScopeOptions {
  return state.status === 'ready' ? state.options : emptyScopeOptions
}

function personOptions(people: readonly WorkflowAssignee[]): UserSelectOption[] {
  return people.map((person) => ({
    id: person.id,
    name: person.display_name?.trim() || person.username,
    username: person.username,
    avatarUrl: person.avatar_url,
  }))
}

const roleOwnerKey = {
  'role-product': 'productOwnerUserIds',
  'role-ui': 'designOwnerUserIds',
  'role-frontend': 'frontendOwnerUserIds',
  'role-backend': 'backendOwnerUserIds',
  'role-tester': 'testerUserIds',
} as const

function withOwners(
  fieldId: keyof typeof roleOwnerKey,
  filters: RequirementPoolFilterValue,
  ids: number[],
): RequirementPoolFilterValue {
  return { ...filters, [roleOwnerKey[fieldId]]: ids }
}

function scopeSummary(filters: RequirementPoolFilterValue, options: PMScopeOptions) {
  const selectedUnits = new Set(filters.planUnitIds.filter((id) => id !== REQUIREMENT_POOL_UNSCHEDULED))
  const fullSets = filters.projectIds.filter((setId) => {
    const children = options.projects.filter((item) => item.parentId === setId)
    return children.length === 0 || children.every((item) => selectedUnits.has(String(item.id)))
  })
  const labels = [
    ...fullSets.map((id) => (
      options.projectSets.find((option) => option.id === id)?.label || poolCopy.unknownProject(id)
    )),
    ...filters.planUnitIds.flatMap((id) => {
      if (id === REQUIREMENT_POOL_UNSCHEDULED) return [poolCopy.unscheduled]
      if (fullSets.some((setId) => options.projects.find((item) => item.id === Number(id))?.parentId === setId)) {
        return []
      }
      return [options.projects.find((option) => option.id === Number(id))?.label || poolCopy.unknownPlanUnit(Number(id))]
    }),
  ]
  return labels.length ? poolCopy.filterMore(labels[0], labels.length - 1) : ''
}

export function RequirementPoolScopeControl({ value, scopeState, onChange }: {
  value: RequirementPoolFilterValue; scopeState: PMScopeCascaderState
  onChange: (value: RequirementPoolFilterValue) => void
}) {
  const summary = scopeSummary(value, scopeOptionsOf(scopeState))
  return (
    <Popover>
      <PopoverTrigger
        type="button"
        aria-label={poolCopy.scopeLevel}
        className={cn(
          'flex h-8 w-44 max-w-full min-w-0 cursor-pointer items-center justify-between gap-2 rounded-md border border-input bg-background px-2.5 py-1 text-sm outline-none',
          'hover:not-data-disabled:bg-muted data-popup-open:bg-muted focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
        )}
        data-slot="select-trigger"
      >
        <span className={cn('min-w-0 flex-1 truncate text-left', summary ? 'text-foreground' : 'text-muted-foreground')}>
          {summary || poolCopy.scopeLevel}
        </span>
        <PhCaretDown aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent className="w-[min(36rem,calc(100vw-1.5rem))] p-0" align="start">
        <div className="border-b border-border p-1">
          <Button
            type="button"
            variant="ghost"
            role="checkbox"
            aria-checked={value.planUnitIds.includes(REQUIREMENT_POOL_UNSCHEDULED)}
            className="h-auto w-full justify-start rounded-md px-2.5 py-1.5 text-sm font-normal"
            onClick={() => onChange(toggleRequirementPoolUnscheduled(value))}
          >
            {poolCopy.unscheduled}
          </Button>
        </div>
        <PMScopeCascader
          idPrefix="requirement-pool-scope"
          presentation="embedded"
          state={scopeState}
          value={requirementPoolScopeSelection(value)}
          labels={scopeLabels}
          levels="project-set-project"
          selectionMode="multiple"
          onChange={(selection: PMScopeSelection) => {
            onChange(applyRequirementPoolScopeSelection(value, selection))
          }}
        />
      </PopoverContent>
    </Popover>
  )
}

function NodeFilterControl({
  value,
  nodeOptions,
  onChange,
}: {
  value: RequirementPoolFilterValue
  nodeOptions: readonly RequirementPoolNodeOption[]
  onChange: (value: RequirementPoolFilterValue) => void
}) {
  const options = useMemo(() => [
    ...nodeOptions.map((option) => ({ value: option.id, label: option.name })),
    { value: REQUIREMENT_POOL_NODE_IDLE, label: poolCopy.workflowNotStarted },
    { value: REQUIREMENT_POOL_NODE_COMPLETED, label: poolCopy.workflowCompleted },
  ], [nodeOptions])
  return (
    <div className="relative w-full min-w-0">
      <Select<string, true>
        multiple
        value={[...value.currentNodeIds]}
        items={options}
        onValueChange={(next) => onChange({ ...value, currentNodeIds: next.map(String) })}
      >
        <SelectTrigger aria-label={poolCopy.workflowOverview} className="w-full min-w-0">
          <SelectValue
            placeholder={poolCopy.workflowOverview}
            className={cn(value.currentNodeIds.length && 'pr-7')}
          />
        </SelectTrigger>
        <SelectContent>
          <SelectLabel>{poolCopy.workflowOverview}</SelectLabel>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      {value.currentNodeIds.length ? (
        <ClearControlButton
          label={copy.ui.clearSelection}
          onClick={() => onChange({ ...value, currentNodeIds: [] })}
        />
      ) : null}
    </div>
  )
}

export function RequirementPoolFilterBar({
  value,
  nodeOptions,
  people,
  peoplePending = false,
  controlPreference = defaultControlPreference,
  actions,
  onChange,
  onReset,
}: RequirementPoolFilterBarProps) {
  const users = personOptions(people)
  const usersUnavailable = peoplePending && people.length === 0
  const canReset = requirementPoolFilterHasValue(value)
  const controlRegistry: readonly { id: RequirementPoolFilterControlId; node: ReactNode }[] = [
    {
      id: 'search',
      node: (
        <label key="search" className="relative block w-full min-w-0">
          <span className="sr-only">{poolCopy.search}</span>
          <SearchInput
            name="requirement-search"
            aria-label={poolCopy.search}
            value={value.q}
            clearable
            placeholder={poolCopy.searchPlaceholder}
            className="h-8 w-full"
            onClear={() => onChange({ ...value, q: '' })}
            onChange={(event) => onChange({ ...value, q: event.currentTarget.value })}
          />
        </label>
      ),
    },
    {
      id: 'workflow',
      node: <NodeFilterControl key="workflow" value={value} nodeOptions={nodeOptions} onChange={onChange} />,
    },
    ...roleFilterIds.map((id) => {
      const label = REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.find((option) => option.id === id)?.label ?? id
      const selected = users.filter((option) => value[roleOwnerKey[id]].includes(option.id))
      return {
        id,
        node: (
          <div key={id} className="w-full min-w-0">
            <MultiUserSelect
              id={`requirement-pool-${id}`}
              label={label}
              value={selected}
              options={users}
              placeholder={label}
              className="w-full"
              contentClassName="min-w-56"
              selectionDisplay="inline"
              clearable
              disabled={usersUnavailable}
              onValueChange={(next) => onChange(withOwners(id, value, next.map((option) => option.id)))}
            />
          </div>
        ),
      }
    }),
  ]
  return (
    <AdaptiveFilterBar
      maxColumns={7}
      aria-label={poolCopy.filters}
      controls={applyTableColumnPreference(controlRegistry, controlPreference).map((control) => ({
        ...control,
        label: REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.find((option) => option.id === control.id)!.label,
        placement: control.id === 'search' ? 'search' : 'field',
      }))}
      endActions={(
        <>
          {actions}
          {canReset ? (
            <Button type="button" variant="outline" onClick={onReset}>
              {poolCopy.reset}
            </Button>
          ) : null}
        </>
      )}
    />
  )
}

export function RequirementPoolFilterBarDirectory({
  workspaceId,
  ...props
}: Omit<
  RequirementPoolFilterBarProps,
  'people' | 'peoplePending'
> & { workspaceId: number }) {
  const directory = useWorkflowAssignees(workspaceId)

  return (
    <RequirementPoolFilterBar
      {...props}
      people={directory.data ?? []}
      peoplePending={directory.isPending}
    />
  )
}
