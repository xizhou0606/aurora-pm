import { MAX_ELEMENTS, MAX_STYLE_VALUE_LENGTH, STYLE_PROPERTIES, validCaptureURL, validViewport } from './capture-policy'
import { captureRectIsVisible, captureWithStableRect, renderedCaptureScale, type CaptureRect } from './capture-geometry'
import { captureResultMessage } from './capture-message'
import { parseNormalizationPlan } from './normalization-policy'
import type { CaptureEvidence, CaptureWarning, NormalizationControlState, NormalizationOperation, NormalizationPlan, NormalizationReport, VisualElement } from './types'

const WEB_SOURCE = 'pm-design-review-web'
const EXTENSION_SOURCE = 'pm-design-review-extension'
const FRAME_SOURCE = 'pm-design-review-extension-frame'
const FRAME_COMMAND_SOURCE = 'pm-design-review-extension-top'
const IMPLEMENTATION_FRAME_SELECTOR = '#pm-design-review-implementation-frame'
const STORAGE_MARKER = 'pmDesignReviewCaptureStyleV1'
const FREEZE_TIMEOUT_MS = 180_000
const FRAME_PREPARE_TIMEOUT_MS = 15_000
const FRAME_CAPTURE_TIMEOUT_MS = 15_000

type FrozenPage = {
  restore: () => void
  timeout: number
  elements: Map<string, Element>
  inventoryElements: Map<string, VisualElement>
  mutations: Array<() => void>
  inventoryToken?: string
}
const frozenPages = new Map<string, FrozenPage>()

function hashString(input: string) {
  let hash = 0xcbf29ce484222325n
  for (let index = 0; index < input.length; index += 1) {
    hash ^= BigInt(input.charCodeAt(index))
    hash = BigInt.asUintN(64, hash * 0x100000001b3n)
  }
  return hash.toString(16).padStart(16, '0')
}

function directText(element: Element) {
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement) return null
  const text = Array.from(element.childNodes)
    .filter((node) => node.nodeType === Node.TEXT_NODE)
    .map((node) => node.textContent?.trim() ?? '')
    .filter(Boolean)
    .join(' ')
  return text.slice(0, 240) || null
}

function controlState(element: Element): VisualElement['control_state'] {
  const isNative = element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement || element instanceof HTMLSelectElement
  const ariaChecked = element.getAttribute('aria-checked')
  const ariaPressed = element.getAttribute('aria-pressed')
  const ariaSelected = element.getAttribute('aria-selected')
  if (!isNative && ariaChecked === null && ariaPressed === null && ariaSelected === null && !element.hasAttribute('data-state')) return null
  return {
    checked: element instanceof HTMLInputElement && ['checkbox', 'radio'].includes(element.type) ? element.checked : null,
    aria_checked: ['true', 'false', 'mixed'].includes(ariaChecked ?? '') ? ariaChecked : null,
    aria_pressed: ['true', 'false', 'mixed'].includes(ariaPressed ?? '') ? ariaPressed : null,
    aria_selected: ['true', 'false'].includes(ariaSelected ?? '') ? ariaSelected : null,
    data_state: element.getAttribute('data-state')?.slice(0, 120) ?? null,
  }
}

function elementFingerprint(element: Element, parentID: string | null) {
  const rect = element.getBoundingClientRect()
  return hashString(JSON.stringify({
    tag: element.tagName.toLowerCase(), role: element.getAttribute('role'), parentID,
    text: directText(element), state: controlState(element),
    bounds: [Math.round(rect.x * 10), Math.round(rect.y * 10), Math.round(rect.width * 10), Math.round(rect.height * 10)],
  }))
}

function visible(element: Element, rect: DOMRect, style: CSSStyleDeclaration) {
  return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden'
    && Number(style.opacity) !== 0 && rect.bottom > 0 && rect.right > 0 && rect.top < innerHeight && rect.left < innerWidth
}

