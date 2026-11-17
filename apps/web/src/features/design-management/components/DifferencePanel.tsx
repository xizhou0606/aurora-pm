import { useEffect, useMemo, useRef, useState } from 'react'
import {
  PhCheck as Check,
  PhCrosshair as LocateFixed,
  PhPencil as Edit3,
  PhPlus as Plus,
  PhProhibit as CircleOff,
  PhArrowCounterClockwise as RotateCcw,
  PhTrash as Trash2,
} from '@/shared/ui/icons'

import { copy } from '../copy'
import {
  AdaptiveFilterBar,
  AlertBanner,
  Button,
  ConfirmDialog,
  EmptyState,
  Input,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
  Textarea,
} from '@/shared/ui'
import {
  filterDifferences,
  severityTone,
  type DesignDifference,
  type DifferenceCategory,
  type DifferenceFilters,
  type DifferenceInput,
  type DifferenceSeverity,
  type Region,
  type VerificationStatus,
} from '../model'

const categories: DifferenceCategory[] = ['layout', 'size', 'typography', 'color', 'component', 'icon_image', 'content', 'effect']
const severities: DifferenceSeverity[] = ['critical', 'major', 'minor']
const verifications: VerificationStatus[] = ['pending', 'confirmed', 'false_positive']
const rechecks = ['new', 'still_present', 'uncertain', 'resolved', 'not_applicable'] as const

type OperationState = 'idle' | 'pending' | 'error'

type Props = {
  items: readonly DesignDifference[]
  filters: DifferenceFilters
  selectedId?: string
  canManage: boolean
  operationState: OperationState
  operationMessage?: string
  seedRegions?: { design?: Region; implementation?: Region }
  onSelect: (id: string) => void
  onFiltersChange: (filters: DifferenceFilters) => void
  onCreate: (input: DifferenceInput) => void
  onUpdate: (id: string, input: DifferenceInput) => void
  onDelete: (id: string) => void
  onVerify: (id: string, status: 'confirmed' | 'false_positive') => void
  onResolve: (id: string) => void
  onReopen: (id: string) => void
}

type Editor = { mode: 'create' | 'edit'; id?: string; draft: DifferenceInput }

const emptyDraft = (regions?: Props['seedRegions']): DifferenceInput => ({
  category: 'layout', severity: 'minor', title: '', description: '', recommendation: '', evidence_notes: '', confidence: 1,
  design_region: regions?.design,
  implementation_region: regions?.implementation,
})

