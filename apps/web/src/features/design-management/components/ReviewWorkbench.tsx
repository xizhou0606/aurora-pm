import { useEffect, useMemo, useReducer, useState } from 'react'
import {
  PhArrowsClockwise as RefreshCw,
  PhArrowsOut as Maximize2,
  PhCirclesThreePlus as Blend,
  PhClockCounterClockwise as History,
  PhColumns as Columns2,
  PhDownloadSimple as ArrowDownToLine,
  PhFileMagnifyingGlass as ScanSearch,
  PhFire as Flame,
  PhFunnel as ListFilter,
  PhMagnifyingGlassMinus as ZoomOut,
  PhMagnifyingGlassPlus as ZoomIn,
  PhRuler as Ruler,
  PhSparkle as Sparkles,
  PhTrash as Trash2,
} from '@/shared/ui/icons'

import { copy } from '../copy'
import {
  AiLoadingOverlay,
  AlertBanner,
  Button,
  Checkbox,
  ConfirmDialog,
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
  ViewTabs,
} from '@/shared/ui'
import { DifferencePanel } from './DifferencePanel'
import { RecheckDecisionPanel } from './RecheckDecisionPanel'
import { ReviewCanvas } from './ReviewCanvas'
import {
  filterDifferences,
  initialWorkbenchState,
  reviewConclusionPresentation,
  reviewStatusPresentation,
  workbenchReducer,
  type DesignDifference,
  type DesignReviewDetail,
  type DeepReviewJob,
  type DifferenceFilters,
  type DifferenceInput,
  type HistoricalRecheckStatus,
  type Region,
} from '../model'

type OperationState = 'idle' | 'pending' | 'error'

type Props = {
  review: DesignReviewDetail
  designURL?: string
  implementationURL?: string
  heatmapURL?: string
  assetState: OperationState
  assetMessage?: string
  differenceState: OperationState
  differenceMessage?: string
  analysisState: OperationState
  analysisMessage?: string
  canCreate: boolean
  canManageDifferences: boolean
  canManageAnalysis: boolean
  canDelete: boolean
  onRoundChange: (id: string) => void
  onRecheck: () => void
  onRetryAnalysis: () => void
  onAbandonAnalysis: () => void
  onDeepReview: (designRegion: Region, implementationRegion: Region, focus: string) => void
  onRetryDeepReview: (id: string) => void
  onCancelDeepReview: (id: string) => void
  onExport: (format: 'markdown' | 'json') => void
  onDelete: () => void
  onCreateDifference: (input: DifferenceInput) => void
  onUpdateDifference: (id: string, input: DifferenceInput) => void
  onDeleteDifference: (id: string) => void
  onVerifyDifference: (id: string, status: 'confirmed' | 'false_positive') => void
  onResolveDifference: (id: string) => void
  onReopenDifference: (id: string) => void
  onResolveRecheckMatch: (id: string, resolution: 'inherit' | 'new') => void
  onOverrideRecheckVerification: (id: string, status: HistoricalRecheckStatus | null) => void
}

