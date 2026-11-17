import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'

import { copy } from '../copy'
import type { NewReviewEvidence } from '../model'
import { webCaptureStructure } from '../utils/capture-structure'

type NormalizeDOM = (
  figmaUrl: string,
  inventory: Record<string, unknown>,
) => Promise<Record<string, unknown>>

type WebCaptureBridge = {
  evidence: NewReviewEvidence
  setEvidence: Dispatch<SetStateAction<NewReviewEvidence>>
  beginCaptureRequest: (requestID: string) => void
  cancelActiveRequest: () => void
}

const EXTENSION_START_TIMEOUT_MS = 3_000
const CAPTURE_RESULT_TIMEOUT_MS = 30_000

export function useWebCaptureBridge(
  figmaUrl: string,
  normalizeDOM: NormalizeDOM,
  onError: (message: string) => void,
): WebCaptureBridge {
  const [evidence, setEvidence] = useState<NewReviewEvidence>({ extension: 'checking' })
  const normalizeDOMRef = useRef(normalizeDOM)
  const figmaURLRef = useRef(figmaUrl)
  const onErrorRef = useRef(onError)
  const activeRequest = useRef<string | undefined>(undefined)
  const startupTimeout = useRef<number | undefined>(undefined)
  const captureTimeout = useRef<number | undefined>(undefined)
  const clearStartupTimeout = () => {
    if (startupTimeout.current !== undefined) window.clearTimeout(startupTimeout.current)
    startupTimeout.current = undefined
  }
  const clearCaptureTimeout = () => {
    if (captureTimeout.current !== undefined) window.clearTimeout(captureTimeout.current)
    captureTimeout.current = undefined
  }
  const cancelActiveRequest = () => {
    if (activeRequest.current) abortRequest(activeRequest.current)
    activeRequest.current = undefined
    clearStartupTimeout()
    clearCaptureTimeout()
  }
  const beginCaptureRequest = (requestID: string) => {
    clearStartupTimeout()
    activeRequest.current = requestID
    setEvidence({ extension: 'capturing' })
    clearCaptureTimeout()
    startupTimeout.current = window.setTimeout(() => {
      if (activeRequest.current !== requestID) return
      activeRequest.current = undefined
      setEvidence((current) => ({ ...current, extension: 'disconnected' }))
      onErrorRef.current(copy.designManagement.newReview.extensionUnavailable)
    }, EXTENSION_START_TIMEOUT_MS)
  }

  useEffect(() => { normalizeDOMRef.current = normalizeDOM }, [normalizeDOM])
  useEffect(() => { figmaURLRef.current = figmaUrl }, [figmaUrl])
  useEffect(() => { onErrorRef.current = onError }, [onError])

  useEffect(() => {
    const listener = (event: MessageEvent<unknown>) => {
      if (event.source !== window || event.origin !== window.origin || !isRecord(event.data)) return
      const data = event.data
      if (data.source !== 'pm-design-review-extension') return
      if (data.type === 'PM_DESIGN_REVIEW_READY') {
        setEvidence((current) => ({ ...current, extension: 'connected' }))
        return
      }
      if (data.request_id !== activeRequest.current) return
      if (data.type === 'PM_DESIGN_REVIEW_CAPTURE_STARTED') {
        clearStartupTimeout()
        clearCaptureTimeout()
        captureTimeout.current = window.setTimeout(() => {
          if (activeRequest.current !== data.request_id) return
          activeRequest.current = undefined
          setEvidence((current) => ({ ...current, extension: 'disconnected' }))
          onErrorRef.current(copy.designManagement.newReview.extensionUnavailable)
        }, CAPTURE_RESULT_TIMEOUT_MS)
        setEvidence((current) => ({ ...current, extension: 'capturing' }))
        return
      }
      if (data.type === 'PM_DESIGN_REVIEW_CAPTURE_ERROR') {
        const extensionInvalidated = typeof data.error === 'string' && /Extension context invalidated/i.test(data.error)
        finishRequest(
          data.extension_disconnected === true || extensionInvalidated ? 'disconnected' : 'connected',
          data.iframe_fallback_available === true && !extensionInvalidated,
        )
        onErrorRef.current(extensionInvalidated
          ? copy.designManagement.newReview.extensionUnavailable
          : typeof data.error === 'string' ? data.error : copy.designManagement.newReview.captureFailed)
        return
      }
      if (data.type === 'PM_DESIGN_REVIEW_NORMALIZATION_INVENTORY') {
        receiveNormalizationInventory(data)
        return
      }
      if (data.type === 'PM_DESIGN_REVIEW_CAPTURE_RESULT') receiveCapture(data)
    }

    const finishRequest = (
      extension: NewReviewEvidence['extension'] = 'connected',
      iframeFallbackAvailable = false,
    ) => {
      activeRequest.current = undefined
      clearStartupTimeout()
      clearCaptureTimeout()
      setEvidence((current) => ({ ...current, extension, iframeFallbackAvailable }))
    }

    const receiveNormalizationInventory = (data: Record<string, unknown>) => {
      if (typeof data.original_screenshot !== 'string' || !isRecord(data.inventory) || typeof data.request_id !== 'string') return
      try {
        const original = dataURLFile(data.original_screenshot, 'implementation-original.png')
        const currentFigmaURL = figmaURLRef.current.trim()
        const inventory = data.inventory
        setEvidence((current) => ({
          ...current,
          implementationOriginal: original,
          inventory,
          normalizationFigmaUrl: currentFigmaURL,
          extension: 'normalizing',
        }))
        void normalizeDOMRef.current(currentFigmaURL, inventory)
          .then((plan) => window.postMessage({
            source: 'pm-design-review-web',
            type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION',
            request_id: data.request_id,
            plan,
          }, window.origin))
          .catch((error: unknown) => {
            abortRequest(data.request_id as string)
            finishRequest()
            onErrorRef.current(error instanceof Error ? error.message : copy.designManagement.newReview.normalizationFailed)
          })
      } catch {
        abortRequest(data.request_id)
        finishRequest()
        onErrorRef.current(copy.designManagement.newReview.captureFailed)
      }
    }

    const receiveCapture = (data: Record<string, unknown>) => {
      if (typeof data.screenshot !== 'string') return
      try {
        const file = dataURLFile(data.screenshot, 'implementation-capture.png')
        const original = typeof data.original_screenshot === 'string'
          ? dataURLFile(data.original_screenshot, 'implementation-original.png')
          : undefined
        const inventory = isRecord(data.inventory) ? data.inventory : undefined
        const normalization = isRecord(data.dom_normalization) ? data.dom_normalization : undefined
        const manifest = isRecord(normalization?.report)
          ? new File([JSON.stringify(normalization, null, 2)], 'dom-normalization-manifest.json', { type: 'application/json' })
          : undefined
        activeRequest.current = undefined
        clearStartupTimeout()
        clearCaptureTimeout()
        setEvidence((current) => ({
          ...current,
          implementation: file,
          implementationOriginal: original ?? current.implementationOriginal,
          inventory,
          normalizationManifest: manifest,
          webCapture: webCaptureStructure(data),
          captureViewport: captureViewport(data.viewport),
          extension: 'captured',
        }))
      } catch {
        finishRequest()
        onErrorRef.current(copy.designManagement.newReview.captureFailed)
      }
    }

    window.addEventListener('message', listener)
    window.postMessage({ source: 'pm-design-review-web', type: 'PM_DESIGN_REVIEW_PING' }, window.origin)
    const timeout = window.setTimeout(() => setEvidence((current) => (
      current.extension === 'checking' ? { ...current, extension: 'disconnected' } : current
    )), 1200)
    return () => {
      window.clearTimeout(timeout)
      clearStartupTimeout()
      clearCaptureTimeout()
      window.removeEventListener('message', listener)
      cancelActiveRequest()
    }
  }, [])

  return { evidence, setEvidence, beginCaptureRequest, cancelActiveRequest }
}

function abortRequest(requestID: string) {
  window.postMessage({
    source: 'pm-design-review-web',
    type: 'PM_DESIGN_REVIEW_ABORT_NORMALIZATION',
    request_id: requestID,
  }, window.origin)
}

function dataURLFile(value: string, name: string): File {
  const [header, body = ''] = value.split(',', 2)
  if (!header.startsWith('data:') || !body) throw new Error('invalid capture data URL')
  const type = /data:([^;]+)/.exec(header)?.[1] ?? 'image/png'
  const bytes = Uint8Array.from(atob(body), (character) => character.charCodeAt(0))
  return new File([bytes], name, { type })
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function captureViewport(value: unknown): NewReviewEvidence['captureViewport'] {
  if (!isRecord(value)) return undefined
  const { width, height, devicePixelRatio } = value
  if (typeof width !== 'number' || !Number.isInteger(width) || typeof height !== 'number' || !Number.isInteger(height) || typeof devicePixelRatio !== 'number') return undefined
  return { width, height, devicePixelRatio }
}