function collectElements() {
  const candidates: Element[] = []
  const walker = document.createTreeWalker(document.body ?? document.documentElement, NodeFilter.SHOW_ELEMENT)
  let current = walker.currentNode as Element | null
  while (current) {
    const rect = current.getBoundingClientRect()
    const style = getComputedStyle(current)
    if (visible(current, rect, style)) candidates.push(current)
    current = walker.nextNode() as Element | null
  }
  const selected = candidates.slice(0, MAX_ELEMENTS)
  const identifiers = new Map(selected.map((element, index) => [element, `el-${index + 1}`]))
  let truncatedStyles = 0
  const elements = selected.map((element): VisualElement => {
    const rect = element.getBoundingClientRect()
    const style = getComputedStyle(element)
    const styles = Object.fromEntries(STYLE_PROPERTIES.flatMap((property) => {
      const raw = style.getPropertyValue(property)
      if (!raw) return []
      if (raw.length > MAX_STYLE_VALUE_LENGTH) truncatedStyles += 1
      return [[property, raw.slice(0, MAX_STYLE_VALUE_LENGTH)]]
    }))
    let parent = element.parentElement
    while (parent && !identifiers.has(parent)) parent = parent.parentElement
    const parentID = parent ? identifiers.get(parent) ?? null : null
    const text = directText(element)
    const state = controlState(element)
    const allowedOperations: VisualElement['allowed_operations'] = []
    if (text !== null && element.children.length === 0) allowedOperations.push('set_text')
    if (state !== null) allowedOperations.push('set_control_state')
    const siblings = element.parentElement
      ? Array.from(element.parentElement.children).filter((item) => item.tagName === element.tagName)
      : []
    if (siblings.length > 1 && siblings.length <= 100 && parentID) allowedOperations.push('repeat_template')
    const fingerprint = elementFingerprint(element, parentID)
    return {
      id: identifiers.get(element)!, parent_id: parentID, tag: element.tagName.toLowerCase(),
      role: element.getAttribute('role'), text, bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
      styles, clipped: rect.top < 0 || rect.left < 0 || rect.bottom > innerHeight || rect.right > innerWidth,
      structural_fingerprint: fingerprint, control_state: state, allowed_operations: allowedOperations,
    }
  })
  return { elements, elementMap: new Map(selected.map((element) => [identifiers.get(element)!, element])), truncated: candidates.length > MAX_ELEMENTS || truncatedStyles > 0, candidateCount: candidates.length, truncatedStyles }
}

async function stabilize() {
  const warnings: CaptureWarning[] = []
  const timeout = (duration: number) => new Promise<'timeout'>((resolve) => setTimeout(() => resolve('timeout'), duration))
  if ('fonts' in document && await Promise.race([document.fonts.ready.then(() => 'ready' as const), timeout(5000)]) === 'timeout') {
    warnings.push({ code: 'fonts_pending', message: '字体在 5 秒内未完成加载' })
  }
  const images = Array.from(document.images).filter((image) => !image.complete)
  if (images.length && await Promise.race([Promise.allSettled(images.map((image) => image.decode())).then(() => 'ready' as const), timeout(5000)]) === 'timeout') {
    warnings.push({ code: 'images_pending', message: `${images.length} 张图片在 5 秒内未完成加载` })
  }
  if (document.querySelector('video')) warnings.push({ code: 'video_present', message: '目标视口包含视频，视频帧无法冻结' })
  if (document.querySelector('canvas')) warnings.push({ code: 'canvas_present', message: '目标视口包含 Canvas，结构证据可能不完整' })
  return warnings
}

function freeze(requestID: string) {
  release(requestID)
  const style = document.createElement('style')
  style.dataset[STORAGE_MARKER] = 'true'
  style.textContent = '*,*::before,*::after{animation-play-state:paused!important;transition-duration:0s!important;caret-color:transparent!important}'
  document.documentElement.append(style)
  const timeout = window.setTimeout(() => release(requestID), FREEZE_TIMEOUT_MS)
  frozenPages.set(requestID, { restore: () => style.remove(), timeout, elements: new Map(), inventoryElements: new Map(), mutations: [] })
}

function release(requestID: string) {
  const frozen = frozenPages.get(requestID)
  if (!frozen) return
  window.clearTimeout(frozen.timeout)
  for (const restore of [...frozen.mutations].reverse()) {
    try { restore() } catch {}
  }
  frozen.restore()
  frozenPages.delete(requestID)
}

async function prepareCapture(requestID: string, originalURL: string, domNormalization: boolean) {
  const warnings = await stabilize()
  freeze(requestID)
  await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  const collected = collectElements()
  const frozen = frozenPages.get(requestID)
  if (!frozen) throw new Error('采集会话已失效')
  frozen.elements = collected.elementMap
  frozen.inventoryElements = new Map(collected.elements.map((element) => [element.id, element]))
  if (collected.truncated) warnings.push({
    code: 'structure_truncated',
    message: `结构证据已按白名单裁剪（可见元素 ${collected.candidateCount}，过长样式 ${collected.truncatedStyles}）`,
  })
  const capturedAt = new Date().toISOString()
  const token = crypto.randomUUID()
  frozen.inventoryToken = token
  return {
    request_id: requestID,
    original_url: originalURL,
    actual_url: location.href,
    captured_at: capturedAt,
    elements: collected.elements,
    structure_truncated: collected.truncated,
    warnings,
    frame_viewport: { width: innerWidth, height: innerHeight, devicePixelRatio: devicePixelRatio || 1 },
    inventory: {
      version: '1.0' as const,
      token,
      captured_at: capturedAt,
      page_fingerprint: hashString(collected.elements.map((item) => `${item.id}:${item.structural_fingerprint}`).join('|')),
      elements: collected.elements,
      truncated: collected.truncated,
    },
    dom_normalization: { requested: domNormalization },
  }
}

