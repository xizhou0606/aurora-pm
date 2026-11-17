import { useLayoutEffect, useMemo, useRef, useState } from 'react'

import { copy } from '../copy'

const FRAME_VIEWPORT_MARGIN = 64

type Size = { width: number; height: number }

type Props = {
  url: string
  viewportWidth: number
  viewportHeight: number
}

export function fittedFrameScale(viewport: Size, available: Size): number {
  if (viewport.width <= 0 || viewport.height <= 0) return 1
  const width = available.width > 0 ? available.width : viewport.width
  const height = available.height > 0 ? available.height : viewport.height
  return Math.min(1, width / viewport.width, height / viewport.height)
}

export function ImplementationFramePreview({ url, viewportWidth, viewportHeight }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [available, setAvailable] = useState<Size>({ width: 0, height: 0 })

  useLayoutEffect(() => {
    const container = containerRef.current
    if (!container) return
    const measure = () => setAvailable({
      width: container.clientWidth,
      height: Math.max(1, window.innerHeight - FRAME_VIEWPORT_MARGIN),
    })
    measure()
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
    observer?.observe(container)
    window.addEventListener('resize', measure)
    return () => {
      observer?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [])

  const scale = useMemo(
    () => fittedFrameScale(
      { width: viewportWidth, height: viewportHeight },
      available,
    ),
    [available, viewportHeight, viewportWidth],
  )
  const renderedWidth = viewportWidth * scale
  const renderedHeight = viewportHeight * scale

  return (
    <div ref={containerRef} className="grid min-h-72 w-full place-items-center overflow-hidden">
      <div
        data-design-review-capture-frame
        data-testid="implementation-frame-viewport"
        data-rendered-scale={scale}
        className="h-(--frame-rendered-h) w-(--frame-rendered-w) overflow-hidden bg-background shadow-sm"
        style={{ '--frame-rendered-w': `${renderedWidth}px`, '--frame-rendered-h': `${renderedHeight}px` }}
      >
        <iframe
          id="pm-design-review-implementation-frame"
          key={url}
          title={copy.designManagement.newReview.implementationUrl}
          src={url}
          referrerPolicy="no-referrer"
          className="block h-(--frame-viewport-h) w-(--frame-viewport-w) origin-top-left scale-(--frame-scale) shrink-0 border-0 bg-background"
          style={{
            '--frame-viewport-w': `${viewportWidth}px`,
            '--frame-viewport-h': `${viewportHeight}px`,
            '--frame-scale': scale,
          }}
        />
      </div>
    </div>
  )
}
