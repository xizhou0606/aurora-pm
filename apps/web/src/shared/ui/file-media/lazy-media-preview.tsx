import {
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from 'react'
import { PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { ResponsivePanel } from '../layout/responsive-panel'
import { Button } from '../primitives/button'

const MIN_IMAGE_SCALE = 0.5
const MAX_IMAGE_SCALE = 4
const IMAGE_SCALE_STEP = 0.2

type ImageOffset = Readonly<{ x: number; y: number }>

function clampImageOffset(
  offset: ImageOffset,
  scale: number,
  stage: HTMLDivElement | null,
  image: HTMLImageElement | null,
): ImageOffset {
  if (scale <= 1 || !stage || !image) return { x: 0, y: 0 }
  const maxX = Math.max(0, (image.clientWidth * scale - stage.clientWidth) / 2)
  const maxY = Math.max(0, (image.clientHeight * scale - stage.clientHeight) / 2)
  return {
    x: Math.min(maxX, Math.max(-maxX, offset.x)),
    y: Math.min(maxY, Math.max(-maxY, offset.y)),
  }
}

export interface LazyMediaPreviewProps {
  open: boolean
  url: string
  title: string
  description?: string
  closeLabel?: string
  alt?: string
  onOpenChange: (open: boolean) => void
  finalFocus?: RefObject<HTMLElement | null> | (() => HTMLElement | null)
  navigation?: {
    current: number
    total: number
    onPrevious: () => void
    onNext: () => void
  }
}

function isVideo(url: string): boolean {
  return /\.(mp4|webm|ogg)(?:$|[?#])/i.test(url)
}

export function LazyMediaPreview({
  open,
  url,
  title,
  description = copy.ui.mediaPreview.description,
  closeLabel,
  alt = title,
  onOpenChange,
  finalFocus,
  navigation,
}: LazyMediaPreviewProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const imageRef = useRef<HTMLImageElement>(null)
  const dragRef = useRef<{
    pointerId: number
    startX: number
    startY: number
    offset: ImageOffset
  } | null>(null)
  const [imageStage, setImageStage] = useState<HTMLDivElement | null>(null)
  const [imageScale, setImageScale] = useState(1)
  const [imageOffset, setImageOffset] = useState<ImageOffset>({ x: 0, y: 0 })
  const [imageDragging, setImageDragging] = useState(false)

  useEffect(() => {
    setImageScale(1)
    setImageOffset({ x: 0, y: 0 })
    setImageDragging(false)
    dragRef.current = null
  }, [open, url])

  useEffect(() => {
    if (open) return
    videoRef.current?.pause()
    if (videoRef.current) videoRef.current.currentTime = 0
  }, [open])

  useEffect(() => {
    if (!open || !navigation || navigation.total <= 1) return undefined
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return
      if (
        event.target instanceof HTMLInputElement
        || event.target instanceof HTMLTextAreaElement
        || event.target instanceof HTMLSelectElement
        || (event.target instanceof HTMLElement && event.target.isContentEditable)
      ) return
      if (event.key === 'ArrowLeft') {
        event.preventDefault()
        event.stopPropagation()
        navigation.onPrevious()
      } else if (event.key === 'ArrowRight') {
        event.preventDefault()
        event.stopPropagation()
        navigation.onNext()
      }
    }
    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [navigation, open])

  useEffect(() => {
    if (!imageStage || !open || !url || isVideo(url)) return undefined
    const handleWheel = (event: WheelEvent) => {
      event.preventDefault()
      event.stopPropagation()
      if (!event.deltaY) return
      setImageScale((current) => {
        const direction = event.deltaY < 0 ? 1 : -1
        const next = Number((current + direction * IMAGE_SCALE_STEP).toFixed(2))
        const scale = Math.min(MAX_IMAGE_SCALE, Math.max(MIN_IMAGE_SCALE, next))
        setImageOffset((offset) => clampImageOffset(
          offset,
          scale,
          imageStage,
          imageRef.current,
        ))
        return scale
      })
    }
    imageStage.addEventListener('wheel', handleWheel, { passive: false })
    return () => imageStage.removeEventListener('wheel', handleWheel)
  }, [imageStage, open, url])

  const startImageDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (imageScale <= 1 || event.button !== 0) return
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      offset: imageOffset,
    }
    setImageDragging(true)
  }
  const moveImage = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return
    event.preventDefault()
    setImageOffset(clampImageOffset({
      x: drag.offset.x + event.clientX - drag.startX,
      y: drag.offset.y + event.clientY - drag.startY,
    }, imageScale, imageStage, imageRef.current))
  }
  const stopImageDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current
    if (!drag || drag.pointerId !== event.pointerId) return
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId)
    }
    dragRef.current = null
    setImageDragging(false)
  }
  const loseImagePointer = () => {
    dragRef.current = null
    setImageDragging(false)
  }

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      closeLabel={closeLabel}
      finalFocus={finalFocus}
      height="viewport"
      className="md:h-[92dvh] md:w-[94vw] md:max-w-none"
      contentClassName="relative min-h-0 overflow-hidden overscroll-none bg-foreground/5"
    >
      <div
        data-slot="media-preview-stage"
        className="absolute inset-4 grid min-h-0 place-items-center overflow-hidden"
      >
        {url ? (
          isVideo(url) ? (
            <video key={url} ref={videoRef} controls autoPlay playsInline preload="metadata" className="max-h-[calc(100dvh-8rem)] max-w-full rounded-lg" aria-label={alt}>
              <source src={url} />
            </video>
          ) : (
            <div
              ref={setImageStage}
              data-slot="media-preview-image-stage"
              className={`grid size-full min-h-0 touch-none place-items-center overflow-hidden overscroll-none ${
                imageScale > 1
                  ? imageDragging ? 'cursor-grabbing' : 'cursor-grab'
                  : 'cursor-zoom-in'
              }`}
              onPointerDown={startImageDrag}
              onPointerMove={moveImage}
              onPointerUp={stopImageDrag}
              onPointerCancel={stopImageDrag}
              onLostPointerCapture={loseImagePointer}
            >
              <div
                data-slot="media-preview-image-pan"
                className={cn(
                  'translate-x-(--media-pan-x) translate-y-(--media-pan-y)',
                  imageDragging ? '' : 'transition-transform duration-100 ease-out',
                )}
                style={{
                  '--media-pan-x': `${imageOffset.x}px`,
                  '--media-pan-y': `${imageOffset.y}px`,
                }}
              >
                <img
                  ref={imageRef}
                  src={url}
                  alt={alt}
                  draggable={false}
                  data-scale={imageScale}
                  className="max-h-[calc(100dvh-8rem)] max-w-full scale-(--media-image-scale) select-none rounded-lg object-contain transition-transform duration-100 ease-out"
                  style={{ '--media-image-scale': `${imageScale}` }}
                />
              </div>
            </div>
          )
        ) : (
          <p className="text-sm text-muted-foreground">{copy.ui.mediaPreview.unavailable}</p>
        )}
        {navigation && navigation.total > 1 ? (
          <>
            <Button
              type="button"
              size="icon-lg"
              variant="secondary"
              aria-label={copy.ui.mediaPreview.previous}
              className="absolute top-1/2 left-3 z-10 -translate-y-1/2 rounded-full bg-background/95 shadow-md hover:bg-background"
              onClick={navigation.onPrevious}
            >
              <PhCaretLeft aria-hidden="true" />
            </Button>
            <Button
              type="button"
              size="icon-lg"
              variant="secondary"
              aria-label={copy.ui.mediaPreview.next}
              className="absolute top-1/2 right-3 z-10 -translate-y-1/2 rounded-full bg-background/95 shadow-md hover:bg-background"
              onClick={navigation.onNext}
            >
              <PhCaretRight aria-hidden="true" />
            </Button>
            <span
              role="status"
              className="absolute bottom-3 left-1/2 z-10 -translate-x-1/2 rounded-full bg-foreground/70 px-2.5 py-1 text-xs font-medium text-background shadow-sm backdrop-blur"
            >
              {copy.ui.mediaPreview.position(navigation.current, navigation.total)}
            </span>
          </>
        ) : null}
      </div>
    </ResponsivePanel>
  )
}