function writeAttribute(element: Element, name: 'aria-checked' | 'aria-pressed' | 'aria-selected' | 'data-state', value: string | null) {
  if (value === null) element.removeAttribute(name)
  else element.setAttribute(name, value)
}

function applyText(element: Element, text: string) {
  if (element.children.length > 0) throw new Error('目标不是叶子文本')
  const nodes = Array.from(element.childNodes).filter((node): node is Text => node.nodeType === Node.TEXT_NODE)
  if (nodes.length === 0) throw new Error('目标没有文本节点')
  const before = nodes.map((node) => node.data)
  nodes[0]!.data = text
  nodes.slice(1).forEach((node) => { node.data = '' })
  return () => nodes.forEach((node, index) => { node.data = before[index] ?? '' })
}

function applyControl(element: Element, state: NormalizationControlState) {
  const before = controlState(element)
  if (!before) throw new Error('目标不是受支持控件')
  if (element instanceof HTMLInputElement && state.checked !== null && ['checkbox', 'radio'].includes(element.type)) element.checked = state.checked
  writeAttribute(element, 'aria-checked', state.aria_checked)
  writeAttribute(element, 'aria-pressed', state.aria_pressed)
  writeAttribute(element, 'aria-selected', state.aria_selected)
  writeAttribute(element, 'data-state', state.data_state)
  return () => {
    if (element instanceof HTMLInputElement && before.checked !== null) element.checked = before.checked
    writeAttribute(element, 'aria-checked', before.aria_checked)
    writeAttribute(element, 'aria-pressed', before.aria_pressed)
    writeAttribute(element, 'aria-selected', before.aria_selected)
    writeAttribute(element, 'data-state', before.data_state)
  }
}

function elementPath(root: Element, target: Element) {
  const path: number[] = []
  let current: Element | null = target
  while (current && current !== root) {
    const parent: Element | null = current.parentElement
    if (!parent) return null
    path.unshift(Array.from(parent.children).indexOf(current))
    current = parent
  }
  return current === root ? path : null
}

function resolvePath(root: Element, path: number[]) {
  let current: Element = root
  for (const index of path) {
    const child = current.children.item(index)
    if (!child) return null
    current = child
  }
  return current
}

function applyRepeated(operation: Extract<NormalizationOperation, { type: 'replace_repeated_items' }>, session: FrozenPage) {
  const container = session.elements.get(operation.container_id)
  const template = session.elements.get(operation.template_id)
  const containerInventory = session.inventoryElements.get(operation.container_id)
  const templateInventory = session.inventoryElements.get(operation.template_id)
  if (!container || !template || template.parentElement !== container) throw new Error('重复项模板失效')
  if (!containerInventory || !templateInventory?.allowed_operations.includes('repeat_template')) throw new Error('重复项操作不在采集白名单')
  const valuePaths = new Map<string, number[]>()
  for (const item of operation.items) for (const value of item.values) {
    if (valuePaths.has(value.target_id)) continue
    const target = session.elements.get(value.target_id)
    const targetInventory = session.inventoryElements.get(value.target_id)
    const path = target ? elementPath(template, target) : null
    if (!path || !targetInventory
      || (value.text !== null && !targetInventory.allowed_operations.includes('set_text'))
      || (value.control_state !== null && !targetInventory.allowed_operations.includes('set_control_state'))) throw new Error('重复项值不属于允许的模板字段')
    valuePaths.set(value.target_id, path)
  }
  const signature = `${template.tagName}|${template.getAttribute('role') ?? ''}`
  const originals = Array.from(container.children).filter((child) => `${child.tagName}|${child.getAttribute('role') ?? ''}` === signature)
  const marker = document.createComment('pm-design-review-repeat-marker')
  container.insertBefore(marker, originals[0] ?? null)
  originals.forEach((node) => node.remove())
  const clones: Element[] = []
  try {
    for (const item of operation.items) {
      const clone = template.cloneNode(true) as Element
      for (const value of item.values) {
        const target = resolvePath(clone, valuePaths.get(value.target_id)!)
        if (!target) throw new Error('重复项克隆结构变化')
        if (value.text !== null) applyText(target, value.text)
        if (value.control_state !== null) applyControl(target, value.control_state)
      }
      container.insertBefore(clone, marker)
      clones.push(clone)
    }
  } catch (error) {
    clones.forEach((node) => node.remove())
    originals.forEach((node) => container.insertBefore(node, marker))
    marker.remove()
    throw error
  }
  return () => {
    clones.forEach((node) => node.remove())
    originals.forEach((node) => container.insertBefore(node, marker))
    marker.remove()
  }
}