export function DifferencePanel({
  items,
  filters,
  selectedId,
  canManage,
  operationState,
  operationMessage,
  seedRegions,
  onSelect,
  onFiltersChange,
  onCreate,
  onUpdate,
  onDelete,
  onVerify,
  onResolve,
  onReopen,
}: Props) {
  const [editor, setEditor] = useState<Editor>()
  const [deleteId, setDeleteId] = useState<string>()
  const [validation, setValidation] = useState('')
  const previousOperationState = useRef(operationState)
  const visible = useMemo(() => filterDifferences(items, filters), [filters, items])
  const edit = (item: DesignDifference) => setEditor({
    mode: 'edit', id: item.id,
    draft: {
      category: item.category,
      severity: item.severity,
      title: item.title,
      description: item.description,
      recommendation: item.recommendation,
      evidence_notes: item.evidence_notes,
      confidence: item.confidence,
      design_region: item.design_region,
      implementation_region: item.implementation_region,
    },
  })
  const save = () => {
    if (!editor?.draft.title.trim()) {
      setValidation(copy.designManagement.differences.required)
      return
    }
    setValidation('')
    if (editor.mode === 'create') onCreate(editor.draft)
    else if (editor.id) onUpdate(editor.id, editor.draft)
  }
  useEffect(() => {
    if (previousOperationState.current === 'pending' && operationState === 'idle') {
      setEditor(undefined)
      setDeleteId(undefined)
    }
    previousOperationState.current = operationState
  }, [operationState])
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col bg-card" aria-label={copy.designManagement.differences.title}>
      <header className="grid shrink-0 gap-2 border-b border-border p-3">
        <div className="flex items-center justify-between gap-2">
          <div><h2 className="text-sm font-semibold">{copy.designManagement.differences.title}</h2><span className="text-xs text-muted-foreground">{copy.designManagement.common.ratio(visible.length, items.length)}</span></div>
          {canManage ? <Button size="sm" onClick={() => setEditor({ mode: 'create', draft: emptyDraft(seedRegions) })}><Plus />{copy.designManagement.differences.add}</Button> : null}
        </div>
        <AdaptiveFilterBar aria-label={copy.designManagement.differences.filterLabel} controls={[
          { id: 'category', label: copy.designManagement.differences.category, node: (<FilterSelect label={copy.designManagement.differences.category} value={filters.category} options={categories.map((value) => [value, copy.designManagement.category[value]])} allLabel={copy.designManagement.differences.allCategories} onChange={(value) => onFiltersChange({ ...filters, category: value as DifferenceFilters['category'] })} />) },
          { id: 'severity', label: copy.designManagement.differences.severity, node: (<FilterSelect label={copy.designManagement.differences.severity} value={filters.severity} options={severities.map((value) => [value, copy.designManagement.severity[value]])} allLabel={copy.designManagement.differences.allSeverities} onChange={(value) => onFiltersChange({ ...filters, severity: value as DifferenceFilters['severity'] })} />) },
          { id: 'verification', label: copy.designManagement.differences.verification, node: (<FilterSelect label={copy.designManagement.differences.verification} value={filters.verification} options={verifications.map((value) => [value, copy.designManagement.verification[value]])} allLabel={copy.designManagement.differences.allVerifications} onChange={(value) => onFiltersChange({ ...filters, verification: value as DifferenceFilters['verification'] })} />) },
          { id: 'recheck', label: copy.designManagement.differences.recheck, node: (<FilterSelect label={copy.designManagement.differences.recheck} value={filters.recheck} options={rechecks.map((value) => [value, copy.designManagement.recheck[value]])} allLabel={copy.designManagement.differences.allRechecks} onChange={(value) => onFiltersChange({ ...filters, recheck: value as DifferenceFilters['recheck'] })} />) },
        ]} />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {visible.length === 0 ? <EmptyState title={copy.designManagement.differences.emptyTitle} /> : (
          <ol className="grid gap-2">
            {visible.map((item) => (
              <li key={item.id} className={item.id === selectedId ? 'rounded-lg border border-primary bg-selection p-3 shadow-[var(--design-shadow-card)]' : 'rounded-lg border border-border bg-card p-3 shadow-[var(--design-shadow-card)]'}>
                <button type="button" aria-pressed={item.id === selectedId} className="block w-full cursor-pointer rounded-md text-left transition-control outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background" onClick={() => onSelect(item.id)}>
                  <div className="flex min-w-0 items-start justify-between gap-2"><div className="min-w-0"><span className="text-xs text-muted-foreground">{copy.designManagement.common.metadata(copy.designManagement.differences.ordinal(item.ordinal), item.source === 'ai' ? copy.designManagement.differences.ai : item.source === 'manual' ? copy.designManagement.differences.manual : copy.designManagement.differences.historyRecovered)}</span><h3 className="mt-1 text-sm font-semibold">{item.title}</h3></div><StatusPill tone={severityTone(item.severity)}>{copy.designManagement.severity[item.severity]}</StatusPill></div>
                  <p className="mt-2 text-xs leading-5 text-muted-foreground">{item.description}</p>
                  <div className="mt-2 flex flex-wrap gap-1.5"><StatusPill tone={item.verification_status === 'false_positive' ? 'neutral' : item.verification_status === 'confirmed' ? 'success' : 'warning'}>{copy.designManagement.verification[item.verification_status]}</StatusPill><StatusPill tone={item.resolution_status === 'resolved' ? 'success' : 'neutral'}>{item.resolution_status === 'resolved' ? copy.designManagement.differences.resolved : copy.designManagement.differences.unresolved}</StatusPill>{item.recheck_status ? <StatusPill tone="info">{copy.designManagement.recheck[item.recheck_status]}</StatusPill> : null}</div>
                </button>
                <div className="mt-3 flex flex-wrap gap-1 border-t border-border pt-2">
                  <Button size="xs" variant="ghost" onClick={() => onSelect(item.id)}><LocateFixed />{copy.designManagement.differences.locate}</Button>
                  {item.can_edit ? <Button size="xs" variant="ghost" onClick={() => edit(item)}><Edit3 />{copy.designManagement.differences.edit}</Button> : null}
                  {item.can_verify ? <><Button size="xs" variant="ghost" aria-pressed={item.verification_status === 'confirmed'} onClick={() => onVerify(item.id, 'confirmed')}><Check />{copy.designManagement.differences.markValid}</Button><Button size="xs" variant="ghost" aria-pressed={item.verification_status === 'false_positive'} onClick={() => onVerify(item.id, 'false_positive')}><CircleOff />{copy.designManagement.differences.markFalsePositive}</Button></> : null}
                  {item.can_reopen ? <Button size="xs" variant="ghost" onClick={() => onReopen(item.id)}><RotateCcw />{copy.designManagement.differences.reopen}</Button> : null}
                  {item.can_resolve ? <Button size="xs" variant="ghost" onClick={() => onResolve(item.id)}><Check />{copy.designManagement.differences.resolve}</Button> : null}
                  {item.can_delete ? <Button size="icon-xs" variant="ghost" aria-label={copy.designManagement.common.delete} title={copy.designManagement.common.delete} onClick={() => setDeleteId(item.id)}><Trash2 /></Button> : null}
                </div>
              </li>
            ))}
          </ol>
        )}
      </div>
      <ResponsivePanel
        open={Boolean(editor)}
        onOpenChange={(open) => { if (!open && operationState !== 'pending') { setEditor(undefined); setValidation('') } }}
        title={editor?.mode === 'edit' ? copy.designManagement.differences.edit : copy.designManagement.differences.add}
        description={editor?.mode === 'edit' ? copy.designManagement.differences.editDescription : copy.designManagement.differences.createDescription}
        pending={operationState === 'pending'}
        footer={<><Button variant="outline" onClick={() => setEditor(undefined)}>{copy.designManagement.common.cancel}</Button><Button onClick={save}>{copy.designManagement.common.save}</Button></>}
      >
        {editor ? <DifferenceForm value={editor.draft} onChange={(draft) => setEditor({ ...editor, draft })} /> : null}
        {validation || operationMessage ? <AlertBanner className="mt-3" tone="error">{validation || operationMessage}</AlertBanner> : null}
      </ResponsivePanel>
      <ConfirmDialog open={Boolean(deleteId)} onOpenChange={(open) => { if (!open) setDeleteId(undefined) }} onConfirm={() => { if (deleteId) onDelete(deleteId) }} title={copy.designManagement.differences.deleteTitle} description={copy.designManagement.differences.deleteDescription} cancelLabel={copy.designManagement.common.cancel} confirmLabel={copy.designManagement.common.delete} pending={operationState === 'pending'} error={operationMessage} />
    </section>
  )
}

