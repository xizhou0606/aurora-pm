import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import {
  AlertBanner,
  Button,
  Checkbox,
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxInputGroup,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
  EmptyState,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  UserIdentity,
} from '@/shared/ui'

import {
  RESOURCE_ACCESS_SUBJECT_KINDS,
  type ResourceAccessEditorIntent,
  type ResourceAccessRule,
  type ResourceAccessSubjectKind,
  type ResourceAccessSubjectControl,
} from '../model'
import { AccessLevelSelect } from './AccessLevelSelect'

const subjectLabel = copy.subject.kinds

function RuleIdentity({ rule }: { rule: ResourceAccessRule }) {
  const name = rule.subject?.name ?? copy.subject.unknown(rule.subjectId)
  if (rule.subjectKind === 'user') {
    return (
      <UserIdentity
        name={name}
        secondary={rule.subject?.secondary}
      />
    )
  }
  return (
    <div className="min-w-0">
      <p className="m-0 truncate text-sm font-medium text-foreground">{name}</p>
      <p className="m-0 text-xs text-muted-foreground">{subjectLabel[rule.subjectKind]}</p>
    </div>
  )
}

function SubjectPicker({
  control,
  disabled,
}: {
  control: ResourceAccessSubjectControl
  disabled: boolean
}) {
  const available = control.capabilities[control.kind].available
  return (
    <div className="grid gap-3 rounded-lg border border-border bg-muted/30 p-3">
      <div className="grid gap-3">
        <div className="grid gap-2">
          <Label htmlFor="resource-access-subject-kind">{copy.subject.kindLabel}</Label>
          <Select
            value={control.kind}
            disabled={disabled}
            onValueChange={(value) => {
              if (RESOURCE_ACCESS_SUBJECT_KINDS.includes(value as ResourceAccessSubjectKind)) {
                control.onKindChange(value as ResourceAccessSubjectKind)
              }
            }}
          >
            <SelectTrigger id="resource-access-subject-kind"><SelectValue>{subjectLabel[control.kind]}</SelectValue></SelectTrigger>
            <SelectContent>
              {RESOURCE_ACCESS_SUBJECT_KINDS.map((kind) => (
                <SelectItem key={kind} value={kind} disabled={!control.capabilities[kind].available}>
                  {subjectLabel[kind]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {!available ? <AlertBanner tone="warning">{copy.subject.unavailable}</AlertBanner> : null}
      {control.status === 'error' ? <AlertBanner tone="error">{copy.subject.loadError}</AlertBanner> : null}
      <div className="grid gap-2">
        <Label htmlFor="resource-access-subject">{copy.subject.candidateLabel}</Label>
        <Combobox
          value={control.candidates.find((candidate) => candidate.id === control.selectedId) ?? null}
          items={control.candidates}
          disabled={disabled || !available || control.status !== 'ready'}
          openOnInputClick
          filter={null}
          itemToStringLabel={(candidate) => candidate.name}
          isItemEqualToValue={(left, right) => left.kind === right.kind && left.id === right.id}
          onInputValueChange={(value, details) => {
            if (details.reason !== 'item-press') control.onQueryChange(value)
          }}
          onValueChange={(candidate) => control.onSelect(candidate?.id)}
        >
          <ComboboxInputGroup>
            <ComboboxInput
              id="resource-access-subject"
              aria-label={copy.subject.searchLabel}
              placeholder={copy.subject.searchPlaceholder}
            />
            <ComboboxTrigger aria-label={copy.subject.candidateLabel} />
          </ComboboxInputGroup>
          <ComboboxContent>
            <ComboboxEmpty>{copy.subject.emptyCandidates}</ComboboxEmpty>
            <ComboboxList
              onScroll={(event) => {
                if (!control.hasMore || control.loadingMore || !control.onLoadMore) return
                const list = event.currentTarget
                if (list.scrollHeight - list.scrollTop - list.clientHeight < 48) control.onLoadMore()
              }}
            >
              {(candidate) => (
                <ComboboxItem key={candidate.id} value={candidate}>
                  {candidate.kind === 'user'
                    ? <UserIdentity name={candidate.name} secondary={candidate.secondary} size="sm" />
                    : candidate.name}
                </ComboboxItem>
              )}
            </ComboboxList>
          </ComboboxContent>
        </Combobox>
      </div>
      <Button
        type="button"
        variant="outline"
        disabled={disabled || !available || !control.selectedId}
        onClick={control.onAdd}
      >
        {copy.subject.add}
      </Button>
    </div>
  )
}

export function SubjectRuleEditor({
  rules,
  control,
  disabled,
  onEdit,
}: {
  rules: readonly ResourceAccessRule[]
  control: ResourceAccessSubjectControl
  disabled: boolean
  onEdit: (action: ResourceAccessEditorIntent) => void
}) {
  return (
    <section className="grid gap-3" aria-label={copy.subject.sectionTitle}>
      {!control.capabilities.organization_unit.available ? (
        <AlertBanner tone="warning">{copy.subject.organizationUnavailable}</AlertBanner>
      ) : null}
      <SubjectPicker control={control} disabled={disabled} />
      {rules.length === 0 ? (
        <EmptyState title={copy.subject.emptyRules} name="hugeicons:user-multiple" />
      ) : (
        <div className="grid gap-2">
          {rules.map((rule) => (
            <div
              key={`${rule.subjectKind}:${rule.subjectId}`}
              className="grid gap-3 rounded-lg border border-border bg-card p-3 sm:grid-cols-[minmax(0,1fr)_10rem_auto] sm:items-end"
            >
              <div className="grid gap-2">
                <RuleIdentity rule={rule} />
                {rule.subjectKind === 'organization_unit' ? (
                  <Label className="flex items-center gap-2 text-sm font-normal">
                    <Checkbox
                      checked={rule.includeDescendants}
                      disabled={disabled}
                      onCheckedChange={(checked) => onEdit({
                        type: 'set_rule_descendants',
                        subjectKind: rule.subjectKind,
                        subjectId: rule.subjectId,
                        included: checked === true,
                      })}
                    />
                    {copy.subject.includeDescendants}
                  </Label>
                ) : null}
              </div>
              <AccessLevelSelect
                id={`resource-access-rule-${rule.subjectKind}-${rule.subjectId}`}
                value={rule.level}
                disabled={disabled}
                onChange={(level) => onEdit({
                  type: 'set_rule_level',
                  subjectKind: rule.subjectKind,
                  subjectId: rule.subjectId,
                  level,
                })}
              />
              <Button
                type="button"
                variant="ghost"
                disabled={disabled}
                onClick={() => onEdit({
                  type: 'remove_rule',
                  subjectKind: rule.subjectKind,
                  subjectId: rule.subjectId,
                })}
              >
                {copy.subject.remove}
              </Button>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
