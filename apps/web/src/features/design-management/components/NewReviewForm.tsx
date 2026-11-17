import {
  PhArrowSquareOut as ExternalLink,
  PhCamera as Camera,
  PhCheck as Check,
  PhDesktop as Monitor,
  PhDeviceMobile as Smartphone,
  PhEye as Eye,
  PhFileImage as FileImage,
  PhFileMagnifyingGlass as ScanSearch,
} from '@/shared/ui/icons'

import type { PMScopeSelection } from '@/shared/api'
import {
  AlertBanner,
  Button,
  Checkbox,
  Input,
  Label,
  PMScopeCascaderField,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  ViewTabs,
  type PMScopeCascaderState,
} from '@/shared/ui'
import {
  MAX_REVIEW_VIEWPORT_SIZE,
  MIN_REVIEW_VIEWPORT_SIZE,
  previewMatchesFigma,
  validHTTPURL,
  type FigmaPreview,
  type FigmaPreviewErrorCopyKey,
  type MobileCaptureDraft,
  type MobileCollector,
  type NewReviewDraft,
  type NewReviewEvidence,
} from '../model'
import { copy } from '../copy'
import { ImplementationFramePreview } from './ImplementationFramePreview'

type Props = {
  draft: NewReviewDraft
  updateDraft: (patch: Partial<NewReviewDraft>) => void
  scopeState: PMScopeCascaderState
  selection: PMScopeSelection
  evidence: NewReviewEvidence
  preview?: FigmaPreview
  previewState: 'idle' | 'pending' | 'error' | 'ready'
  previewErrorKey?: FigmaPreviewErrorCopyKey
  submitState: 'idle' | 'pending' | 'error'
  validation?: string
  message?: string
  collectors: readonly MobileCollector[]
  collectorId: string
  deviceId: string
  mobileDevices: NonNullable<MobileCollector['inventory']>['devices']
  completedCaptures: readonly MobileCaptureDraft[]
  mobileCaptureState: 'idle' | 'pending' | 'error'
  onModeChange: (mode: NewReviewDraft['mode']) => void
  onRequirementChange: (selection: PMScopeSelection) => void
  onFigmaChange: (value: string) => void
  onPreview: () => void
  onImplementationURLChange: (value: string) => void
  onCaptureSettingsChange: (patch: Partial<Pick<NewReviewDraft, 'viewportWidth' | 'viewportHeight' | 'devicePixelRatio' | 'topbar'>>) => void
  onDOMNormalizationChange: (enabled: boolean) => void
  onPlatformChange: (platform: NewReviewDraft['platform']) => void
  onCollectorChange: (id: string) => void
  onDeviceChange: (id: string) => void
  onStartMobileCapture: () => void
  onLoadMobileCapture: (id: string) => void
  onChooseEvidence: (file?: File) => void
  onCapture: () => void
  onFallbackCapture: () => void
  onSubmit: () => void
}