async function applyNormalization(requestID: string, plan: NormalizationPlan) {
  const session = frozenPages.get(requestID)
  const validatedPlan = parseNormalizationPlan(plan)
  if (!session || !validatedPlan || session.inventoryToken !== validatedPlan.inventory_token) throw new Error('归一化方案与采集清单不匹配')
  const reports: NormalizationReport['operations'] = []
  const usedTargets = new Set<string>()
  for (const operation of validatedPlan.operations) {
    const targetID = operation.type === 'replace_repeated_items' ? operation.template_id : operation.target_id
    const target = session.elements.get(targetID)
    const inventoryTarget = session.inventoryElements.get(targetID)
    let reason: string | null = null
    if (operation.confidence < 0.9) reason = '置信度低于安全阈值'
    else if (usedTargets.has(targetID)) reason = '目标元素被重复修改'
    else if (!target?.isConnected) reason = '目标元素已失效'
    else if (!inventoryTarget) reason = '目标元素不在采集清单'
    else if (operation.type === 'set_text' && !inventoryTarget.allowed_operations.includes('set_text')) reason = '文本操作不在采集白名单'
    else if (operation.type === 'set_control_state' && !inventoryTarget.allowed_operations.includes('set_control_state')) reason = '控件操作不在采集白名单'
    else {
      const expected = operation.type === 'replace_repeated_items' ? operation.expected_template_fingerprint : operation.expected_fingerprint
      if (elementFingerprint(target, inventoryTarget.parent_id) !== expected) reason = '目标元素指纹已变化'
      if (!reason && operation.type === 'replace_repeated_items') {
        const container = session.elements.get(operation.container_id)
        const containerInventory = session.inventoryElements.get(operation.container_id)
        if (!container?.isConnected || !containerInventory || elementFingerprint(container, containerInventory.parent_id) !== operation.expected_container_fingerprint) reason = '重复项容器指纹已变化'
      }
    }
    try {
      if (!reason) {
        const restore = operation.type === 'set_text' ? applyText(target!, operation.text)
          : operation.type === 'set_control_state' ? applyControl(target!, operation.state)
            : applyRepeated(operation, session)
        session.mutations.push(restore)
        usedTargets.add(targetID)
      }
    } catch (error) {
      reason = error instanceof Error ? error.message : '扩展运行时校验失败'
    }
    reports.push({ type: operation.type, target_id: targetID, result: reason ? 'skipped' : 'applied', reason })
  }
  await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  const collected = collectElements()
  const applied = reports.filter((item) => item.result === 'applied').length
  if (validatedPlan.mismatch_detected && !validatedPlan.already_aligned && applied === 0) throw new Error('归一化方案未安全应用任何操作')
  return {
    elements: collected.elements, structure_truncated: collected.truncated,
    report: {
      plan: { model: validatedPlan.model, prompt_version: validatedPlan.prompt_version, already_aligned: validatedPlan.already_aligned, mismatch_detected: validatedPlan.mismatch_detected, summary: validatedPlan.summary, skipped: validatedPlan.skipped },
      operations: reports, counts: { applied, skipped: reports.length - applied + validatedPlan.skipped.length },
    } satisfies NormalizationReport,
  }
}

