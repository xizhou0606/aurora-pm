import type { CaptureEvidence, CaptureTask, NormalizationPlan } from './types'
import { parseNormalizationPlan } from './normalization-policy'

const TASK_STORAGE_KEY = 'pmDesignReviewCaptureTasksV1'

type PreparedEvidence = Omit<CaptureEvidence, 'viewport' | 'screenshot' | 'original_screenshot'>
type PendingCapture = {
  task: CaptureTask
  evidence: PreparedEvidence
  originalScreenshot: string
  pageZoom: number
}

const pendingCaptures = new Map<string, PendingCapture>()

async function readTasks() {
  const stored = await chrome.storage.session.get(TASK_STORAGE_KEY)
  return (stored[TASK_STORAGE_KEY] as Record<string, CaptureTask> | undefined) ?? {}
}

async function writeTasks(tasks: Record<string, CaptureTask>) {
  await chrome.storage.session.set({ [TASK_STORAGE_KEY]: tasks })
}

function taskForRequest(tasks: Record<string, CaptureTask>, requestID: string) {
  return Object.values(tasks).find((task) => task.request_id === requestID)
}

async function focusWorkbench(task: CaptureTask) {
  const workbench = await chrome.tabs.get(task.workbench_tab_id)
  await chrome.tabs.update(task.workbench_tab_id, { active: true })
  if (workbench.windowId !== undefined) await chrome.windows.update(workbench.windowId, { focused: true })
}

async function cleanupCapture(task: CaptureTask, removeTask = true) {
  pendingCaptures.delete(task.request_id)
  try {
    await chrome.tabs.sendMessage(task.target_tab_id, { type: 'PM_DESIGN_REVIEW_RELEASE_CAPTURE', request_id: task.request_id })
  } catch {}
  const debuggee = { tabId: task.target_tab_id }
  try { await chrome.debugger.sendCommand(debuggee, 'Emulation.clearDeviceMetricsOverride') } catch {}
  try { await chrome.debugger.detach(debuggee) } catch {}
  if (!removeTask) return
  const tasks = await readTasks()
  if (tasks[String(task.target_tab_id)]?.request_id === task.request_id) {
    delete tasks[String(task.target_tab_id)]
    await writeTasks(tasks)
  }
}

async function captureScreenshot(tabID: number) {
  const screenshot = await chrome.debugger.sendCommand({ tabId: tabID }, 'Page.captureScreenshot', {
    format: 'png', fromSurface: true, captureBeyondViewport: false,
  }) as { data?: string }
  if (!screenshot.data) throw new Error('浏览器未生成页面截图')
  return `data:image/png;base64,${screenshot.data}`
}

async function activeTab() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
  return tab
}

async function captureVisibleTab(sender: chrome.runtime.MessageSender) {
  const tabID = sender.tab?.id
  const windowID = sender.tab?.windowId
  if (tabID === undefined || windowID === undefined) {
    throw new Error('无法识别设计走查工作台标签页')
  }
  const [screenshot, pageZoom] = await Promise.all([
    Promise.race([
      chrome.tabs.captureVisibleTab(windowID, { format: 'png' }),
      new Promise<never>((_, reject) => setTimeout(() => reject(new Error('浏览器截图超时，请重试或使用标签页采集')), 10_000)),
    ]),
    chrome.tabs.getZoom(tabID),
  ])
  return { ok: true, screenshot, page_zoom: pageZoom }
}

async function startCapture(message: {
  request_id: string
  original_url: string
  viewport: CaptureTask['viewport']
  dom_normalization: boolean
}, sender: chrome.runtime.MessageSender) {
  const workbenchTabID = sender.tab?.id
  if (workbenchTabID === undefined) throw new Error('无法识别设计走查工作台')
  const tasks = await readTasks()
  const previous = Object.values(tasks).find((task) => task.workbench_tab_id === workbenchTabID)
  if (previous) {
    await cleanupCapture(previous)
    try { await chrome.tabs.remove(previous.target_tab_id) } catch {}
    delete tasks[String(previous.target_tab_id)]
  }
  const target = await chrome.tabs.create({ url: message.original_url, active: true })
  if (target.id === undefined) throw new Error('浏览器未能打开目标页面')
  tasks[String(target.id)] = {
    request_id: message.request_id,
    workbench_tab_id: workbenchTabID,
    target_tab_id: target.id,
    original_url: message.original_url,
    viewport: message.viewport,
    dom_normalization: message.dom_normalization,
    state: 'ready',
    created_at: new Date().toISOString(),
  }
  await writeTasks(tasks)
  return { ok: true, target_tab_id: target.id }
}

