import { execFile } from 'node:child_process'
import { chmod, mkdir, readFile, writeFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { promisify } from 'node:util'
import type { CollectorConfig, CollectorCredentials } from './types'

const execFileAsync = promisify(execFile)
const KEYCHAIN_SERVICE = 'com.one2all.pm.design-review-mobile-collector'
export const CONFIG_PATH = join(homedir(), 'Library', 'Application Support', 'PM Design Review Mobile Collector', 'config.json')

export function validateServerURL(input: string) {
  const url = new URL(input)
  const local = ['localhost', '127.0.0.1', '::1'].includes(url.hostname)
  if (url.protocol !== 'https:' && !(local && url.protocol === 'http:')) {
    throw new Error('移动采集器只连接 HTTPS 服务；本机开发地址可使用 HTTP')
  }
  return url.origin
}

async function writeToken(collectorID: string, token: string) {
  if (process.platform !== 'darwin') throw new Error('PM 移动采集器仅支持 macOS Keychain')
  await execFileAsync('/usr/bin/security', ['add-generic-password', '-U', '-a', collectorID, '-s', KEYCHAIN_SERVICE, '-w', token], { encoding: 'utf8' })
}

async function readToken(collectorID: string) {
  if (process.platform !== 'darwin') throw new Error('PM 移动采集器仅支持 macOS Keychain')
  const { stdout } = await execFileAsync('/usr/bin/security', ['find-generic-password', '-a', collectorID, '-s', KEYCHAIN_SERVICE, '-w'], { encoding: 'utf8' })
  const token = stdout.trim()
  if (!token) throw new Error('Keychain 中没有有效的移动采集器凭证，请重新配对')
  return token
}

export async function saveConfig(credentials: CollectorCredentials) {
  const config: CollectorConfig = { server_url: credentials.server_url, collector_id: credentials.collector_id }
  await writeToken(config.collector_id, credentials.token)
  await mkdir(dirname(CONFIG_PATH), { recursive: true })
  await writeFile(CONFIG_PATH, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 })
  await chmod(CONFIG_PATH, 0o600)
}

export async function loadConfig(): Promise<CollectorCredentials> {
  const config = JSON.parse(await readFile(CONFIG_PATH, 'utf8')) as CollectorConfig
  return { ...config, token: await readToken(config.collector_id) }
}