type RuntimeMessage =
  | { type: 'PM_DESIGN_REVIEW_PREPARE_CAPTURE'; request_id: string; original_url: string; dom_normalization: boolean }
  | { type: 'PM_DESIGN_REVIEW_RELEASE_CAPTURE'; request_id: string }
  | { type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION'; request_id: string; plan: NormalizationPlan }
  | { type: 'PM_DESIGN_REVIEW_DELIVER_NORMALIZATION_INVENTORY'; request_id: string; inventory: CaptureEvidence['inventory']; original_screenshot: string }
  | { type: 'PM_DESIGN_REVIEW_DELIVER_CAPTURE'; evidence: CaptureEvidence }
  | { type: 'PM_DESIGN_REVIEW_DELIVER_ERROR'; request_id: string; error: string }

type PreparedPageEvidence = Awaited<ReturnType<typeof prepareCapture>>
type FrameCommand =
  | { source: typeof FRAME_COMMAND_SOURCE; type: 'PM_DESIGN_REVIEW_PREPARE_FRAME'; request_id: string; frame_token: string; original_url: string; dom_normalization: boolean }
  | { source: typeof FRAME_COMMAND_SOURCE; type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION_FRAME'; request_id: string; frame_token: string; plan: NormalizationPlan }
  | { source: typeof FRAME_COMMAND_SOURCE; type: 'PM_DESIGN_REVIEW_RELEASE_FRAME'; request_id: string; frame_token: string }
type FrameResponse = (PreparedPageEvidence & {
  source: typeof FRAME_SOURCE
  type: 'PM_DESIGN_REVIEW_FRAME_READY'
  frame_token: string
}) | (Awaited<ReturnType<typeof applyNormalization>> & {
  source: typeof FRAME_SOURCE
  type: 'PM_DESIGN_REVIEW_NORMALIZATION_FRAME_APPLIED'
  request_id: string
  frame_token: string
}) | {
  source: typeof FRAME_SOURCE
  type: 'PM_DESIGN_REVIEW_FRAME_ERROR'
  request_id: string
  frame_token: string
  error: string
}

type DirectCaptureSession = {
  frame: HTMLIFrameElement
  originalURL: string
  viewport: { width: number; height: number; devicePixelRatio: number; pageZoom: number }
  prepared: PreparedPageEvidence
  originalScreenshot: string
  pageZoom: number
  frameToken: string
}

const directCaptureSessions = new Map<string, DirectCaptureSession>()

chrome.runtime.onMessage.addListener((message: RuntimeMessage, _sender, respond) => {
  if (message.type === 'PM_DESIGN_REVIEW_PREPARE_CAPTURE') {
    prepareCapture(message.request_id, message.original_url, message.dom_normalization)
      .then((evidence) => respond({ ok: true, evidence }))
      .catch((error: unknown) => { release(message.request_id); respond({ ok: false, error: error instanceof Error ? error.message : '页面准备失败' }) })
    return true
  }
  if (message.type === 'PM_DESIGN_REVIEW_RELEASE_CAPTURE') {
    release(message.request_id)
    respond({ ok: true })
    return false
  }
  if (message.type === 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION') {
    applyNormalization(message.request_id, message.plan).then((evidence) => respond({ ok: true, evidence })).catch((error: unknown) => respond({ ok: false, error: error instanceof Error ? error.message : '归一化应用失败' }))
    return true
  }
  if (message.type === 'PM_DESIGN_REVIEW_DELIVER_NORMALIZATION_INVENTORY' && window.top === window) {
    window.postMessage({ source: EXTENSION_SOURCE, type: 'PM_DESIGN_REVIEW_NORMALIZATION_INVENTORY', request_id: message.request_id, inventory: message.inventory, original_screenshot: message.original_screenshot }, window.origin)
    respond({ ok: true })
    return false
  }
  if (message.type === 'PM_DESIGN_REVIEW_DELIVER_CAPTURE' && window.top === window) {
    window.postMessage(captureResultMessage(message.evidence), window.origin)
    respond({ ok: true })
    return false
  }
  if (message.type === 'PM_DESIGN_REVIEW_DELIVER_ERROR' && window.top === window) {
    window.postMessage({ source: EXTENSION_SOURCE, type: 'PM_DESIGN_REVIEW_CAPTURE_ERROR', request_id: message.request_id, error: message.error }, window.origin)
    respond({ ok: true })
    return false
  }
  return false
})

function nextStablePaint() {
  return new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
}

function postCaptureError(requestID: string, error: unknown, fallbackAvailable = false) {
  window.postMessage({
    source: EXTENSION_SOURCE,
    type: 'PM_DESIGN_REVIEW_CAPTURE_ERROR',
    request_id: requestID,
    error: error instanceof Error ? error.message : '网页采集失败',
    iframe_fallback_available: fallbackAvailable,
  }, window.origin)
}

function postCaptureProgress(requestID: string, stage: string) {
  window.postMessage({
    source: EXTENSION_SOURCE,
    type: 'PM_DESIGN_REVIEW_CAPTURE_PROGRESS',
    request_id: requestID,
    stage,
  }, window.origin)
}

function withTimeout<T>(work: Promise<T>, duration: number, message: string) {
  return Promise.race([
    work,
    new Promise<never>((_, reject) => window.setTimeout(() => reject(new Error(message)), duration)),
  ])
}

function postFrameError(requestID: string, frameToken: string, error: unknown) {
  window.parent.postMessage({
    source: FRAME_SOURCE,
    type: 'PM_DESIGN_REVIEW_FRAME_ERROR',
    request_id: requestID,
    frame_token: frameToken,
    error: error instanceof Error ? error.message : '无法处理嵌入页面',
  } satisfies FrameResponse, '*')
}

function waitForFrame<T extends FrameResponse>(frame: HTMLIFrameElement, requestID: string, frameToken: string, expectedType: T['type']) {
  return new Promise<T>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      window.removeEventListener('message', listener)
      reject(new Error('无法读取嵌入页面。页面可能禁止 iframe，或扩展没有该域名权限。'))
    }, FRAME_PREPARE_TIMEOUT_MS)
    const listener = (event: MessageEvent<FrameResponse>) => {
      const data = event.data
      if (!frame.isConnected || !frame.contentWindow || !data || data.source !== FRAME_SOURCE || data.request_id !== requestID || data.frame_token !== frameToken) return
      if ('actual_url' in data && (!validCaptureURL(data.actual_url) || new URL(data.actual_url).origin !== event.origin)) return
      if (data.type === 'PM_DESIGN_REVIEW_FRAME_ERROR') {
        window.clearTimeout(timeout)
        window.removeEventListener('message', listener)
        reject(new Error(data.error))
        return
      }
      if (data.type !== expectedType) return
      window.clearTimeout(timeout)
      window.removeEventListener('message', listener)
      resolve(data as T)
    }
    window.addEventListener('message', listener)
  })
}