export function ReviewWorkbench(props: Props) {
  const { review } = props
  const [state, dispatch] = useReducer(workbenchReducer, {
    ...initialWorkbenchState,
    selectedDifferenceId: review.differences[0]?.id,
  })
  const [showAll, setShowAll] = useState(true)
  const [filters, setFilters] = useState<DifferenceFilters>({ category: 'all', severity: 'all', verification: 'all', recheck: 'all' })
  const [overlayOpacity, setOverlayOpacity] = useState(50)
  const [deepOpen, setDeepOpen] = useState(false)
  const [recheckOpen, setRecheckOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const visibleDifferences = useMemo(
    () => filterDifferences(review.differences, filters),
    [filters, review.differences],
  )
  const selected = visibleDifferences.find((item) => item.id === state.selectedDifferenceId)
  const marked = showAll ? visibleDifferences : selected ? [selected] : []
  const designMarkers = useMemo(() => regions(marked, 'design_region'), [marked])
  const implementationMarkers = useMemo(() => regions(marked, 'implementation_region'), [marked])
  const status = reviewStatusPresentation(review.status)
  const conclusion = reviewConclusionPresentation(review.conclusion)
  const canvasMode = review.dimensions_match ? state.canvasMode : 'compare'
  const selectDifference = (id: string) => dispatch({ type: 'select-difference', id })
  const selectionReady = Boolean(state.selections.design && state.selections.implementation)
  const pendingRecheckCount = review.pending_recheck_matches.length + review.recheck_verifications.filter((item) => item.effective_status === 'uncertain' || (item.effective_status === 'still_present' && !item.current_difference_id)).length
  useEffect(() => {
    if (!state.selectedDifferenceId || !visibleDifferences.some((item) => item.id === state.selectedDifferenceId)) {
      dispatch({ type: 'select-difference', id: visibleDifferences[0]?.id })
    }
  }, [state.selectedDifferenceId, visibleDifferences])
  const renderCanvas = (side: 'design' | 'implementation') => {
    const design = side === 'design'
    return <ReviewCanvas
      side={side}
      label={design ? copy.designManagement.detail.designCanvas : copy.designManagement.detail.implementationCanvas}
      imageURL={design ? props.designURL : props.implementationURL}
      width={design ? review.design_width || review.viewport_width : review.viewport_width}
      height={design ? review.design_height || review.viewport_height : review.viewport_height}
      markers={design ? designMarkers : implementationMarkers}
      selectedId={state.selectedDifferenceId}
      selection={state.selections[side]}
      zoom={state.zoom}
      rulers={state.rulers}
      horizontalGuides={state.guides.horizontal}
      verticalGuides={state.guides[side]}
      onSelect={selectDifference}
      onSelectRegion={(target, region) => dispatch({ type: 'select-region', side: target, region })}
      onAddGuide={(target, axis, coordinate) => dispatch({ type: 'add-guide', side: target, axis, coordinate })}
      onRemoveGuide={(target, axis, coordinate) => dispatch({ type: 'remove-guide', side: target, axis, coordinate })}
    />
  }
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <section className="grid shrink-0 gap-2 border-b border-border bg-card px-3 py-2">
        <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <StatusPill tone={status.tone}>{status.label}</StatusPill>
            {review.status === 'completed' ? <StatusPill tone={conclusion.tone}>{conclusion.label}</StatusPill> : null}
            <span className="max-w-80 truncate text-xs text-muted-foreground">{copy.designManagement.common.path(review.project_set_name, review.plan_unit_title, review.requirement_title)}</span>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <Select items={review.rounds.map(round => ({ value: round.id, label: copy.designManagement.detail.roundValue(round.round_number) }))} value={review.id} onValueChange={(next) => next && props.onRoundChange(String(next))}>
              <SelectTrigger className="w-32" aria-label={copy.designManagement.detail.rounds}><History /><SelectValue /></SelectTrigger>
              <SelectContent>{review.rounds.map((round) => <SelectItem key={round.id} value={round.id}>{copy.designManagement.detail.roundValue(round.round_number)}</SelectItem>)}</SelectContent>
            </Select>
            {props.canCreate && ['completed', 'failed', 'abandoned'].includes(review.status) ? <Button size="sm" variant="outline" onClick={props.onRecheck}><RefreshCw />{copy.designManagement.detail.recheck}</Button> : null}
            {review.round_number > 1 ? <Button size="sm" variant={pendingRecheckCount ? 'default' : 'outline'} onClick={() => setRecheckOpen(true)}><History />{copy.designManagement.detail.recheckDecisions}{pendingRecheckCount ? <span className="tabular-nums">{pendingRecheckCount}</span> : null}</Button> : null}
            <Button size="icon-sm" variant="outline" title={copy.designManagement.detail.exportMarkdown} aria-label={copy.designManagement.detail.exportMarkdown} onClick={() => props.onExport('markdown')}><ArrowDownToLine /></Button>
            <Button size="icon-sm" variant="outline" title={copy.designManagement.detail.exportJson} aria-label={copy.designManagement.detail.exportJson} onClick={() => props.onExport('json')}><ArrowDownToLine /></Button>
            {props.canDelete ? <Button size="icon-sm" variant="destructive" title={copy.designManagement.detail.deleteReview} aria-label={copy.designManagement.detail.deleteReview} onClick={() => setDeleteOpen(true)}><Trash2 /></Button> : null}
          </div>
        </div>
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <ViewTabs value={canvasMode} label={copy.designManagement.detail.title} density="compact" tabs={[
            { id: 'compare', label: <><Columns2 className="size-3.5" />{copy.designManagement.detail.compare}</> },
            { id: 'overlay', label: <><Blend className="size-3.5" />{copy.designManagement.detail.overlay}</>, disabled: !review.dimensions_match },
            { id: 'difference', label: <><Flame className="size-3.5" />{copy.designManagement.detail.difference}</>, disabled: !review.dimensions_match || !props.heatmapURL },
          ]} onValueChange={(mode) => dispatch({ type: 'canvas-mode', mode: mode as typeof state.canvasMode })} />
          <Button size="icon-sm" variant="outline" title={copy.designManagement.detail.zoomOut} aria-label={copy.designManagement.detail.zoomOut} onClick={() => dispatch({ type: 'zoom', value: state.zoom - 0.25 })}><ZoomOut /></Button>
          <span className="min-w-12 text-center text-xs text-muted-foreground">{copy.designManagement.common.percentage(Math.round(state.zoom * 100))}</span>
          <Button size="icon-sm" variant="outline" title={copy.designManagement.detail.zoomIn} aria-label={copy.designManagement.detail.zoomIn} onClick={() => dispatch({ type: 'zoom', value: state.zoom + 0.25 })}><ZoomIn /></Button>
          <Button size="icon-sm" variant="outline" title={copy.designManagement.detail.fit} aria-label={copy.designManagement.detail.fit} onClick={() => dispatch({ type: 'zoom', value: 1 })}><Maximize2 /></Button>
          <Button size="sm" variant={state.rulers ? 'default' : 'outline'} onClick={() => dispatch({ type: 'toggle-rulers' })}><Ruler />{state.rulers ? copy.designManagement.detail.hideRulers : copy.designManagement.detail.showRulers}</Button>
          <label className="flex items-center gap-2 text-xs"><Checkbox checked={showAll} onCheckedChange={(checked) => setShowAll(checked === true)} />{copy.designManagement.detail.showAll}</label>
          <Button size="sm" variant="outline" onClick={() => dispatch({ type: 'toggle-drawer', open: !state.differenceDrawerOpen })}><ListFilter />{state.differenceDrawerOpen ? copy.designManagement.detail.hideDifferences : copy.designManagement.detail.showDifferences}</Button>
          {props.canManageAnalysis && review.status === 'completed' ? <Button size="sm" variant="outline" onClick={() => setDeepOpen(true)}><Sparkles />{copy.designManagement.detail.deepReview}</Button> : null}
          {selectionReady ? <StatusPill tone="success">{copy.designManagement.detail.selectionReady}</StatusPill> : null}
          {selectionReady ? <Button size="xs" variant="ghost" onClick={() => dispatch({ type: 'clear-region' })}>{copy.designManagement.detail.clearSelection}</Button> : null}
        </div>
        {!review.dimensions_match ? <AlertBanner tone="warning">{copy.designManagement.detail.dimensionsMismatch}</AlertBanner> : null}
        {review.status === 'failed' ? <AlertBanner tone="error" action={props.canManageAnalysis ? <div className="flex gap-2"><Button size="sm" variant="outline" onClick={props.onRetryAnalysis}>{copy.designManagement.detail.retryAnalysis}</Button>{review.source_review_id ? <Button size="sm" variant="outline" onClick={props.onAbandonAnalysis}>{copy.designManagement.detail.abandonAnalysis}</Button> : null}</div> : undefined}>{review.last_error || copy.designManagement.detail.analysisError}</AlertBanner> : null}
      </section>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col md:flex-row">
        <main className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <ViewTabs className="mx-3 mt-2 md:hidden" value={state.narrowSide} label={copy.designManagement.detail.title} density="compact" tabs={[{ id: 'design', label: copy.designManagement.detail.designView }, { id: 'implementation', label: copy.designManagement.detail.implementationView }]} onValueChange={(side) => dispatch({ type: 'narrow-side', side: side as typeof state.narrowSide })} />
          {canvasMode === 'compare' ? <>
            <div className="hidden min-h-0 min-w-0 flex-1 grid-cols-2 divide-x divide-border md:grid">
              {renderCanvas('design')}
              {renderCanvas('implementation')}
            </div>
            <div className="flex min-h-0 min-w-0 flex-1 md:hidden">
              {renderCanvas(state.narrowSide)}
            </div>
          </> : <CompositeCanvas mode={canvasMode} designURL={props.designURL} implementationURL={props.implementationURL} heatmapURL={props.heatmapURL} opacity={overlayOpacity} width={review.design_width || review.viewport_width} height={review.design_height || review.viewport_height} markers={designMarkers} selectedId={state.selectedDifferenceId} onSelect={selectDifference} />}
          {canvasMode === 'overlay' ? <label className="flex shrink-0 items-center gap-3 border-t border-border bg-card px-3 py-2 text-xs"><span>{copy.designManagement.detail.overlayOpacity}</span><Input className="h-6 max-w-72" type="range" min={25} max={75} step={25} value={overlayOpacity} onChange={(event) => setOverlayOpacity(Number(event.target.value))} /><span>{copy.designManagement.common.percentage(overlayOpacity)}</span></label> : null}
          {props.assetState === 'pending' || ['queued', 'analyzing'].includes(review.status) ? <AiLoadingOverlay label={review.status === 'analyzing' ? copy.designManagement.status.analyzing : copy.designManagement.detail.assetLoading} /> : null}
          {props.assetState === 'error' ? <div className="absolute inset-x-3 top-3 z-20"><AlertBanner tone="error">{props.assetMessage || copy.designManagement.detail.assetFailed}</AlertBanner></div> : null}
        </main>
        <aside className={state.differenceDrawerOpen ? 'flex max-h-[48%] min-h-0 min-w-0 border-t border-border md:max-h-none md:w-96 md:border-t-0 md:border-l' : 'hidden md:hidden'}>
          <DifferencePanel
            items={review.differences}
            filters={filters}
            selectedId={state.selectedDifferenceId}
            canManage={props.canManageDifferences}
            operationState={props.differenceState}
            operationMessage={props.differenceMessage}
            seedRegions={state.selections}
            onSelect={selectDifference}
            onFiltersChange={setFilters}
            onCreate={props.onCreateDifference}
            onUpdate={props.onUpdateDifference}
            onDelete={props.onDeleteDifference}
            onVerify={props.onVerifyDifference}
            onResolve={props.onResolveDifference}
            onReopen={props.onReopenDifference}
          />
        </aside>
      </div>
      <footer className="grid shrink-0 gap-2 border-t border-border bg-card px-3 py-2 sm:grid-cols-[minmax(0,1fr)_auto]">
        <div className="min-w-0"><span className="text-xs font-medium">{copy.designManagement.detail.analysisSummary}</span><p className="mt-0.5 truncate text-xs text-muted-foreground">{review.analysis_summary || copy.designManagement.detail.noSummary}</p></div>
        <span className="self-center text-xs text-muted-foreground">{copy.designManagement.common.metadata(review.created_by_name, new Date(review.created_at).toLocaleString('zh-CN'))}</span>
      </footer>
      <DeepReviewPanel open={deepOpen} onOpenChange={setDeepOpen} selections={state.selections} jobs={review.deep_review_jobs} state={props.analysisState} message={props.analysisMessage} onSubmit={props.onDeepReview} onRetry={props.onRetryDeepReview} onCancel={props.onCancelDeepReview} />
      <RecheckDecisionPanel open={recheckOpen} onOpenChange={setRecheckOpen} matches={review.pending_recheck_matches} verifications={review.recheck_verifications} canManage={props.canManageDifferences} operationState={props.differenceState === 'pending' ? 'pending' : 'idle'} message={props.differenceMessage} onResolveMatch={props.onResolveRecheckMatch} onOverrideVerification={props.onOverrideRecheckVerification} />
      <ConfirmDialog open={deleteOpen} onOpenChange={setDeleteOpen} onConfirm={props.onDelete} title={copy.designManagement.detail.deleteTitle} description={copy.designManagement.detail.deleteDescription} cancelLabel={copy.designManagement.common.cancel} confirmLabel={copy.designManagement.common.delete} pending={props.analysisState === 'pending'} error={props.analysisMessage} />
    </div>
  )
}

