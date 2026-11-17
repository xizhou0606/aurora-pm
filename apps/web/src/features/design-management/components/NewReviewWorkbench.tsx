import { useEffect, useMemo, useReducer, useState } from 'react'

import type { PMScopeSelection } from '@/shared/api'
import { copy } from '../copy'
import { useWebCaptureBridge } from '../hooks/useWebCaptureBridge'
import {
  MAX_EVIDENCE_BYTES,
  editableMobileTarget,
  figmaPreviewViewport,
  initialNewReviewDraft,
  loadMobileCaptureEvidence,
  newReviewValidationIssue,
  previewMatchesFigma,
  requirementScopeOptions,
  requirementScopeSelection,
  validFigmaURL,
  validHTTPURL,
  type FigmaPreview,
  type NewReviewDraft,
  type NewReviewEvidence,
  withoutMobileEvidence,
} from '../model'
import { mobileStructureFile, webStructureFiles } from '../utils/capture-structure'
import { NewReviewForm } from './NewReviewForm'
import type { NewReviewWorkbenchProps } from './new-review-contract'

export type { NewReviewSubmission } from '../model'

export function NewReviewWorkbench(props: NewReviewWorkbenchProps) {
  const [draft, updateDraft] = useReducer(
    (state: NewReviewDraft, patch: Partial<NewReviewDraft>) => ({ ...state, ...patch }),
    initialNewReviewDraft,
  )
  const [validation, setValidation] = useState('')
  const [collectorId, setCollectorId] = useState('')
  const [deviceId, setDeviceId] = useState('')
  const { evidence, setEvidence, beginCaptureRequest, cancelActiveRequest } = useWebCaptureBridge(
    draft.figmaUrl,
    props.onNormalizeDOM,
    setValidation,
  )
  const scopeOptions = useMemo(
    () => requirementScopeOptions(props.requirements),
    [props.requirements],
  )
  const selection = useMemo(
    () => requirementScopeSelection(draft.requirementId, props.requirements),
    [draft.requirementId, props.requirements],
  )
  const availableCollectors = useMemo(
    () => props.collectors.filter((item) => (
      !item.revoked_at && item.doctor_ready && item.version === '0.1.0'
    )),
    [props.collectors],
  )
  const selectedCollector = availableCollectors.find((item) => item.id === collectorId)
  const mobileDevices = (selectedCollector?.inventory?.devices ?? []).filter(
    (item) => item.platform === draft.platform,
  )
  const completedCaptures = props.captures.filter((item) => (
    item.platform === draft.platform
    && item.screenshot_available
    && (item.status === 'succeeded' || item.status === 'consumed')
  ))

  const clearEvidence = () => {
    cancelActiveRequest()
    setEvidence((current) => ({ extension: current.extension }))
  }

  useEffect(() => {
    if (!props.sourceReview) return
    updateDraft({
      mode: props.sourceReview.implementation_kind,
      requirementId: props.sourceReview.requirement_id,
      figmaUrl: props.sourceReview.figma_url,
      implementationUrl: props.sourceReview.implementation_target.original_url ?? '',
      platform: props.sourceReview.implementation_target.platform ?? 'ios',
      appId: props.sourceReview.implementation_target.app_id ?? '',
      appName: props.sourceReview.implementation_target.app_name ?? '',
      appVersion: props.sourceReview.implementation_target.app_version ?? '',
      orientation: props.sourceReview.implementation_target.orientation ?? 'portrait',
      viewportWidth: props.sourceReview.viewport_width,
      viewportHeight: props.sourceReview.viewport_height,
      devicePixelRatio: props.sourceReview.device_pixel_ratio,
    })
  }, [props.sourceReview])

  useEffect(() => {
    if (!props.sourceReview && props.initialRequirementId && props.requirements.some((item) => item.id === props.initialRequirementId)) {
      updateDraft({ requirementId: props.initialRequirementId })
    }
  }, [props.initialRequirementId, props.requirements, props.sourceReview])

  useEffect(() => {
    const viewport = figmaPreviewViewport(props.preview, draft.figmaUrl)
    if (viewport) {
      if (viewport.viewportWidth !== draft.viewportWidth || viewport.viewportHeight !== draft.viewportHeight) clearEvidence()
      updateDraft(viewport)
    }
  }, [
    draft.figmaUrl,
    props.preview?.file_key,
    props.preview?.node_id,
    props.preview?.width,
    props.preview?.height,
  ])

  const changeMode = (mode: NewReviewDraft['mode']) => {
    if (mode === draft.mode) return
    clearEvidence()
    setCollectorId('')
    setDeviceId('')
    updateDraft({ mode })
  }
  const changeFigmaURL = (figmaUrl: string) => {
    setValidation('')
    props.onResetPreview()
    updateDraft({ figmaUrl })
    if (evidence.normalizationManifest || evidence.implementationOriginal) clearEvidence()
  }
  const changeImplementationURL = (implementationUrl: string) => {
    if (implementationUrl !== draft.implementationUrl && evidence.implementation) clearEvidence()
    updateDraft({ implementationUrl })
  }
  const changeCaptureSettings = (patch: Partial<Pick<NewReviewDraft, 'viewportWidth' | 'viewportHeight' | 'devicePixelRatio' | 'topbar'>>) => {
    const changed = (patch.viewportWidth !== undefined && patch.viewportWidth !== draft.viewportWidth)
      || (patch.viewportHeight !== undefined && patch.viewportHeight !== draft.viewportHeight)
      || (patch.devicePixelRatio !== undefined && patch.devicePixelRatio !== draft.devicePixelRatio)
      || (patch.topbar !== undefined && patch.topbar !== draft.topbar)
    if (changed) clearEvidence()
    updateDraft(patch)
  }
  const changeDOMNormalization = (enabled: boolean) => {
    updateDraft({ domNormalization: enabled })
    if (!enabled && evidence.implementationOriginal) {
      setEvidence((current) => ({
        ...current,
        implementation: current.implementationOriginal,
        implementationOriginal: undefined,
        normalizationManifest: undefined,
        normalizationFigmaUrl: undefined,
        inventory: undefined,
        webCapture: current.webCapture ? {
          ...current.webCapture,
          elements: current.webCapture.originalElements,
          structureTruncated: current.webCapture.originalStructureTruncated,
        } : undefined,
      }))
    }
  }
  const changePlatform = (platform: NewReviewDraft['platform']) => {
    if (platform === draft.platform) return
    clearEvidence()
    setCollectorId('')
    setDeviceId('')
    updateDraft({ platform, appId: '', appName: '', appVersion: '' })
  }
  const changeCollector = (id: string) => {
    setEvidence(withoutMobileEvidence)
    setCollectorId(id)
    setDeviceId('')
  }
  const changeDevice = (id: string) => {
    setEvidence(withoutMobileEvidence)
    setDeviceId(id)
  }

  const chooseEvidence = (file?: File) => {
    if (!file) return
    if (!file.type.startsWith('image/') || file.size > MAX_EVIDENCE_BYTES) {
      setValidation(copy.designManagement.newReview.invalidEvidence)
      return
    }
    setValidation('')
    clearEvidence()
    setEvidence((current) => ({ ...current, implementation: file }))
  }

  const loadMobileCapture = async (captureId: string) => {
    setValidation('')
    setEvidence((current) => ({ ...current, mobileLoading: true }))
    try {
      const result = await loadMobileCaptureEvidence(
        captureId,
        props.captures,
        draft.platform,
        props.onLoadMobileCapture,
      )
      setEvidence((current) => ({
        ...current,
        implementation: result.implementation,
        mobileCaptureId: captureId,
        mobileTarget: result.target,
        mobileCapture: result.capture,
        mobileLoading: false,
      }))
      updateDraft({
        appId: result.target.app_id ?? '',
        appName: result.target.app_name ?? '',
        appVersion: result.target.app_version ?? '',
        orientation: result.target.orientation ?? 'portrait',
        viewportWidth: result.screen?.width ?? draft.viewportWidth,
        viewportHeight: result.screen?.height ?? draft.viewportHeight,
        devicePixelRatio: result.screen?.scale ?? draft.devicePixelRatio,
      })
    } catch {
      setEvidence(withoutMobileEvidence)
      setValidation(copy.designManagement.newReview.invalidMobileCapture)
    }
  }

  const capture = () => {
    if (!validHTTPURL(draft.implementationUrl)) return setValidation(copy.designManagement.newReview.invalidTarget)
    if (draft.domNormalization && (!props.preview || !previewMatchesFigma(props.preview, draft.figmaUrl))) {
      return setValidation(copy.designManagement.newReview.figmaEvidenceRequired)
    }
    setValidation('')
    const requestID = crypto.randomUUID()
    beginCaptureRequest(requestID)
    window.postMessage({
      source: 'pm-design-review-web',
      type: 'PM_DESIGN_REVIEW_CAPTURE_REQUEST',
      request_id: requestID,
      payload: {
        url: draft.implementationUrl,
        frame_selector: '#pm-design-review-implementation-frame',
        dom_normalization: draft.domNormalization,
        viewport: {
          width: draft.viewportWidth,
          height: draft.viewportHeight - (draft.topbar ? 44 : 0),
          devicePixelRatio: draft.devicePixelRatio,
        },
      },
    }, window.origin)
  }

  const captureFromTargetTab = () => {
    if (!validHTTPURL(draft.implementationUrl)) return setValidation(copy.designManagement.newReview.invalidTarget)
    setValidation('')
    const requestID = crypto.randomUUID()
    beginCaptureRequest(requestID)
    window.postMessage({
      source: 'pm-design-review-web',
      type: 'PM_DESIGN_REVIEW_START_TAB_CAPTURE',
      request_id: requestID,
      payload: {
        url: draft.implementationUrl,
        dom_normalization: draft.domNormalization,
        viewport: {
          width: draft.viewportWidth,
          height: draft.viewportHeight - (draft.topbar ? 44 : 0),
          devicePixelRatio: draft.devicePixelRatio,
        },
      },
    }, window.origin)
  }

  const submit = () => {
    const issue = newReviewValidationIssue(draft, evidence, props.preview)
    if (issue) return setValidation(copy.designManagement.newReview[issue])
    const region = draft.scoped ? draft.region : undefined
    const topbar = draft.mode === 'web' && draft.topbar
      ? { enabled: true, platform: 'ios', height: 44, time: '9:41', background_color: '#ffffff', icon_theme: 'dark' }
      : { enabled: false }
    const mobileTarget = draft.mode === 'mobile'
      ? editableMobileTarget(evidence.mobileTarget!, draft)
      : undefined
    const structures = draft.mode === 'web'
      ? webStructureFiles(evidence.webCapture, {
          actualUrl: draft.implementationUrl.trim(),
          viewportWidth: draft.viewportWidth,
          viewportHeight: draft.viewportHeight,
          topbar,
        })
      : { current: mobileStructureFile(evidence.mobileCapture!, mobileTarget!, draft.viewportWidth, draft.viewportHeight), original: undefined }
    setValidation('')
    props.onSubmit({
      requirement_id: draft.requirementId!,
      source_review_id: props.sourceReview?.id,
      figma_url: draft.figmaUrl.trim(),
      implementation_target: draft.mode === 'web'
        ? { kind: 'web', original_url: draft.implementationUrl.trim(), actual_url: evidence.webCapture?.actualUrl || draft.implementationUrl.trim() }
        : mobileTarget!,
      viewport_width: draft.viewportWidth,
      viewport_height: draft.viewportHeight,
      device_pixel_ratio: draft.devicePixelRatio,
      page_zoom: 1,
      analysis_scope: region ? { enabled: true, version: '1.0', selection_side: 'design', design_region: region, implementation_region: region } : { enabled: false },
      mobile_topbar: topbar,
	      dom_normalization: { enabled: draft.mode === 'web' && draft.domNormalization },
      implementationFile: evidence.implementation!,
      implementationOriginalFile: evidence.implementationOriginal,
      implementationStructureFile: structures.current,
      implementationStructureOriginalFile: draft.domNormalization ? structures.original : undefined,
      normalizationManifest: evidence.normalizationManifest,
      mobileCaptureId: evidence.mobileCaptureId,
    })
  }

  const scopeState = props.requirementsState === 'ready'
    ? { status: 'ready' as const, options: scopeOptions }
    : props.requirementsState === 'error'
      ? { status: 'error' as const, onRetry: props.onRetryRequirements }
      : { status: 'loading' as const }
  return <NewReviewForm
    draft={draft}
    updateDraft={updateDraft}
    scopeState={scopeState}
    selection={selection}
    evidence={evidence}
    preview={props.preview}
    previewState={props.previewState}
    previewErrorKey={props.previewErrorKey}
    submitState={props.submitState}
    validation={validation}
    message={props.message}
    collectors={availableCollectors}
    collectorId={collectorId}
    deviceId={deviceId}
    mobileDevices={mobileDevices}
    completedCaptures={completedCaptures}
    mobileCaptureState={props.mobileCaptureState}
    onModeChange={changeMode}
    onRequirementChange={(next: PMScopeSelection) => updateDraft({ requirementId: next.requirementIds[0] })}
    onFigmaChange={changeFigmaURL}
    onPreview={() => {
      if (!validFigmaURL(draft.figmaUrl)) {
        setValidation(copy.designManagement.newReview.invalidFigma)
        return
      }
      setValidation('')
      props.onPreview(draft.figmaUrl.trim())
    }}
    onImplementationURLChange={changeImplementationURL}
    onCaptureSettingsChange={changeCaptureSettings}
    onDOMNormalizationChange={changeDOMNormalization}
    onPlatformChange={changePlatform}
    onCollectorChange={changeCollector}
    onDeviceChange={changeDevice}
    onStartMobileCapture={() => props.onStartMobileCapture({ collector_id: collectorId, platform: draft.platform, device_id: deviceId })}
    onLoadMobileCapture={(id) => void loadMobileCapture(id)}
    onChooseEvidence={chooseEvidence}
    onCapture={capture}
    onFallbackCapture={captureFromTargetTab}
    onSubmit={submit}
  />
}