export function NewReviewForm(props: Props) {
  const { draft, evidence, preview } = props
  const currentPreview = preview && previewMatchesFigma(preview, draft.figmaUrl) ? preview : undefined
  const previewImage = currentPreview
    ? <img src={currentPreview.image_url} alt={currentPreview.node_name} className="max-h-72 w-full object-contain" />
    : null
  return (
    <div className="grid min-w-0 gap-5 p-3 sm:p-5 xl:grid-cols-[minmax(20rem,28rem)_minmax(0,1fr)]">
      <form className="grid min-w-0 content-start gap-5" onSubmit={(event) => { event.preventDefault(); props.onSubmit() }}>
        <section className="grid gap-3 border-b border-border pb-5">
          <ViewTabs value={draft.mode} label={copy.designManagement.newReview.mode} tabs={[
            { id: 'web', label: copy.designManagement.newReview.web },
            { id: 'mobile', label: copy.designManagement.newReview.mobile },
          ]} onValueChange={(value) => props.onModeChange(value as NewReviewDraft['mode'])} />
          <PMScopeCascaderField
            idPrefix="design-review-requirement"
            state={props.scopeState}
            value={props.selection}
            selectionMode="single-requirement"
            labels={{
              label: copy.designManagement.newReview.requirement,
              description: copy.designManagement.newReview.requirementPlaceholder,
              projectSet: copy.designManagement.filters.projectSet,
              project: copy.designManagement.filters.planUnit,
              requirement: copy.designManagement.filters.requirement,
              selectProjectSetFirst: copy.designManagement.scope.selectProjectSetFirst,
              selectProjectFirst: copy.designManagement.scope.selectProjectFirst,
            }}
            onChange={props.onRequirementChange}
          />
        </section>

        <section className="grid gap-2 border-b border-border pb-5">
          <Label htmlFor="design-review-figma">{copy.designManagement.newReview.figmaUrl}</Label>
          <div className="flex min-w-0 gap-2">
            <Input id="design-review-figma" value={draft.figmaUrl} placeholder={copy.designManagement.newReview.figmaPlaceholder} onChange={(event) => props.onFigmaChange(event.target.value)} />
            <Button type="button" variant="outline" disabled={props.previewState === 'pending'} onClick={props.onPreview}><Eye />{copy.designManagement.newReview.preview}</Button>
          </div>
          {props.previewState === 'error' ? <p role="alert" className="text-xs text-destructive">{copy.designManagement.newReview[props.previewErrorKey ?? 'figmaPreviewFailed']}</p> : null}
        </section>

        {draft.mode === 'web' ? <WebTargetFields {...props} /> : <MobileTargetFields {...props} />}

        <section className="grid gap-3 border-b border-border pb-5 sm:grid-cols-3">
          <NumberField id="review-width" label={copy.designManagement.newReview.viewportWidth} value={draft.viewportWidth} min={MIN_REVIEW_VIEWPORT_SIZE} max={MAX_REVIEW_VIEWPORT_SIZE} onChange={(viewportWidth) => props.onCaptureSettingsChange({ viewportWidth })} />
          <NumberField id="review-height" label={copy.designManagement.newReview.viewportHeight} value={draft.viewportHeight} min={MIN_REVIEW_VIEWPORT_SIZE} max={MAX_REVIEW_VIEWPORT_SIZE} onChange={(viewportHeight) => props.onCaptureSettingsChange({ viewportHeight })} />
          <NumberField id="review-dpr" label={copy.designManagement.newReview.devicePixelRatio} value={draft.devicePixelRatio} min={0.5} max={8} step={0.5} onChange={(devicePixelRatio) => props.onCaptureSettingsChange({ devicePixelRatio })} />
          <label className="flex items-center gap-2 text-sm sm:col-span-3"><Checkbox checked={draft.scoped} onCheckedChange={(checked) => props.updateDraft({ scoped: checked === true })} />{copy.designManagement.newReview.selectedRegion}</label>
          {draft.scoped ? (['x', 'y', 'width', 'height'] as const).map((key) => <NumberField key={key} id={`review-region-${key}`} label={copy.designManagement.region[key]} value={draft.region[key]} min={0} max={1} step={0.01} onChange={(value) => props.updateDraft({ region: { ...draft.region, [key]: value } })} />) : null}
        </section>

        <section className="grid gap-3">
          <span className="text-sm font-medium">{copy.designManagement.newReview.evidence}</span>
          {currentPreview ? <AlertBanner tone="success">{copy.designManagement.newReview.designEvidenceServerReady}</AlertBanner> : <AlertBanner tone="warning">{copy.designManagement.newReview.designEvidencePreviewRequired}</AlertBanner>}
          <EvidenceInput id="implementation-evidence" label={copy.designManagement.newReview.implementationEvidence} file={evidence.implementation} onChange={props.onChooseEvidence} />
        </section>
        {props.validation || props.message ? <AlertBanner tone="error">{props.validation || props.message}</AlertBanner> : null}
        <Button type="submit" disabled={props.submitState === 'pending'}><ScanSearch />{props.submitState === 'pending' ? copy.designManagement.newReview.submitting : copy.designManagement.newReview.submit}</Button>
      </form>

      <section className="min-h-80 min-w-0 overflow-hidden rounded-md border border-border bg-muted/30">
        <div className="flex items-center gap-2 border-b border-border bg-card px-3 py-2 text-sm font-medium"><FileImage className="size-4" />{currentPreview?.node_name || copy.designManagement.newReview.designEvidence}</div>
        <div className="grid min-h-72 place-items-center overflow-auto p-3">
          {props.previewState === 'pending' ? <span className="text-sm text-muted-foreground">{copy.designManagement.newReview.previewing}</span> : previewImage ?? <Monitor className="size-12 text-muted-foreground" />}
        </div>
        {draft.mode === 'web' && validHTTPURL(draft.implementationUrl) ? (
          <div className="max-w-full overflow-hidden border-t border-border bg-muted/30 p-3">
            <ImplementationFramePreview
              url={draft.implementationUrl}
              viewportWidth={draft.viewportWidth}
              viewportHeight={draft.viewportHeight - (draft.topbar ? 44 : 0)}
            />
          </div>
        ) : null}
      </section>
    </div>
  )
}

