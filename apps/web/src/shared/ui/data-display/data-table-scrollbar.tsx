import {
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent,
  type RefObject,
} from 'react'

export interface DataTableScrollbarState {
  verticalTrackRef: RefObject<HTMLDivElement | null>
  verticalThumbRef: RefObject<HTMLDivElement | null>
  horizontalTrackRef: RefObject<HTMLDivElement | null>
  horizontalThumbRef: RefObject<HTMLDivElement | null>
  actionShadowVisible: boolean
  rootRef: RefObject<HTMLDivElement | null>
  handleVerticalPointerDown: (event: PointerEvent<HTMLDivElement>) => void
  handleVerticalPointerMove: (event: PointerEvent<HTMLDivElement>) => void
  handleVerticalPointerUp: (event: PointerEvent<HTMLDivElement>) => void
  handleVerticalKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
  handleHorizontalPointerDown: (event: PointerEvent<HTMLDivElement>) => void
  handleHorizontalPointerMove: (event: PointerEvent<HTMLDivElement>) => void
  handleHorizontalPointerUp: (event: PointerEvent<HTMLDivElement>) => void
  handleHorizontalKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
}

export function useDataTableScrollbar({
  bodyScrollEnabled,
  columnCount,
  hasActionColumn,
  status,
}: {
  bodyScrollEnabled: boolean
  columnCount: number
  hasActionColumn: boolean
  status: string
}): DataTableScrollbarState {
  const scrollContainerRef = useRef<HTMLElement | null>(null)
  const verticalTrackRef = useRef<HTMLDivElement>(null)
  const verticalThumbRef = useRef<HTMLDivElement>(null)
  const horizontalTrackRef = useRef<HTMLDivElement>(null)
  const horizontalThumbRef = useRef<HTMLDivElement>(null)
  const verticalDragRef = useRef<{
    pointerId: number; startClientY: number; startScrollTop: number
  } | null>(null)
  const horizontalDragRef = useRef<{
    pointerId: number; startClientX: number; startScrollLeft: number
  } | null>(null)
  const [actionShadowVisible, setActionShadowVisible] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const scrollContainer = rootRef.current?.querySelector<HTMLElement>(
      '[data-slot="table-container"]',
    )
    const tableHeader = scrollContainer?.querySelector<HTMLElement>(
      '[data-slot="table-header"]',
    )
    if (!scrollContainer) {
      scrollContainerRef.current = null
      setActionShadowVisible(false)
      return undefined
    }
    scrollContainerRef.current = scrollContainer

    const updateScrollState = () => {
      const remainingScroll =
        scrollContainer.scrollWidth - scrollContainer.clientWidth - scrollContainer.scrollLeft
      setActionShadowVisible(hasActionColumn && remainingScroll > 1)

      const vTrack = verticalTrackRef.current
      const vThumb = verticalThumbRef.current
      const hTrack = horizontalTrackRef.current
      const hThumb = horizontalThumbRef.current
      if (!vTrack || !vThumb || !hTrack || !hThumb) return

      const headerHeight = tableHeader?.offsetHeight ?? 0
      const maxScrollLeft = Math.max(0, scrollContainer.scrollWidth - scrollContainer.clientWidth)
      const hVisible = maxScrollLeft > 1
      const hScrollbarH = hVisible ? 8 : 0
      const bodyViewportH = Math.max(0, scrollContainer.clientHeight - headerHeight)
      const bodyContentH = Math.max(bodyViewportH, scrollContainer.scrollHeight - headerHeight)
      const trackH = Math.max(0, bodyViewportH - hScrollbarH)
      const maxScrollTop = Math.max(0, scrollContainer.scrollHeight - scrollContainer.clientHeight)
      const vVisible = maxScrollTop > 1 && trackH > 0
      const thumbH = vVisible
        ? Math.min(trackH, Math.max(28, (trackH * bodyViewportH) / bodyContentH))
        : trackH
      const maxThumbOffset = Math.max(0, trackH - thumbH)
      const thumbOffset = maxScrollTop > 0
        ? Math.min(maxThumbOffset, (scrollContainer.scrollTop / maxScrollTop) * maxThumbOffset)
        : 0

      // Vertical track update
      vTrack.hidden = !vVisible
      vTrack.style.top = `${headerHeight}px`
      vTrack.style.height = `${trackH}px`
      vTrack.setAttribute('aria-valuemax', String(Math.round(maxScrollTop)))
      vTrack.setAttribute('aria-valuenow', String(Math.round(scrollContainer.scrollTop)))
      vThumb.style.height = `${thumbH}px`
      vThumb.style.transform = `translateY(${thumbOffset}px)`

      // Horizontal track update
      const hTrackW = Math.max(0, scrollContainer.clientWidth - (vVisible ? 8 : 0) - 8)
      const hThumbW = hVisible
        ? Math.min(hTrackW, Math.max(28, (hTrackW * scrollContainer.clientWidth) / scrollContainer.scrollWidth))
        : hTrackW
      const maxHThumbOffset = Math.max(0, hTrackW - hThumbW)
      const hThumbOffset = maxScrollLeft > 0
        ? Math.min(maxHThumbOffset, (scrollContainer.scrollLeft / maxScrollLeft) * maxHThumbOffset)
        : 0

      hTrack.hidden = !hVisible
      hTrack.style.width = `${hTrackW}px`
      hTrack.setAttribute('aria-valuemax', String(Math.round(maxScrollLeft)))
      hTrack.setAttribute('aria-valuenow', String(Math.round(scrollContainer.scrollLeft)))
      hThumb.style.width = `${hThumbW}px`
      hThumb.style.transform = `translateX(${hThumbOffset}px)`
    }

    updateScrollState()
    scrollContainer.addEventListener('scroll', updateScrollState, { passive: true })

    const resizeObserver = typeof ResizeObserver === 'undefined'
      ? undefined
      : new ResizeObserver(updateScrollState)
    resizeObserver?.observe(scrollContainer)
    const tableElement = scrollContainer.querySelector('table')
    if (tableElement) resizeObserver?.observe(tableElement)
    if (tableHeader) resizeObserver?.observe(tableHeader)

    return () => {
      scrollContainer.removeEventListener('scroll', updateScrollState)
      resizeObserver?.disconnect()
      if (scrollContainerRef.current === scrollContainer) scrollContainerRef.current = null
    }
  }, [bodyScrollEnabled, columnCount, hasActionColumn, status])

  // ---- Vertical scrollbar handlers ----

  const scrollFromVerticalTrack = (clientY: number) => {
    const sc = scrollContainerRef.current
    const track = verticalTrackRef.current
    const thumb = verticalThumbRef.current
    if (!sc || !track || !thumb) return
    const trackRect = track.getBoundingClientRect()
    const thumbH = thumb.offsetHeight
    const maxOffset = Math.max(0, track.clientHeight - thumbH)
    const maxScroll = Math.max(0, sc.scrollHeight - sc.clientHeight)
    const offset = Math.min(maxOffset, Math.max(0, clientY - trackRect.top - thumbH / 2))
    sc.scrollTop = maxOffset > 0 ? (offset / maxOffset) * maxScroll : 0
  }

  const handleVerticalPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    const sc = scrollContainerRef.current
    if (!sc || event.button !== 0) return
    event.preventDefault()
    if (event.target === verticalThumbRef.current) {
      verticalDragRef.current = {
        pointerId: event.pointerId,
        startClientY: event.clientY,
        startScrollTop: sc.scrollTop,
      }
      event.currentTarget.setPointerCapture?.(event.pointerId)
      return
    }
    scrollFromVerticalTrack(event.clientY)
  }

  const handleVerticalPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = verticalDragRef.current
    const sc = scrollContainerRef.current
    const track = verticalTrackRef.current
    const thumb = verticalThumbRef.current
    if (!drag || drag.pointerId !== event.pointerId || !sc || !track || !thumb) return
    const maxOffset = Math.max(0, track.clientHeight - thumb.offsetHeight)
    const maxScroll = Math.max(0, sc.scrollHeight - sc.clientHeight)
    if (maxOffset <= 0) return
    sc.scrollTop = Math.min(
      maxScroll,
      Math.max(0, drag.startScrollTop + ((event.clientY - drag.startClientY) / maxOffset) * maxScroll),
    )
  }

  const handleVerticalPointerUp = (event: PointerEvent<HTMLDivElement>) => {
    if (verticalDragRef.current?.pointerId !== event.pointerId) return
    verticalDragRef.current = null
    event.currentTarget.releasePointerCapture?.(event.pointerId)
  }

  const handleVerticalKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const sc = scrollContainerRef.current
    if (!sc) return
    const pageStep = Math.max(40, sc.clientHeight / 2)
    const next = {
      ArrowUp: sc.scrollTop - 40,
      ArrowDown: sc.scrollTop + 40,
      PageUp: sc.scrollTop - pageStep,
      PageDown: sc.scrollTop + pageStep,
      Home: 0,
      End: sc.scrollHeight,
    }[event.key]
    if (next === undefined) return
    event.preventDefault()
    sc.scrollTop = next
  }

  // ---- Horizontal scrollbar handlers ----

  const scrollFromHorizontalTrack = (clientX: number) => {
    const sc = scrollContainerRef.current
    const track = horizontalTrackRef.current
    const thumb = horizontalThumbRef.current
    if (!sc || !track || !thumb) return
    const trackRect = track.getBoundingClientRect()
    const thumbW = thumb.offsetWidth
    const maxOffset = Math.max(0, track.clientWidth - thumbW)
    const maxScroll = Math.max(0, sc.scrollWidth - sc.clientWidth)
    const offset = Math.min(maxOffset, Math.max(0, clientX - trackRect.left - thumbW / 2))
    sc.scrollLeft = maxOffset > 0 ? (offset / maxOffset) * maxScroll : 0
  }

  const handleHorizontalPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    const sc = scrollContainerRef.current
    if (!sc || event.button !== 0) return
    event.preventDefault()
    if (event.target === horizontalThumbRef.current) {
      horizontalDragRef.current = {
        pointerId: event.pointerId,
        startClientX: event.clientX,
        startScrollLeft: sc.scrollLeft,
      }
      event.currentTarget.setPointerCapture?.(event.pointerId)
      return
    }
    scrollFromHorizontalTrack(event.clientX)
  }

  const handleHorizontalPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = horizontalDragRef.current
    const sc = scrollContainerRef.current
    const track = horizontalTrackRef.current
    const thumb = horizontalThumbRef.current
    if (!drag || drag.pointerId !== event.pointerId || !sc || !track || !thumb) return
    const maxOffset = Math.max(0, track.clientWidth - thumb.offsetWidth)
    const maxScroll = Math.max(0, sc.scrollWidth - sc.clientWidth)
    if (maxOffset <= 0) return
    sc.scrollLeft = Math.min(
      maxScroll,
      Math.max(0, drag.startScrollLeft + ((event.clientX - drag.startClientX) / maxOffset) * maxScroll),
    )
  }

  const handleHorizontalPointerUp = (event: PointerEvent<HTMLDivElement>) => {
    if (horizontalDragRef.current?.pointerId !== event.pointerId) return
    horizontalDragRef.current = null
    event.currentTarget.releasePointerCapture?.(event.pointerId)
  }

  const handleHorizontalKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const sc = scrollContainerRef.current
    if (!sc) return
    const pageStep = Math.max(40, sc.clientWidth / 2)
    const next = {
      ArrowLeft: sc.scrollLeft - 40,
      ArrowRight: sc.scrollLeft + 40,
      PageUp: sc.scrollLeft - pageStep,
      PageDown: sc.scrollLeft + pageStep,
      Home: 0,
      End: sc.scrollWidth,
    }[event.key]
    if (next === undefined) return
    event.preventDefault()
    sc.scrollLeft = next
  }

  return {
    verticalTrackRef,
    verticalThumbRef,
    horizontalTrackRef,
    horizontalThumbRef,
    actionShadowVisible,
    rootRef,
    handleVerticalPointerDown,
    handleVerticalPointerMove,
    handleVerticalPointerUp,
    handleVerticalKeyDown,
    handleHorizontalPointerDown,
    handleHorizontalPointerMove,
    handleHorizontalPointerUp,
    handleHorizontalKeyDown,
  }
}