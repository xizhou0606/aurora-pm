export type CaptureRect = { x: number; y: number; width: number; height: number }
export type CaptureViewport = { width: number; height: number }

const RECT_TOLERANCE = 1
const SCALE_TOLERANCE = 0.03

function snapshotRect(rect: CaptureRect): CaptureRect {
  return { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
}

export function captureRectMoved(before: CaptureRect, after: CaptureRect, tolerance = RECT_TOLERANCE) {
  return Math.abs(before.x - after.x) > tolerance
    || Math.abs(before.y - after.y) > tolerance
    || Math.abs(before.width - after.width) > tolerance
    || Math.abs(before.height - after.height) > tolerance
}

export function renderedCaptureScale(rect: CaptureRect, viewport: CaptureViewport) {
  if (viewport.width <= 0 || viewport.height <= 0 || rect.width <= 0 || rect.height <= 0) {
    throw new Error('实现页面截图区域无效')
  }
  const scaleX = rect.width / viewport.width
  const scaleY = rect.height / viewport.height
  if (Math.abs(scaleX - scaleY) > SCALE_TOLERANCE) throw new Error('实现页面不是等比缩放，请刷新后重试')
  return (scaleX + scaleY) / 2
}

export function captureRectIsVisible(rect: CaptureRect, viewport: CaptureViewport, tolerance = RECT_TOLERANCE) {
  return rect.x >= -tolerance
    && rect.y >= -tolerance
    && rect.x + rect.width <= viewport.width + tolerance
    && rect.y + rect.height <= viewport.height + tolerance
}

export async function captureWithStableRect<T>({
  measure,
  capture,
  settle,
  attempts = 2,
}: {
  measure: () => CaptureRect
  capture: () => Promise<T>
  settle: () => Promise<void>
  attempts?: number
}) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    await settle()
    const before = snapshotRect(measure())
    const value = await capture()
    const after = snapshotRect(measure())
    if (!captureRectMoved(before, after)) return { value, rect: before }
  }
  throw new Error('截图期间实现页面发生移动，请保持窗口稳定后重试')
}