function WebTargetFields(props: Props) {
  const { draft, evidence } = props
  const connected = evidence.extension !== 'disconnected' && evidence.extension !== 'checking'
  const busy = evidence.extension === 'capturing' || evidence.extension === 'normalizing'
  return <section className="grid gap-3 border-b border-border pb-5">
    <Field label={copy.designManagement.newReview.implementationUrl} id="design-review-url"><Input id="design-review-url" value={draft.implementationUrl} placeholder={copy.designManagement.newReview.implementationPlaceholder} onChange={(event) => props.onImplementationURLChange(event.target.value)} /></Field>
    <label className="flex items-center gap-2 text-sm"><Checkbox checked={draft.domNormalization} onCheckedChange={(checked) => props.onDOMNormalizationChange(checked === true)} />{copy.designManagement.newReview.domNormalization}</label>
    <label className="flex items-center gap-2 text-sm"><Checkbox checked={draft.topbar} onCheckedChange={(checked) => props.onCaptureSettingsChange({ topbar: checked === true })} />{copy.designManagement.newReview.topbar}</label>
    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
      <StatusDot active={connected} />
      <span>{evidence.extension === 'capturing' ? copy.designManagement.newReview.extensionCapturing : evidence.extension === 'normalizing' ? copy.designManagement.newReview.extensionNormalizing : connected ? copy.designManagement.newReview.extensionConnected : copy.designManagement.newReview.extensionDisconnected}</span>
      <Button type="button" size="sm" variant="outline" disabled={!connected || busy} onClick={props.onCapture}><Camera />{copy.designManagement.newReview.captureTarget}</Button>
      {evidence.iframeFallbackAvailable ? <Button type="button" size="sm" variant="outline" disabled={!connected || busy} onClick={props.onFallbackCapture}><ExternalLink />{copy.designManagement.newReview.captureTargetFallback}</Button> : null}
    </div>
    <p className="m-0 text-xs text-muted-foreground">{copy.designManagement.newReview.targetFallback}</p>
  </section>
}