async function cropScreenshot(dataURL: string, rect: CaptureRect, viewport: { width: number; height: number }) {
  renderedCaptureScale(rect, viewport)
  if (!captureRectIsVisible(rect, { width: innerWidth, height: innerHeight })) {
    throw new Error('嵌入页面未完整显示在当前窗口中，请扩大窗口或使用标签页采集')
  }
  const image = await createImageBitmap(await (await fetch(dataURL)).blob())
  try {
    const screenScaleX = image.width / innerWidth
    const screenScaleY = image.height / innerHeight
    const sourceX = Math.max(0, Math.round(rect.x * screenScaleX))
    const sourceY = Math.max(0, Math.round(rect.y * screenScaleY))
    const sourceWidth = Math.min(image.width - sourceX, Math.round(rect.width * screenScaleX))
    const sourceHeight = Math.min(image.height - sourceY, Math.round(rect.height * screenScaleY))
    if (sourceWidth <= 0 || sourceHeight <= 0) throw new Error('实现页面截图区域不可见')
    const canvas = document.createElement('canvas')
    canvas.width = viewport.width
    canvas.height = viewport.height
    const context = canvas.getContext('2d', { alpha: false })
    if (!context) throw new Error('浏览器无法创建截图画布')
    context.imageSmoothingEnabled = true
    context.imageSmoothingQuality = 'high'
    context.drawImage(image, sourceX, sourceY, sourceWidth, sourceHeight, 0, 0, viewport.width, viewport.height)
    return canvas.toDataURL('image/png')
  } finally {
    image.close()
  }
}

async function captureCurrentFrame(requestID: string, frame: HTMLIFrameElement, viewport: { width: number; height: number }) {
  const captureFrame = frame.closest('[data-design-review-capture-frame]') ?? frame
  captureFrame.scrollIntoView({ block: 'center', inline: 'center' })
  postCaptureProgress(requestID, 'requesting_visible_tab')
  const { value: response, rect } = await withTimeout(captureWithStableRect({
    settle: nextStablePaint,
    measure: () => frame.getBoundingClientRect(),
    capture: () => chrome.runtime.sendMessage({ type: 'PM_DESIGN_REVIEW_CAPTURE_VISIBLE_TAB' }) as Promise<{
        ok: boolean
        screenshot?: string
        page_zoom?: number
        error?: string
      }>,
  }), FRAME_CAPTURE_TIMEOUT_MS, '浏览器截图超时，请重试或使用标签页采集')
  postCaptureProgress(requestID, 'visible_tab_received')
  if (!response?.ok || !response.screenshot) throw new Error(response?.error ?? '无法截取当前标签页')
  const pageZoom = response.page_zoom ?? 1
  if (Math.abs(pageZoom - 1) > 0.001) throw new Error(`页面缩放为 ${Math.round(pageZoom * 100)}%，请恢复到 100%`)
  postCaptureProgress(requestID, 'cropping')
  const screenshot = await withTimeout(
    cropScreenshot(response.screenshot, rect, viewport),
    FRAME_CAPTURE_TIMEOUT_MS,
    '浏览器截图裁剪超时，请重试或使用标签页采集',
  )
  postCaptureProgress(requestID, 'crop_complete')
  return { screenshot, pageZoom }
}

