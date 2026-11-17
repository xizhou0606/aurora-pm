import { arch, hostname } from 'node:os'
import { z } from 'zod'
import { COLLECTOR_VERSION, MOBILEWRIGHT_VERSION } from './runtime-versions'
import type { CapturedMobileEvidence, MobileAutomationAdapter } from './mobilewright-adapter'
import type { CollectorCredentials, MobileCaptureCommand } from './types'

const commandSchema = z.object({ id: z.string().uuid(), platform: z.enum(['ios', 'android']), device_id: z.string().min(1).max(300) }).strict()
const basePath = '/api/v1/platform/design-review-mobile-captures'

function endpoint(config: Pick<CollectorCredentials, 'server_url'>, path: string) {
  return new URL(`${basePath}${path}`, config.server_url).toString()
}

async function collectorRequest(config: CollectorCredentials, path: string, init: RequestInit = {}) {
  const response = await fetch(endpoint(config, path), {
    ...init,
    headers: { authorization: `Collector ${config.token}`, ...init.headers },
  })
  if (response.status === 401) throw new Error('采集器配对已失效，请在 PM 设计管理中重新配对')
  return response
}

async function responseData(response: Response) {
  const body = await response.json().catch(() => ({})) as { data?: unknown; error?: string }
  if (!response.ok) throw new Error(body.error ?? `PM API 请求失败（HTTP ${response.status}）`)
  return body.data
}

export async function pairCollector(serverURL: string, code: string, name: string, adapter: MobileAutomationAdapter) {
  const devices = await adapter.listDevices()
  const body = {
    code, name, version: COLLECTOR_VERSION, mobilewright_version: MOBILEWRIGHT_VERSION,
    hostname: hostname(), architecture: arch() === 'arm64' ? 'arm64' : 'x64',
    inventory: { devices, doctor: { ready: devices.length > 0, summary: devices.length ? `检测到 ${devices.length} 台在线设备` : '未检测到在线设备' } },
  }
  const response = await fetch(endpoint({ server_url: serverURL }, '/pair'), {
    method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body),
  })
  const data = await responseData(response) as { collector?: { id?: string }; token?: string }
  if (!data.collector?.id || !data.token) throw new Error('PM API 未返回完整配对凭证')
  return { server_url: new URL(serverURL).origin, collector_id: data.collector.id, token: data.token }
}

export async function heartbeat(config: CollectorCredentials, adapter: MobileAutomationAdapter) {
  const devices = await adapter.listDevices()
  const response = await collectorRequest(config, '/collector/heartbeat', {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      version: COLLECTOR_VERSION, mobilewright_version: MOBILEWRIGHT_VERSION, hostname: hostname(),
      architecture: arch() === 'arm64' ? 'arm64' : 'x64', devices,
      doctor: { ready: devices.length > 0, summary: devices.length ? `检测到 ${devices.length} 台在线设备` : '未检测到在线设备' },
    }),
  })
  await responseData(response)
}

export async function claim(config: CollectorCredentials) {
  const response = await collectorRequest(config, '/collector/claim', { method: 'POST' })
  if (response.status === 204) return null
  return commandSchema.parse(await responseData(response))
}

async function complete(config: CollectorCredentials, command: MobileCaptureCommand, evidence: CapturedMobileEvidence) {
  const form = new FormData()
  form.set('metadata', JSON.stringify(evidence.result))
  form.set('screenshot', new Blob([new Uint8Array(evidence.screenshot)], { type: 'image/png' }), 'capture.png')
  let lastError: Error | null = null
  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      const response = await collectorRequest(config, `/collector/drafts/${command.id}/complete`, { method: 'POST', body: form })
      await responseData(response)
      return
    } catch (error) {
      lastError = error instanceof Error ? error : new Error('上传移动证据失败')
      if (attempt === 2 || /配对已失效/.test(lastError.message)) throw lastError
      await new Promise((resolve) => setTimeout(resolve, 250 * (attempt + 1)))
    }
  }
  throw lastError ?? new Error('上传移动证据失败')
}

async function fail(config: CollectorCredentials, command: MobileCaptureCommand, error: unknown) {
  await collectorRequest(config, `/collector/drafts/${command.id}/fail`, {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ error: error instanceof Error ? error.message.slice(0, 1000) : '移动采集失败' }),
  }).catch(() => undefined)
}

export async function runCollector(config: CollectorCredentials, adapter: MobileAutomationAdapter, signal?: AbortSignal) {
  let lastHeartbeat = 0
  while (!signal?.aborted) {
    if (Date.now() - lastHeartbeat >= 10_000) {
      await heartbeat(config, adapter)
      lastHeartbeat = Date.now()
    }
    const command = await claim(config)
    if (command) {
      try { await complete(config, command, await adapter.capture(command)) } catch (error) { await fail(config, command, error) }
      continue
    }
    await new Promise((resolve) => setTimeout(resolve, 1700 + Math.floor(Math.random() * 600)))
  }
}
