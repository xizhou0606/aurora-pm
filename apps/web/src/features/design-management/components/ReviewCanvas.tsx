import { useRef, type PointerEvent } from 'react'
import { PhImageBroken as ImageOff } from '@/shared/ui/icons'

import { copy } from '../copy'
import { cn } from '@/shared/lib/utils'
import { canvasZoomClass, type DesignDifference, type Region } from '../model'

type Marker = Pick<DesignDifference, 'id' | 'ordinal' | 'title' | 'severity'> & { region: Region }

type Props = {
  side: 'design' | 'implementation'
  label: string
  imageURL?: string
  width: number
  height: number
  markers: readonly Marker[]
  selectedId?: string
  selection?: Region
  zoom: number
  rulers: boolean
  horizontalGuides: readonly number[]
  verticalGuides: readonly number[]
  onSelect: (id: string) => void
  onSelectRegion: (side: Props['side'], region: Region) => void
  onAddGuide: (side: Props['side'], axis: 'x' | 'y', coordinate: number) => void
  onRemoveGuide: (side: Props['side'], axis: 'x' | 'y', coordinate: number) => void
}

export function ReviewCanvas({
  side,
  label,
  imageURL,
  width,
  height,
  markers,
  selectedId,
  selection,
  zoom,
  rulers,
  horizontalGuides,
  verticalGuides,
  onSelect,
  onSelectRegion,
  onAddGuide,
  onRemoveGuide,
}: Props) {
  const origin = useRef<{ x: number; y: number } | undefined>(undefined)
  const overlay = useRef<SVGSVGElement>(null)
  const coordinate = (event: PointerEvent<SVGSVGElement>) => {
    const bounds = overlay.current?.getBoundingClientRect()
    if (!bounds) return { x: 0, y: 0 }
    return {
      x: Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width)),
      y: Math.max(0, Math.min(1, (event.clientY - bounds.top) / bounds.height)),
    }
  }
  const start = (event: PointerEvent<SVGSVGElement>) => {
    const target = event.target as Element
    if (target.closest('[data-difference-marker], [data-alignment-guide]')) return
    const point = coordinate(event)
    const ruler = rulers ? target.closest('[data-ruler-axis]')?.getAttribute('data-ruler-axis') : undefined
    if (ruler === 'x') {
      onAddGuide(side, 'x', Math.round(point.x * width))
      return
    }
    if (ruler === 'y') {
      onAddGuide(side, 'y', Math.round(point.y * height))
      return
    }
    origin.current = point
    event.currentTarget.setPointerCapture(event.pointerId)
  }
  const finish = (event: PointerEvent<SVGSVGElement>) => {
    const started = origin.current
    origin.current = undefined
    if (!started) return
    const ended = coordinate(event)
    const region = {
      x: Math.min(started.x, ended.x),
      y: Math.min(started.y, ended.y),
      width: Math.abs(started.x - ended.x),
      height: Math.abs(started.y - ended.y),
    }
    if (region.width * width >= 16 && region.height * height >= 16) onSelectRegion(side, region)
  }
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
      <header className="flex h-9 shrink-0 items-center justify-between border-b border-border bg-card px-3">
        <span className="truncate text-xs font-medium">{label}</span>
        <span className="text-xs text-muted-foreground">{copy.designManagement.common.dimension(width, height)}</span>
      </header>
      <div className="relative min-h-64 min-w-0 flex-1 overflow-auto bg-muted/40 p-5">
        {imageURL ? (
          <div className={cn('relative inline-block origin-top-left transition-transform duration-150 motion-reduce:transition-none', canvasZoomClass(zoom))}>
            <img src={imageURL} alt={label} draggable={false} className="block max-w-none select-none" />
            <svg
              ref={overlay}
              role="application"
              aria-label={copy.designManagement.detail.selectionHint}
              viewBox={`0 0 ${width} ${height}`}
              className="absolute inset-0 size-full touch-none"
              onPointerDown={start}
              onPointerUp={finish}
              onPointerCancel={() => { origin.current = undefined }}
            >
              {rulers ? <CanvasRulers width={width} height={height} horizontalGuides={horizontalGuides} verticalGuides={verticalGuides} onAdd={(axis, value) => onAddGuide(side, axis, value)} onRemove={(axis, value) => onRemoveGuide(side, axis, value)} /> : null}
              {markers.map((marker) => <g key={marker.id} data-difference-marker="true" role="button" tabIndex={0} aria-label={copy.designManagement.differences.ordinal(marker.ordinal)} onClick={() => onSelect(marker.id)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(marker.id) } }} className="cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <rect x={marker.region.x * width} y={marker.region.y * height} width={marker.region.width * width} height={marker.region.height * height} rx="4" className={marker.id === selectedId ? 'fill-primary/20 stroke-primary stroke-[3]' : marker.severity === 'critical' ? 'fill-red-500/15 stroke-red-500 stroke-2' : marker.severity === 'major' ? 'fill-amber-500/15 stroke-amber-500 stroke-2' : 'fill-blue-500/10 stroke-blue-500 stroke-2'} />
                <text x={marker.region.x * width + 6} y={marker.region.y * height + 18} className="fill-foreground text-xs font-semibold">{copy.designManagement.common.ordinalShort(marker.ordinal)}</text>
                <title>{marker.title}</title>
              </g>)}
              {selection ? <rect x={selection.x * width} y={selection.y * height} width={selection.width * width} height={selection.height * height} rx="3" className="pointer-events-none fill-emerald-500/15 stroke-emerald-500 stroke-[3] stroke-dasharray-[8_6]" /> : null}
            </svg>
          </div>
        ) : <div className="grid min-h-64 place-items-center text-muted-foreground"><div className="grid justify-items-center gap-2"><ImageOff className="size-9" /><span className="text-xs">{copy.designManagement.detail.assetUnavailable}</span></div></div>}
      </div>
    </section>
  )
}