function MobileTargetFields(props: Props) {
  const { draft } = props
  return <section className="grid gap-3 border-b border-border pb-5 sm:grid-cols-2">
    <SelectField label={copy.designManagement.newReview.platform} value={draft.platform} options={[['ios', copy.designManagement.newReview.ios], ['android', copy.designManagement.newReview.android]]} onChange={(value) => props.onPlatformChange(value as NewReviewDraft['platform'])} />
    <SelectField label={copy.designManagement.newReview.orientation} value={draft.orientation} options={[['portrait', copy.designManagement.newReview.portrait], ['landscape', copy.designManagement.newReview.landscape]]} onChange={(value) => props.updateDraft({ orientation: value as NewReviewDraft['orientation'] })} />
    <SelectField label={copy.designManagement.newReview.mobileCollector} value={props.collectorId} options={props.collectors.map((item) => [item.id, `${item.name} · ${item.hostname}`])} onChange={props.onCollectorChange} />
    <SelectField label={copy.designManagement.newReview.mobileDevice} value={props.deviceId} options={(props.mobileDevices ?? []).map((item) => [item.id, `${item.name} · ${item.os_version ?? copy.designManagement.common.none}`])} onChange={props.onDeviceChange} />
    <Button type="button" variant="outline" disabled={!props.collectorId || !props.deviceId || props.mobileCaptureState === 'pending'} onClick={props.onStartMobileCapture}><Camera />{props.mobileCaptureState === 'pending' ? copy.designManagement.newReview.startingMobileCapture : copy.designManagement.newReview.startMobileCapture}</Button>
    <SelectField label={copy.designManagement.newReview.completedMobileCapture} value={props.evidence.mobileCaptureId ?? ''} options={props.completedCaptures.map((item) => [item.id, `${item.app_id ?? item.device_id} · ${formatCaptureTime(item.captured_at)}`])} onChange={props.onLoadMobileCapture} />
    <Field label={copy.designManagement.newReview.appId} id="design-review-app-id"><Input id="design-review-app-id" value={draft.appId} onChange={(event) => props.updateDraft({ appId: event.target.value })} /></Field>
    <Field label={copy.designManagement.newReview.appName} id="design-review-app-name"><Input id="design-review-app-name" value={draft.appName} onChange={(event) => props.updateDraft({ appName: event.target.value })} /></Field>
    <Field label={copy.designManagement.newReview.appVersion} id="design-review-app-version"><Input id="design-review-app-version" value={draft.appVersion} onChange={(event) => props.updateDraft({ appVersion: event.target.value })} /></Field>
  </section>
}

function Field({ label, id, children }: { label: string; id: string; children: React.ReactNode }) {
  return <div className="grid gap-1.5"><Label htmlFor={id}>{label}</Label>{children}</div>
}

function NumberField({ id, label, value, min, max, step, onChange }: { id: string; label: string; value: number; min: number; max?: number; step?: number; onChange: (value: number) => void }) {
  return <Field label={label} id={id}><Input id={id} type="number" value={value} min={min} max={max} step={step} onChange={(event) => onChange(Number(event.target.value))} /></Field>
}

function SelectField({ label, value, options, onChange }: { label: string; value: string; options: Array<[string, string]>; onChange: (value: string) => void }) {
  return <div className="grid gap-1.5"><Label>{label}</Label><Select items={options.map(([id, text]) => ({ value: id, label: text }))} value={value} onValueChange={(next) => next && onChange(String(next))}><SelectTrigger aria-label={label}><SelectValue /></SelectTrigger><SelectContent>{options.map(([id, text]) => <SelectItem key={id} value={id}>{text}</SelectItem>)}</SelectContent></Select></div>
}

function EvidenceInput({ id, label, file, onChange }: { id: string; label: string; file?: File; onChange: (file?: File) => void }) {
  return <div className="grid gap-1.5"><Label htmlFor={id}>{label}</Label><div className="flex min-w-0 items-center gap-2"><Input id={id} type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => onChange(event.target.files?.[0])} /><span className="max-w-40 truncate text-xs text-muted-foreground">{file ? `${file.name} · ${Math.ceil(file.size / 1024)} KB` : copy.designManagement.common.none}</span></div></div>
}

function StatusDot({ active }: { active: boolean }) {
  return active ? <Check className="size-4 text-emerald-600 dark:text-emerald-400" /> : <Smartphone className="size-4" />
}

function formatCaptureTime(value?: string): string {
  return value ? new Date(value).toLocaleString('zh-CN') : copy.designManagement.common.none
}