function regions(items: readonly DesignDifference[], key: 'design_region' | 'implementation_region') {
  return items.flatMap((item) => item[key] ? [{ id: item.id, ordinal: item.ordinal, title: item.title, severity: item.severity, region: item[key]! }] : [])
}

function CompositeCanvas({ mode, designURL, implementationURL, heatmapURL, opacity, width, height, markers, selectedId, onSelect }: { mode: 'overlay' | 'difference'; designURL?: string; implementationURL?: string; heatmapURL?: string; opacity: number; width: number; height: number; markers: ReturnType<typeof regions>; selectedId?: string; onSelect: (id: string) => void }) {
  const source = mode === 'difference' ? heatmapURL : designURL
  const opacityClass = opacity === 25 ? 'opacity-25' : opacity === 75 ? 'opacity-75' : 'opacity-50'
  return <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"><header className="flex h-9 shrink-0 items-center border-b border-border bg-card px-3 text-xs font-medium">{mode === 'difference' ? copy.designManagement.detail.difference : copy.designManagement.detail.overlay}</header><div className="min-h-0 flex-1 overflow-auto bg-muted/40 p-5">{source ? <div className="relative inline-block"><img src={source} alt={mode === 'difference' ? copy.designManagement.detail.difference : copy.designManagement.detail.designCanvas} width={width} height={height} className="block max-w-none" />{mode === 'overlay' && implementationURL ? <img src={implementationURL} alt={copy.designManagement.detail.implementationCanvas} className={`absolute inset-0 size-full ${opacityClass}`} /> : null}<svg viewBox={`0 0 ${width} ${height}`} className="absolute inset-0 size-full">{markers.map((marker) => <g key={marker.id} role="button" tabIndex={0} onClick={() => onSelect(marker.id)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') onSelect(marker.id) }} className="cursor-pointer"><rect x={marker.region.x * width} y={marker.region.y * height} width={marker.region.width * width} height={marker.region.height * height} rx="4" className={marker.id === selectedId ? 'fill-primary/20 stroke-primary stroke-[3]' : 'fill-transparent stroke-foreground/60 stroke-2'} /><text x={marker.region.x * width + 6} y={marker.region.y * height + 18} className="fill-foreground text-xs font-semibold">{copy.designManagement.common.ordinalShort(marker.ordinal)}</text><title>{marker.title}</title></g>)}</svg></div> : <div className="grid min-h-64 place-items-center text-xs text-muted-foreground">{copy.designManagement.detail.assetUnavailable}</div>}</div></section>
}