function DifferenceForm({ value, onChange }: { value: DifferenceInput; onChange: (value: DifferenceInput) => void }) {
  const update = <K extends keyof DifferenceInput>(key: K, next: DifferenceInput[K]) => onChange({ ...value, [key]: next })
  return <div className="grid gap-4">
    <div className="grid gap-3 sm:grid-cols-2"><SelectField label={copy.designManagement.differences.category} value={value.category} options={categories.map((id) => [id, copy.designManagement.category[id]])} onChange={(next) => update('category', next as DifferenceCategory)} /><SelectField label={copy.designManagement.differences.severity} value={value.severity} options={severities.map((id) => [id, copy.designManagement.severity[id]])} onChange={(next) => update('severity', next as DifferenceSeverity)} /></div>
    <TextField label={copy.designManagement.differences.titleField} value={value.title} onChange={(next) => update('title', next)} />
    <TextAreaField label={copy.designManagement.differences.description} value={value.description} onChange={(next) => update('description', next)} />
    <TextAreaField label={copy.designManagement.differences.recommendation} value={value.recommendation} onChange={(next) => update('recommendation', next)} />
    <TextAreaField label={copy.designManagement.differences.evidenceNotes} value={value.evidence_notes} onChange={(next) => update('evidence_notes', next)} />
    <RegionFields label={copy.designManagement.differences.designRegion} value={value.design_region} onChange={(next) => update('design_region', next)} />
    <RegionFields label={copy.designManagement.differences.implementationRegion} value={value.implementation_region} onChange={(next) => update('implementation_region', next)} />
  </div>
}

function RegionFields({ label, value, onChange }: { label: string; value?: Region; onChange: (value?: Region) => void }) {
  const enabled = Boolean(value)
  return <fieldset className="grid gap-2 rounded-md border border-border p-3"><div className="flex items-center justify-between gap-2"><legend className="text-sm font-medium">{label}</legend><Button type="button" size="xs" variant="outline" onClick={() => onChange(enabled ? undefined : { x: 0, y: 0, width: 1, height: 1 })}>{enabled ? copy.designManagement.common.delete : copy.designManagement.common.enabled}</Button></div>{value ? <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">{(['x', 'y', 'width', 'height'] as const).map((key) => <label key={key} className="grid gap-1 text-xs text-muted-foreground">{copy.designManagement.region[key]}<Input type="number" min={0} max={1} step={0.01} value={value[key]} onChange={(event) => onChange({ ...value, [key]: Number(event.target.value) })} /></label>)}</div> : <span className="text-xs text-muted-foreground">{copy.designManagement.differences.noRegion}</span>}</fieldset>
}

function FilterSelect({ label, value, options, allLabel, onChange }: { label: string; value: string; options: Array<readonly [string, string]>; allLabel: string; onChange: (value: string) => void }) {
  return <Select items={[{ value: 'all', label: allLabel }, ...options.map(([id, text]) => ({ value: id, label: text }))]} value={value} onValueChange={(next) => next && onChange(String(next))}><SelectTrigger aria-label={label}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{allLabel}</SelectItem>{options.map(([id, text]) => <SelectItem key={id} value={id}>{text}</SelectItem>)}</SelectContent></Select>
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: Array<readonly [string, string]>; onChange: (value: string) => void }) {
  return <div className="grid gap-1.5"><Label>{label}</Label><Select items={options.map(([id, text]) => ({ value: id, label: text }))} value={value} onValueChange={(next) => next && onChange(String(next))}><SelectTrigger aria-label={label}><SelectValue /></SelectTrigger><SelectContent>{options.map(([id, text]) => <SelectItem key={id} value={id}>{text}</SelectItem>)}</SelectContent></Select></div>
}

function TextField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <label className="grid gap-1.5 text-sm font-medium">{label}<Input value={value} maxLength={160} onChange={(event) => onChange(event.target.value)} /></label>
}

function TextAreaField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <label className="grid gap-1.5 text-sm font-medium">{label}<Textarea value={value} onChange={(event) => onChange(event.target.value)} /></label>
}