async function status() {
  const tab = await activeTab()
  if (tab?.id === undefined) return { ok: true, task: null }
  const tasks = await readTasks()
  const task = tasks[String(tab.id)] ?? Object.values(tasks).find((item) => item.workbench_tab_id === tab.id) ?? null
  return { ok: true, task: task ? { ...task, on_target_tab: task.target_tab_id === tab.id } : null }
}

async function focusTarget() {
  const tab = await activeTab()
  const tasks = await readTasks()
  const task = Object.values(tasks).find((item) => item.workbench_tab_id === tab?.id)
  if (!task) throw new Error('当前工作台没有待完成的采集任务')
  await chrome.tabs.update(task.target_tab_id, { active: true })
  return { ok: true }
}

async function focusTaskWorkbench() {
  const tab = await activeTab()
  const tasks = await readTasks()
  const task = tasks[String(tab?.id)] ?? Object.values(tasks).find((item) => item.workbench_tab_id === tab?.id)
  if (!task) throw new Error('当前页面没有待处理的采集任务')
  await focusWorkbench(task)
  return { ok: true }
}

async function completeCapture() {
  const tab = await activeTab()
  if (tab?.id === undefined) throw new Error('无法识别当前标签页')
  const tasks = await readTasks()
  const task = tasks[String(tab.id)]
  if (!task) throw new Error('当前页面不是 PM 设计走查等待采集的目标页')
  if (task.state === 'awaiting_normalization') throw new Error('正在等待工作台生成归一化方案')
  const pageZoom = await chrome.tabs.getZoom(task.target_tab_id)
  if (Math.abs(pageZoom - 1) > 0.001) throw new Error(`页面缩放为 ${Math.round(pageZoom * 100)}%，请恢复到 100%`)

  const debuggee = { tabId: task.target_tab_id }
  try {
    await chrome.debugger.attach(debuggee, '1.3')
    await chrome.debugger.sendCommand(debuggee, 'Emulation.setDeviceMetricsOverride', {
      width: task.viewport.width,
      height: task.viewport.height,
      deviceScaleFactor: task.viewport.devicePixelRatio,
      mobile: false,
      screenWidth: task.viewport.width,
      screenHeight: task.viewport.height,
    })
    const response = await chrome.tabs.sendMessage(task.target_tab_id, {
      type: 'PM_DESIGN_REVIEW_PREPARE_CAPTURE', request_id: task.request_id,
      original_url: task.original_url, dom_normalization: task.dom_normalization,
    }) as { ok: boolean; evidence?: PreparedEvidence; error?: string }
    if (!response.ok || !response.evidence) throw new Error(response.error ?? '无法读取目标页面')
    const screenshot = await captureScreenshot(task.target_tab_id)
    if (task.dom_normalization) {
      const waitingTask = { ...task, state: 'awaiting_normalization' as const }
      tasks[String(task.target_tab_id)] = waitingTask
      await writeTasks(tasks)
      pendingCaptures.set(task.request_id, { task: waitingTask, evidence: response.evidence, originalScreenshot: screenshot, pageZoom })
      const delivery = await chrome.tabs.sendMessage(task.workbench_tab_id, {
        type: 'PM_DESIGN_REVIEW_DELIVER_NORMALIZATION_INVENTORY',
        request_id: task.request_id,
        inventory: response.evidence.inventory,
        original_screenshot: screenshot,
      }) as { ok?: boolean }
      if (!delivery?.ok) throw new Error('工作台未响应归一化清单')
      return { ok: true, awaiting_normalization: true }
    }
    const evidence: CaptureEvidence = {
      ...response.evidence,
      viewport: { ...task.viewport, pageZoom },
      screenshot,
    }
    const delivery = await chrome.tabs.sendMessage(task.workbench_tab_id, { type: 'PM_DESIGN_REVIEW_DELIVER_CAPTURE', evidence }) as { ok?: boolean }
    if (!delivery?.ok) throw new Error('工作台未响应采集结果')
    await cleanupCapture(task)
    await focusWorkbench(task)
    return { ok: true }
  } catch (error) {
    await cleanupCapture(task)
    throw error
  }
}