function DeepReviewPanel({ open, onOpenChange, selections, jobs, state, message, onSubmit, onRetry, onCancel }: { open: boolean; onOpenChange: (open: boolean) => void; selections: { design?: Region; implementation?: Region }; jobs: DeepReviewJob[]; state: OperationState; message?: string; onSubmit: Props['onDeepReview']; onRetry: Props['onRetryDeepReview']; onCancel: Props['onCancelDeepReview'] }) {
  const [focus, setFocus] = useState('')
  const ready = Boolean(selections.design && selections.implementation)
  const submit = () => { if (selections.design && selections.implementation) onSubmit(selections.design, selections.implementation, focus) }
  return <ResponsivePanel open={open} onOpenChange={onOpenChange} title={copy.designManagement.detail.deepReview} description={copy.designManagement.detail.deepDescription} pending={state === 'pending'} footer={<><Button variant="outline" onClick={() => onOpenChange(false)}>{copy.designManagement.common.cancel}</Button><Button disabled={!ready} onClick={submit}><ScanSearch />{copy.designManagement.detail.startDeepReview}</Button></>}><div className="grid gap-4">{ready ? <AlertBanner tone="success"><div>{copy.designManagement.common.labeled(copy.designManagement.detail.designCanvas, copy.designManagement.detail.regionValue(selections.design!))}<br />{copy.designManagement.common.labeled(copy.designManagement.detail.implementationCanvas, copy.designManagement.detail.regionValue(selections.implementation!))}</div></AlertBanner> : <AlertBanner tone="warning">{copy.designManagement.detail.deepSelectionMissing}</AlertBanner>}<div className="grid gap-1.5"><Label htmlFor="deep-review-focus">{copy.designManagement.detail.focus}</Label><Textarea id="deep-review-focus" maxLength={500} value={focus} placeholder={copy.designManagement.detail.focusPlaceholder} onChange={(event) => setFocus(event.target.value)} /></div>{message ? <AlertBanner tone="error">{message}</AlertBanner> : null}<DeepReviewHistory jobs={jobs} onRetry={onRetry} onCancel={onCancel} /></div></ResponsivePanel>
}

