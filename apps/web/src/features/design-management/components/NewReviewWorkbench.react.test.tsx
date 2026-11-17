import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '../copy'
import type { FigmaPreview } from '../model'
import { NewReviewWorkbench } from './NewReviewWorkbench'
import type { NewReviewWorkbenchProps } from './new-review-contract'

const figmaURL = 'https://www.figma.com/design/figma-file/Checkout?node-id=1-2'
const implementationURL = 'https://implementation.example/checkout'

function workbenchProps(preview?: FigmaPreview): NewReviewWorkbenchProps {
  return {
    requirements: [],
    requirementsState: 'loading',
    onRetryRequirements: vi.fn(),
    preview,
    previewState: preview ? 'ready' : 'idle',
    submitState: 'idle',
    collectors: [],
    captures: [],
    mobileCaptureState: 'idle',
    onPreview: vi.fn(),
    onResetPreview: vi.fn(),
    onNormalizeDOM: vi.fn(),
    onStartMobileCapture: vi.fn(),
    onLoadMobileCapture: vi.fn(),
    onSubmit: vi.fn(),
  }
}

describe('NewReviewWorkbench', () => {
  it('validates malformed Figma links before requesting a preview', () => {
    const props = workbenchProps()
    render(<NewReviewWorkbench {...props} />)

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.figmaUrl), {
      target: { value: 'https://www.figma.com/design/figma-file/Checkout' },
    })
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.newReview.preview }))

    expect(props.onPreview).not.toHaveBeenCalled()
    expect(screen.getByText(copy.designManagement.newReview.invalidFigma)).toBeTruthy()
  })

  it('requests a preview for a valid Figma node link', () => {
    const props = workbenchProps()
    render(<NewReviewWorkbench {...props} />)

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.figmaUrl), {
      target: { value: `  ${figmaURL}  ` },
    })
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.newReview.preview }))

    expect(props.onPreview).toHaveBeenCalledWith(figmaURL)
  })

  it('clears stale preview state when the Figma link changes', () => {
    const props = workbenchProps()
    render(<NewReviewWorkbench {...props} />)

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.figmaUrl), {
      target: { value: figmaURL },
    })

    expect(props.onResetPreview).toHaveBeenCalledTimes(1)
  })

  it('shows the preview error selected by the API status instead of blaming the URL', () => {
    render(<NewReviewWorkbench
      {...workbenchProps()}
      previewState="error"
      previewErrorKey="figmaPreviewUnavailable"
    />)

    expect(screen.getByText(copy.designManagement.newReview.figmaPreviewUnavailable)).toBeTruthy()
    expect(screen.queryByText(copy.designManagement.newReview.invalidFigma)).toBeNull()
  })

  it('uses the current Figma node dimensions as the implementation iframe viewport', async () => {
    const preview: FigmaPreview = {
      file_key: 'figma-file',
      node_id: '1:2',
      node_name: 'Checkout mobile',
      image_url: 'https://cdn.example/design.png',
      width: 390,
      height: 844,
    }
    const view = render(<NewReviewWorkbench {...workbenchProps()} />)

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.figmaUrl), {
      target: { value: figmaURL },
    })
    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.implementationUrl), {
      target: { value: implementationURL },
    })
    view.rerender(<NewReviewWorkbench {...workbenchProps(preview)} />)

    await waitFor(() => {
      expect((screen.getByLabelText(copy.designManagement.newReview.viewportWidth) as HTMLInputElement).value).toBe('390')
      expect((screen.getByLabelText(copy.designManagement.newReview.viewportHeight) as HTMLInputElement).value).toBe('844')
    })
    const iframe = screen.getByTitle(copy.designManagement.newReview.implementationUrl) as HTMLIFrameElement
    expect(iframe.id).toBe('pm-design-review-implementation-frame')
    expect(iframe.style.getPropertyValue('--frame-viewport-w')).toBe('390px')
    expect(iframe.style.getPropertyValue('--frame-viewport-h')).toBe('844px')
    expect(iframe.className).toContain('origin-top-left')
    expect(screen.getByTestId('implementation-frame-viewport').classList.contains('overflow-hidden')).toBe(true)
    expect(iframe.getAttribute('sandbox')).toBeNull()

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.viewportWidth), {
      target: { value: '412' },
    })
    view.rerender(<NewReviewWorkbench {...workbenchProps({ ...preview })} />)
    expect((screen.getByLabelText(copy.designManagement.newReview.viewportWidth) as HTMLInputElement).value).toBe('412')

    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.figmaUrl), {
      target: { value: 'https://www.figma.com/design/other-file/Checkout?node-id=4-5' },
    })
    view.rerender(<NewReviewWorkbench {...workbenchProps(preview)} />)
    expect((screen.getByLabelText(copy.designManagement.newReview.viewportWidth) as HTMLInputElement).value).toBe('412')
  })

  it('requests direct capture from the stable implementation iframe', async () => {
    const postMessage = vi.spyOn(window, 'postMessage')
    render(<NewReviewWorkbench {...workbenchProps()} />)
    fireEvent.change(screen.getByLabelText(copy.designManagement.newReview.implementationUrl), {
      target: { value: implementationURL },
    })
    fireEvent(window, new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: { source: 'pm-design-review-extension', type: 'PM_DESIGN_REVIEW_READY', version: '0.1.0' },
    }))

    const captureButton = screen.getByRole('button', { name: copy.designManagement.newReview.captureTarget }) as HTMLButtonElement
    await waitFor(() => expect(captureButton.disabled).toBe(false))
    fireEvent.click(captureButton)

    const request = postMessage.mock.calls.map(([message]) => message).find((message) => (
      typeof message === 'object' && message?.type === 'PM_DESIGN_REVIEW_CAPTURE_REQUEST'
    ))
    expect(request).toMatchObject({
      source: 'pm-design-review-web',
      payload: {
        url: implementationURL,
        frame_selector: '#pm-design-review-implementation-frame',
        viewport: { width: 1440, height: 900, devicePixelRatio: 1 },
      },
    })
    const requestID = (request as { request_id: string }).request_id
    fireEvent(window, new MessageEvent('message', {
      source: window,
      origin: window.origin,
      data: {
        source: 'pm-design-review-extension',
        type: 'PM_DESIGN_REVIEW_CAPTURE_ERROR',
        request_id: requestID,
        error: '无法读取嵌入页面',
        iframe_fallback_available: true,
      },
    }))
    const fallbackButton = await screen.findByRole('button', { name: copy.designManagement.newReview.captureTargetFallback })
    fireEvent.click(fallbackButton)
    expect(postMessage.mock.calls.map(([message]) => message).some((message) => (
      typeof message === 'object' && message?.type === 'PM_DESIGN_REVIEW_START_TAB_CAPTURE'
    ))).toBe(true)
    postMessage.mockRestore()
  })
})
