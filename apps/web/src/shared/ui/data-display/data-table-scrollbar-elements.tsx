import type { KeyboardEvent, PointerEvent, RefObject } from 'react'

import { copy } from '@/shared/copy'

const dataTableCopy = copy.ui.dataTable

export function DataTableScrollbarElements({
  bodyScrollEnabled,
  verticalTrackRef,
  verticalThumbRef,
  horizontalTrackRef,
  horizontalThumbRef,
  onVerticalKeyDown,
  onVerticalPointerDown,
  onVerticalPointerMove,
  onVerticalPointerUp,
  onHorizontalKeyDown,
  onHorizontalPointerDown,
  onHorizontalPointerMove,
  onHorizontalPointerUp,
}: {
  bodyScrollEnabled: boolean
  verticalTrackRef: RefObject<HTMLDivElement | null>
  verticalThumbRef: RefObject<HTMLDivElement | null>
  horizontalTrackRef: RefObject<HTMLDivElement | null>
  horizontalThumbRef: RefObject<HTMLDivElement | null>
  onVerticalKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
  onVerticalPointerDown: (event: PointerEvent<HTMLDivElement>) => void
  onVerticalPointerMove: (event: PointerEvent<HTMLDivElement>) => void
  onVerticalPointerUp: (event: PointerEvent<HTMLDivElement>) => void
  onHorizontalKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void
  onHorizontalPointerDown: (event: PointerEvent<HTMLDivElement>) => void
  onHorizontalPointerMove: (event: PointerEvent<HTMLDivElement>) => void
  onHorizontalPointerUp: (event: PointerEvent<HTMLDivElement>) => void
}) {
  if (!bodyScrollEnabled) return null

  return (
    <>
      <div
        ref={verticalTrackRef}
        data-slot="table-body-scrollbar"
        role="scrollbar"
        aria-label={dataTableCopy.verticalScrollbarLabel}
        aria-orientation="vertical"
        aria-valuemin={0}
        className="absolute right-1 z-40 w-1 touch-none bg-transparent"
        hidden
        tabIndex={0}
        onKeyDown={onVerticalKeyDown}
        onPointerDown={onVerticalPointerDown}
        onPointerMove={onVerticalPointerMove}
        onPointerUp={onVerticalPointerUp}
        onPointerCancel={onVerticalPointerUp}
      >
        <div
          ref={verticalThumbRef}
          data-slot="table-body-scrollbar-thumb"
          className="absolute inset-x-0 top-0 min-h-7 rounded-full bg-[var(--pm-scrollbar-thumb)] transition-colors hover:bg-[var(--pm-scrollbar-thumb-hover)]"
        />
      </div>
      <div
        ref={horizontalTrackRef}
        data-slot="table-horizontal-scrollbar"
        role="scrollbar"
        aria-label={dataTableCopy.horizontalScrollbarLabel}
        aria-orientation="horizontal"
        aria-valuemin={0}
        className="absolute bottom-1 left-1 z-40 h-1 touch-none bg-transparent"
        hidden
        tabIndex={0}
        onKeyDown={onHorizontalKeyDown}
        onPointerDown={onHorizontalPointerDown}
        onPointerMove={onHorizontalPointerMove}
        onPointerUp={onHorizontalPointerUp}
        onPointerCancel={onHorizontalPointerUp}
      >
        <div
          ref={horizontalThumbRef}
          data-slot="table-horizontal-scrollbar-thumb"
          className="absolute inset-y-0 left-0 min-w-7 rounded-full bg-[var(--pm-scrollbar-thumb)] transition-colors hover:bg-[var(--pm-scrollbar-thumb-hover)]"
        />
      </div>
    </>
  )
}