function CanvasRulers({ width, height, horizontalGuides, verticalGuides, onAdd, onRemove }: {
  width: number
  height: number
  horizontalGuides: readonly number[]
  verticalGuides: readonly number[]
  onAdd: (axis: 'x' | 'y', coordinate: number) => void
  onRemove: (axis: 'x' | 'y', coordinate: number) => void
}) {
  const xTicks = rulerTicks(width)
  const yTicks = rulerTicks(height)
  return <g data-testid="canvas-rulers">
    <g className="pointer-events-none stroke-border/40" aria-hidden="true">
      {xTicks.filter((tick) => tick.major).map((tick) => <line key={`grid-x-${tick.coordinate}`} x1={tick.coordinate} y1="24" x2={tick.coordinate} y2={height} vectorEffect="non-scaling-stroke" />)}
      {yTicks.filter((tick) => tick.major).map((tick) => <line key={`grid-y-${tick.coordinate}`} x1="24" y1={tick.coordinate} x2={width} y2={tick.coordinate} vectorEffect="non-scaling-stroke" />)}
    </g>
    <g data-ruler-axis="x" role="button" tabIndex={0} aria-label={copy.designManagement.detail.horizontalRuler} className="outline-none focus-visible:ring-2 focus-visible:ring-ring" onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onAdd('x', nextGuide(verticalGuides, width)) } }}><rect x="0" y="0" width={width} height="24" className="cursor-s-resize fill-card/95 stroke-border" vectorEffect="non-scaling-stroke" /></g>
    <g data-ruler-axis="y" role="button" tabIndex={0} aria-label={copy.designManagement.detail.verticalRuler} className="outline-none focus-visible:ring-2 focus-visible:ring-ring" onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onAdd('y', nextGuide(horizontalGuides, height)) } }}><rect x="0" y="0" width="24" height={height} className="cursor-e-resize fill-card/95 stroke-border" vectorEffect="non-scaling-stroke" /></g>
    <g className="pointer-events-none fill-muted-foreground stroke-muted-foreground" aria-hidden="true">
      {xTicks.map((tick) => <g key={`tick-x-${tick.coordinate}`}><line x1={tick.coordinate} y1={tick.major ? 8 : 16} x2={tick.coordinate} y2="24" vectorEffect="non-scaling-stroke" />{tick.label ? <text x={tick.coordinate + 3} y="11" className="text-xs">{tick.label}</text> : null}</g>)}
      {yTicks.map((tick) => <g key={`tick-y-${tick.coordinate}`}><line x1={tick.major ? 8 : 16} y1={tick.coordinate} x2="24" y2={tick.coordinate} vectorEffect="non-scaling-stroke" />{tick.label ? <text x="4" y={tick.coordinate + 12} className="text-xs">{tick.label}</text> : null}</g>)}
    </g>
    {verticalGuides.map((coordinate) => <Guide key={`guide-x-${coordinate}`} axis="x" coordinate={coordinate} width={width} height={height} onRemove={onRemove} />)}
    {horizontalGuides.map((coordinate) => <Guide key={`guide-y-${coordinate}`} axis="y" coordinate={coordinate} width={width} height={height} onRemove={onRemove} />)}
  </g>
}

function Guide({ axis, coordinate, width, height, onRemove }: { axis: 'x' | 'y'; coordinate: number; width: number; height: number; onRemove: (axis: 'x' | 'y', coordinate: number) => void }) {
  const label = copy.designManagement.detail.guide(axis.toUpperCase(), coordinate)
  const remove = () => onRemove(axis, coordinate)
  return <g data-alignment-guide="true" role="button" tabIndex={0} aria-label={label} className="cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={(event) => { event.stopPropagation(); remove() }} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); remove() } }}>
    <line x1={axis === 'x' ? coordinate : 24} y1={axis === 'x' ? 24 : coordinate} x2={axis === 'x' ? coordinate : width} y2={axis === 'x' ? height : coordinate} className="stroke-transparent stroke-[12]" vectorEffect="non-scaling-stroke" />
    <line x1={axis === 'x' ? coordinate : 24} y1={axis === 'x' ? 24 : coordinate} x2={axis === 'x' ? coordinate : width} y2={axis === 'x' ? height : coordinate} className="stroke-amber-500 stroke-2" vectorEffect="non-scaling-stroke" />
    <title>{label}</title>
  </g>
}

function rulerTicks(length: number) {
  return Array.from({ length: Math.floor(length / 50) + 1 }, (_, index) => ({
    coordinate: index * 50,
    major: index % 2 === 0,
    label: index > 0 && index % 2 === 0 ? String(index * 50) : undefined,
  }))
}

function nextGuide(guides: readonly number[], length: number) {
  return Array.from({ length: Math.max(1, Math.floor(length / 100)) }, (_, index) => (index + 1) * 100)
    .find((coordinate) => coordinate < length && !guides.includes(coordinate)) ?? Math.max(0, Math.round(length / 2))
}