function DeepReviewHistory({ jobs, onRetry, onCancel }: { jobs: DeepReviewJob[]; onRetry: (id: string) => void; onCancel: (id: string) => void }) {
  return <section className="grid gap-2 border-t border-border pt-4"><h3 className="text-sm font-medium">{copy.designManagement.detail.deepHistory}</h3>{jobs.length === 0 ? <p className="text-xs text-muted-foreground">{copy.designManagement.detail.deepEmpty}</p> : jobs.map((job) => <div key={job.id} className="grid gap-2 rounded-md border border-border bg-muted/30 p-3"><div className="flex flex-wrap items-center justify-between gap-2"><div className="flex flex-wrap items-center gap-2"><StatusPill tone={deepJobTone(job.status)}>{copy.designManagement.deepStatus[job.status]}</StatusPill><span className="text-xs text-muted-foreground">{copy.designManagement.common.metadata(copy.designManagement.detail.deepAttempts(job.attempts), copy.designManagement.detail.deepGenerated(job.generated_difference_count))}</span></div><div className="flex gap-1.5">{job.can_retry ? <Button size="xs" variant="outline" onClick={() => onRetry(job.id)}><RefreshCw />{copy.designManagement.detail.retryDeepReview}</Button> : null}{job.can_cancel ? <Button size="xs" variant="outline" onClick={() => onCancel(job.id)}>{copy.designManagement.detail.cancelDeepReview}</Button> : null}</div></div>{job.focus ? <p className="text-xs text-foreground">{job.focus}</p> : null}{job.last_error ? <AlertBanner tone="error">{job.last_error}</AlertBanner> : null}</div>)}</section>
}

function deepJobTone(status: DeepReviewJob['status']) {
  if (status === 'succeeded') return 'success' as const
  if (status === 'failed') return 'danger' as const
  if (status === 'pending') return 'warning' as const
  if (status === 'running') return 'info' as const
  return 'neutral' as const
}