async function beginDirectCapture(
  requestID: string,
  originalURL: string,
  viewport: { width: number; height: number; devicePixelRatio: number; pageZoom: number },
  domNormalization: boolean,
  frame: HTMLIFrameElement,
) {
  const frameToken = crypto.randomUUID()
  try {
    const preparedPromise = waitForFrame<Extract<FrameResponse, { type: 'PM_DESIGN_REVIEW_FRAME_READY' }>>(
      frame,
      requestID,
      frameToken,
      'PM_DESIGN_REVIEW_FRAME_READY',
    )
    frame.contentWindow!.postMessage({
      source: FRAME_COMMAND_SOURCE,
      type: 'PM_DESIGN_REVIEW_PREPARE_FRAME',
      request_id: requestID,
      frame_token: frameToken,
      original_url: originalURL,
      dom_normalization: domNormalization,
    } satisfies FrameCommand, '*')
    const preparedMessage = await preparedPromise
    const {
      source: _source,
      type: _type,
      frame_token: _frameToken,
      ...prepared
    } = preparedMessage
    if (prepared.frame_viewport.width !== viewport.width || prepared.frame_viewport.height !== viewport.height) {
      throw new Error(`实现页面实测视口为 ${prepared.frame_viewport.width} × ${prepared.frame_viewport.height}，与目标 ${viewport.width} × ${viewport.height} 不一致`)
    }
    const original = await captureCurrentFrame(requestID, frame, viewport)
    if (domNormalization) {
      directCaptureSessions.set(requestID, {
        frame,
        originalURL,
        viewport,
        prepared,
        originalScreenshot: original.screenshot,
        pageZoom: original.pageZoom,
        frameToken,
      })
      window.postMessage({
        source: EXTENSION_SOURCE,
        type: 'PM_DESIGN_REVIEW_NORMALIZATION_INVENTORY',
        request_id: requestID,
        inventory: prepared.inventory,
        original_screenshot: original.screenshot,
      }, window.origin)
      return
    }
    const evidence: CaptureEvidence = {
      ...prepared,
      viewport: { ...viewport, pageZoom: original.pageZoom },
      screenshot: original.screenshot,
    }
    postCaptureProgress(requestID, 'delivering_result')
    window.postMessage(captureResultMessage(evidence), window.origin)
    frame.contentWindow!.postMessage({ source: FRAME_COMMAND_SOURCE, type: 'PM_DESIGN_REVIEW_RELEASE_FRAME', request_id: requestID, frame_token: frameToken } satisfies FrameCommand, '*')
  } catch (error) {
    frame.contentWindow?.postMessage({ source: FRAME_COMMAND_SOURCE, type: 'PM_DESIGN_REVIEW_RELEASE_FRAME', request_id: requestID, frame_token: frameToken } satisfies FrameCommand, '*')
    throw error
  }
}

async function applyDirectNormalization(requestID: string, plan: NormalizationPlan) {
  const session = directCaptureSessions.get(requestID)
  if (!session?.frame.contentWindow) throw new Error('嵌入页面归一化任务已失效')
  const appliedPromise = waitForFrame<Extract<FrameResponse, { type: 'PM_DESIGN_REVIEW_NORMALIZATION_FRAME_APPLIED' }>>(
    session.frame,
    requestID,
    session.frameToken,
    'PM_DESIGN_REVIEW_NORMALIZATION_FRAME_APPLIED',
  )
  session.frame.contentWindow.postMessage({
    source: FRAME_COMMAND_SOURCE,
    type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION_FRAME',
    request_id: requestID,
    frame_token: session.frameToken,
    plan,
  } satisfies FrameCommand, '*')
  const applied = await appliedPromise
  const normalized = await captureCurrentFrame(requestID, session.frame, session.viewport)
  const evidence: CaptureEvidence = {
    ...session.prepared,
    elements: applied.elements,
    structure_truncated: applied.structure_truncated,
    viewport: { ...session.viewport, pageZoom: normalized.pageZoom },
    screenshot: normalized.screenshot,
    original_screenshot: session.originalScreenshot,
    dom_normalization: { requested: true, report: applied.report },
  }
  postCaptureProgress(requestID, 'delivering_result')
  window.postMessage(captureResultMessage(evidence), window.origin)
  releaseDirectCapture(requestID)
}

function releaseDirectCapture(requestID: string) {
  const session = directCaptureSessions.get(requestID)
  session?.frame.contentWindow?.postMessage({
    source: FRAME_COMMAND_SOURCE,
    type: 'PM_DESIGN_REVIEW_RELEASE_FRAME',
    request_id: requestID,
    frame_token: session.frameToken,
  } satisfies FrameCommand, '*')
  directCaptureSessions.delete(requestID)
}

if (window.top !== window) {
  window.addEventListener('message', async (event: MessageEvent<FrameCommand>) => {
    if (event.data?.source !== FRAME_COMMAND_SOURCE) return
    const command = event.data
    if (command.type === 'PM_DESIGN_REVIEW_RELEASE_FRAME') {
      release(command.request_id)
      return
    }
    try {
      if (command.type === 'PM_DESIGN_REVIEW_PREPARE_FRAME') {
        const evidence = await prepareCapture(command.request_id, command.original_url, command.dom_normalization)
        window.parent.postMessage({ source: FRAME_SOURCE, type: 'PM_DESIGN_REVIEW_FRAME_READY', frame_token: command.frame_token, ...evidence } satisfies FrameResponse, '*')
      } else {
        const evidence = await applyNormalization(command.request_id, command.plan)
        window.parent.postMessage({
          source: FRAME_SOURCE,
          type: 'PM_DESIGN_REVIEW_NORMALIZATION_FRAME_APPLIED',
          request_id: command.request_id,
          frame_token: command.frame_token,
          ...evidence,
        } satisfies FrameResponse, '*')
      }
    } catch (error) {
      release(command.request_id)
      postFrameError(command.request_id, command.frame_token, error)
    }
  })
  window.parent.postMessage({ source: FRAME_SOURCE, type: 'PM_DESIGN_REVIEW_FRAME_LOADED', request_id: '' }, '*')
}

