export const MAX_ELEMENTS = 1500
export const MAX_STYLE_VALUE_LENGTH = 300

export const STYLE_PROPERTIES = [
  'display', 'position', 'z-index', 'overflow', 'overflow-x', 'overflow-y',
  'margin-top', 'margin-right', 'margin-bottom', 'margin-left',
  'padding-top', 'padding-right', 'padding-bottom', 'padding-left',
  'border-top-width', 'border-right-width', 'border-bottom-width', 'border-left-width',
  'border-top-color', 'border-right-color', 'border-bottom-color', 'border-left-color',
  'border-radius', 'background-color', 'background-image', 'color', 'opacity', 'box-shadow',
  'font-family', 'font-size', 'font-weight', 'line-height', 'letter-spacing', 'text-align',
  'object-fit', 'object-position', 'transform',
] as const

export function validCaptureURL(value: string) {
  try { return ['http:', 'https:'].includes(new URL(value).protocol) } catch { return false }
}

export function validViewport(value: unknown): value is { width: number; height: number; devicePixelRatio: number } {
  if (!value || typeof value !== 'object') return false
  const viewport = value as Record<string, unknown>
  return Number.isInteger(viewport.width) && Number(viewport.width) >= 240 && Number(viewport.width) <= 7680
    && Number.isInteger(viewport.height) && Number(viewport.height) >= 240 && Number(viewport.height) <= 7680
    && typeof viewport.devicePixelRatio === 'number' && viewport.devicePixelRatio > 0 && viewport.devicePixelRatio <= 8
}
