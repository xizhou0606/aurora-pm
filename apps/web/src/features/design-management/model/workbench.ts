import type { CanvasMode, Region } from './types'

export type WorkbenchState = {
  canvasMode: CanvasMode
  narrowSide: 'design' | 'implementation'
  zoom: number
  rulers: boolean
  selectedDifferenceId?: string
  differenceDrawerOpen: boolean
  selections: { design?: Region; implementation?: Region }
  guides: { horizontal: number[]; design: number[]; implementation: number[] }
}

export type WorkbenchEvent =
  | { type: 'canvas-mode'; mode: CanvasMode }
  | { type: 'narrow-side'; side: WorkbenchState['narrowSide'] }
  | { type: 'zoom'; value: number }
  | { type: 'toggle-rulers' }
  | { type: 'select-difference'; id?: string }
  | { type: 'toggle-drawer'; open: boolean }
  | { type: 'select-region'; side: 'design' | 'implementation'; region: Region }
  | { type: 'clear-region' }
  | { type: 'add-guide'; side: 'design' | 'implementation'; axis: 'x' | 'y'; coordinate: number }
  | { type: 'remove-guide'; side: 'design' | 'implementation'; axis: 'x' | 'y'; coordinate: number }

export const initialWorkbenchState: WorkbenchState = {
  canvasMode: 'compare', narrowSide: 'design', zoom: 1, rulers: false,
  differenceDrawerOpen: true, selections: {},
  guides: { horizontal: [], design: [], implementation: [] },
}

const zoomClasses: Record<string, string> = {
  '0.25': 'scale-[.25]',
  '0.5': 'scale-50',
  '0.75': 'scale-75',
  '1': 'scale-100',
  '1.25': 'scale-125',
  '1.5': 'scale-150',
  '1.75': 'scale-[1.75]',
  '2': 'scale-200',
}

export function canvasZoomClass(zoom: number): string {
  return zoomClasses[String(zoom)] ?? 'scale-100'
}

export function workbenchReducer(state: WorkbenchState, event: WorkbenchEvent): WorkbenchState {
  switch (event.type) {
    case 'canvas-mode': return { ...state, canvasMode: event.mode }
    case 'narrow-side': return { ...state, narrowSide: event.side }
    case 'zoom': return { ...state, zoom: Math.min(2, Math.max(0.25, event.value)) }
    case 'toggle-rulers': return { ...state, rulers: !state.rulers }
    case 'select-difference': return { ...state, selectedDifferenceId: event.id, differenceDrawerOpen: true }
    case 'toggle-drawer': return { ...state, differenceDrawerOpen: event.open }
    case 'select-region': return { ...state, selections: { ...state.selections, [event.side]: event.region } }
    case 'clear-region': return { ...state, selections: {} }
    case 'add-guide': {
      const key = event.axis === 'y' ? 'horizontal' : event.side
      if (state.guides[key].includes(event.coordinate)) return state
      return { ...state, guides: { ...state.guides, [key]: [...state.guides[key], event.coordinate].sort((a, b) => a - b) } }
    }
    case 'remove-guide': {
      const key = event.axis === 'y' ? 'horizontal' : event.side
      return { ...state, guides: { ...state.guides, [key]: state.guides[key].filter((value) => value !== event.coordinate) } }
    }
  }
}