if (window.top === window) {
  const announce = async () => {
    try {
      const response = await chrome.runtime.sendMessage({ type: 'PM_DESIGN_REVIEW_GET_STATUS' }) as { ok?: boolean }
      if (response?.ok) window.postMessage({ source: EXTENSION_SOURCE, type: 'PM_DESIGN_REVIEW_READY', version: chrome.runtime.getManifest().version }, window.origin)
    } catch {
      // Reloading an extension invalidates existing content scripts. The PM page must be refreshed.
    }
  }
  void announce()
  window.addEventListener('message', async (event: MessageEvent) => {
    if (event.source !== window || event.origin !== window.origin || !event.data || typeof event.data !== 'object') return
    const data = event.data as Record<string, unknown>
    if (data.source !== WEB_SOURCE) return
    if (data.type === 'PM_DESIGN_REVIEW_PING') {
      void announce()
      return
    }
    const requestID = typeof data.request_id === 'string' ? data.request_id : ''
    if (data.type === 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION') {
      try {
        if (directCaptureSessions.has(requestID)) {
          await applyDirectNormalization(requestID, data.plan as NormalizationPlan)
          return
        }
        const response = await chrome.runtime.sendMessage({ type: data.type, request_id: requestID, plan: data.plan }) as { ok: boolean; error?: string }
        if (!response.ok) postCaptureError(requestID, response.error ?? '归一化操作失败')
      } catch (error) {
        releaseDirectCapture(requestID)
        postCaptureError(requestID, error, true)
      }
      return
    }
    if (data.type === 'PM_DESIGN_REVIEW_ABORT_NORMALIZATION') {
      if (directCaptureSessions.has(requestID)) {
        releaseDirectCapture(requestID)
        return
      }
      try {
        await chrome.runtime.sendMessage({ type: data.type, request_id: requestID })
      } catch {}
      return
    }
    if (data.type === 'PM_DESIGN_REVIEW_START_TAB_CAPTURE') {
      const payload = data.payload as Record<string, unknown> | undefined
      const url = typeof payload?.url === 'string' ? payload.url : ''
      const viewport = payload?.viewport
      if (!requestID || !validCaptureURL(url) || !validViewport(viewport)) {
        postCaptureError(requestID, new Error('采集 URL 或视口无效'))
        return
      }
      try {
        const response = await chrome.runtime.sendMessage({
          type: 'PM_DESIGN_REVIEW_START_CAPTURE',
          request_id: requestID,
          original_url: url,
          viewport: { ...viewport, pageZoom: 1 },
          dom_normalization: payload?.dom_normalization === true,
        }) as { ok: boolean; error?: string }
        if (!response.ok) throw new Error(response.error ?? '无法启动标签页采集')
        window.postMessage({ source: EXTENSION_SOURCE, type: 'PM_DESIGN_REVIEW_CAPTURE_STARTED', request_id: requestID }, window.origin)
      } catch (error) {
        postCaptureError(requestID, error)
      }
      return
    }
    if (data.type !== 'PM_DESIGN_REVIEW_CAPTURE_REQUEST') return
    const payload = data.payload as Record<string, unknown> | undefined
    const url = typeof payload?.url === 'string' ? payload.url : ''
    const viewport = payload?.viewport
    const frameSelector = payload?.frame_selector
    if (!requestID || !validCaptureURL(url) || !validViewport(viewport) || frameSelector !== IMPLEMENTATION_FRAME_SELECTOR) {
      postCaptureError(requestID, new Error('采集 URL、iframe 或视口无效'))
      return
    }
    const frame = document.querySelector<HTMLIFrameElement>(IMPLEMENTATION_FRAME_SELECTOR)
    if (!frame?.contentWindow || !validCaptureURL(frame.src) || new URL(frame.src).href !== new URL(url).href) {
      postCaptureError(requestID, new Error('找不到与实现地址匹配的嵌入页面'), true)
      return
    }
    window.postMessage({ source: EXTENSION_SOURCE, type: 'PM_DESIGN_REVIEW_CAPTURE_STARTED', request_id: requestID }, window.origin)
    try {
      await beginDirectCapture(
        requestID,
        url,
        { ...(viewport as { width: number; height: number; devicePixelRatio: number }), pageZoom: 1 },
        payload?.dom_normalization === true,
        frame,
      )
    } catch (error) {
      releaseDirectCapture(requestID)
      postCaptureError(requestID, error, true)
    }
  })
}