async function applyNormalization(message: { request_id: string; plan: NormalizationPlan }, sender: chrome.runtime.MessageSender) {
  const tasks = await readTasks()
  const task = taskForRequest(tasks, message.request_id)
  if (!task || sender.tab?.id !== task.workbench_tab_id || task.state !== 'awaiting_normalization') throw new Error('归一化任务已失效')
  const pending = pendingCaptures.get(message.request_id)
  if (!pending) {
    await cleanupCapture(task)
    throw new Error('扩展后台已重启，请重新采集页面')
  }
  const plan = parseNormalizationPlan(message.plan)
  if (!plan) {
    await cleanupCapture(task)
    throw new Error('归一化方案未通过扩展安全校验')
  }
  try {
    const normalized = await chrome.tabs.sendMessage(task.target_tab_id, {
      type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION', request_id: task.request_id, plan,
    }) as { ok: boolean; evidence?: Pick<CaptureEvidence, 'elements' | 'structure_truncated'> & { report: NonNullable<CaptureEvidence['dom_normalization']['report']> }; error?: string }
    if (!normalized.ok || !normalized.evidence) throw new Error(normalized.error ?? '目标页面拒绝归一化方案')
    const screenshot = await captureScreenshot(task.target_tab_id)
    const evidence: CaptureEvidence = {
      ...pending.evidence,
      ...normalized.evidence,
      viewport: { ...task.viewport, pageZoom: pending.pageZoom },
      screenshot,
      original_screenshot: pending.originalScreenshot,
      dom_normalization: { requested: true, report: normalized.evidence.report },
    }
    const delivery = await chrome.tabs.sendMessage(task.workbench_tab_id, { type: 'PM_DESIGN_REVIEW_DELIVER_CAPTURE', evidence }) as { ok?: boolean }
    if (!delivery?.ok) throw new Error('工作台未响应归一化采集结果')
    await cleanupCapture(task)
    await focusWorkbench(task)
    return { ok: true }
  } catch (error) {
    await cleanupCapture(task)
    throw error
  }
}

async function abortNormalization(message: { request_id: string }, sender: chrome.runtime.MessageSender) {
  const tasks = await readTasks()
  const task = taskForRequest(tasks, message.request_id)
  if (!task || sender.tab?.id !== task.workbench_tab_id) throw new Error('归一化任务已失效')
  await cleanupCapture(task)
  await focusWorkbench(task)
  return { ok: true }
}

type BackgroundRequest =
  | { type: 'PM_DESIGN_REVIEW_CAPTURE_VISIBLE_TAB' }
  | { type: 'PM_DESIGN_REVIEW_START_CAPTURE'; request_id: string; original_url: string; viewport: CaptureTask['viewport']; dom_normalization: boolean }
  | { type: 'PM_DESIGN_REVIEW_GET_STATUS' }
  | { type: 'PM_DESIGN_REVIEW_FOCUS_TARGET' }
  | { type: 'PM_DESIGN_REVIEW_FOCUS_WORKBENCH' }
  | { type: 'PM_DESIGN_REVIEW_COMPLETE_CAPTURE' }
  | { type: 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION'; request_id: string; plan: NormalizationPlan }
  | { type: 'PM_DESIGN_REVIEW_ABORT_NORMALIZATION'; request_id: string }

async function handle(message: BackgroundRequest, sender: chrome.runtime.MessageSender) {
  if (message.type === 'PM_DESIGN_REVIEW_CAPTURE_VISIBLE_TAB') return captureVisibleTab(sender)
  if (message.type === 'PM_DESIGN_REVIEW_START_CAPTURE') return startCapture(message, sender)
  if (message.type === 'PM_DESIGN_REVIEW_GET_STATUS') return status()
  if (message.type === 'PM_DESIGN_REVIEW_FOCUS_TARGET') return focusTarget()
  if (message.type === 'PM_DESIGN_REVIEW_FOCUS_WORKBENCH') return focusTaskWorkbench()
  if (message.type === 'PM_DESIGN_REVIEW_COMPLETE_CAPTURE') return completeCapture()
  if (message.type === 'PM_DESIGN_REVIEW_APPLY_NORMALIZATION') return applyNormalization(message, sender)
  if (message.type === 'PM_DESIGN_REVIEW_ABORT_NORMALIZATION') return abortNormalization(message, sender)
  throw new Error('不支持的扩展操作')
}

chrome.runtime.onMessage.addListener((message: BackgroundRequest, sender, respond) => {
  handle(message, sender).then(respond).catch((error: unknown) => respond({ ok: false, error: error instanceof Error ? error.message : '扩展操作失败' }))
  return true
})

chrome.tabs.onRemoved.addListener((tabID) => {
  void (async () => {
    const tasks = await readTasks()
    const target = tasks[String(tabID)]
    if (target) {
      await cleanupCapture(target)
      try {
        await chrome.tabs.sendMessage(target.workbench_tab_id, {
          type: 'PM_DESIGN_REVIEW_DELIVER_ERROR', request_id: target.request_id, error: '目标标签页已关闭，采集任务已取消',
        })
      } catch {}
    }
    for (const task of Object.values(tasks)) {
      if (task.workbench_tab_id === tabID) await cleanupCapture(task)
    }
  })()
})
