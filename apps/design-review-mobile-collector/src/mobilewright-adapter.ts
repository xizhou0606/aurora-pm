import { createHash } from 'node:crypto'
import type { DeviceInfo, ViewNode } from '@mobilewright/protocol'
import { android, ios } from 'mobilewright'
import { normalizeAndroidScreenGeometry, pngDimensions, readAndroidDensity } from './android-geometry'
import { flattenViewTree } from './native-elements'
import { COLLECTOR_VERSION, MOBILEWRIGHT_VERSION } from './runtime-versions'
import type { MobileCaptureCommand, MobileCaptureResult, MobileDevice, MobilePlatform } from './types'

export type CapturedMobileEvidence = { screenshot: Buffer; result: MobileCaptureResult }
export interface MobileAutomationAdapter { listDevices(): Promise<MobileDevice[]>; capture(command: MobileCaptureCommand): Promise<CapturedMobileEvidence> }

type MobileDeviceSession = {
  screen: { screenshot(): Promise<Buffer>; viewTree(): Promise<ViewNode[]> }
  screenSize(): Promise<{ width: number; height: number; scale: number }>
  getOrientation(): Promise<'portrait' | 'landscape'>
  getForegroundApp(): Promise<{ bundleId: string; name?: string; version?: string }>
  close(): Promise<void>
}

function deviceOf(info: DeviceInfo): MobileDevice {
  return {
    id: info.id, name: info.name, platform: info.platform,
    type: info.type === 'real' ? 'real' : info.platform === 'ios' ? 'simulator' : 'emulator',
    model: info.model ?? null, os_version: info.osVersion ?? null,
  }
}

const provider = {
  async listDevices() {
    const [iosDevices, androidDevices] = await Promise.all([ios.devices().catch(() => []), android.devices().catch(() => [])])
    return [...iosDevices, ...androidDevices].filter((device) => device.state === 'online').map(deviceOf)
  },
  async launch(platform: MobilePlatform, deviceID: string) {
    return (platform === 'ios' ? ios : android).launch({ deviceId: deviceID, autoAppLaunch: false }) as unknown as MobileDeviceSession
  },
}

const sha256 = (buffer: Buffer) => createHash('sha256').update(buffer).digest('hex')
const optional = (after?: string, before?: string) => after?.trim() || before?.trim() || null

export class MobilewrightAdapter implements MobileAutomationAdapter {
  async listDevices() { return provider.listDevices() }

  async capture(command: MobileCaptureCommand): Promise<CapturedMobileEvidence> {
    const selected = (await this.listDevices()).find((device) => device.id === command.device_id && device.platform === command.platform)
    if (!selected) throw new Error('所选移动设备已离线，请刷新设备后重试')
    const device = await provider.launch(command.platform, command.device_id)
    try {
      const beforeApp = await device.getForegroundApp()
      const appID = beforeApp.bundleId?.trim()
      if (!appID) throw new Error('无法识别前台 App，请打开目标 App 并停留在待走查页面')
      const [reportedScreen, orientation, density] = await Promise.all([
        device.screenSize(), device.getOrientation(), command.platform === 'android' ? readAndroidDensity(command.device_id) : Promise.resolve(null),
      ])
      const before = await device.screen.screenshot()
      const tree = await device.screen.viewTree()
      await new Promise((resolve) => setTimeout(resolve, 300))
      const after = await device.screen.screenshot()
      const afterApp = await device.getForegroundApp()
      if (afterApp.bundleId?.trim() !== appID) throw new Error('采集期间前台 App 已切换，请恢复目标页面后重试')
      const geometry = command.platform === 'android'
        ? normalizeAndroidScreenGeometry(reportedScreen, pngDimensions(after), density)
        : { screen: reportedScreen, coordinateScale: 1, warning: null }
      const flattened = flattenViewTree(tree, { platform: command.platform, appID, width: geometry.screen.width, height: geometry.screen.height, coordinateScale: geometry.coordinateScale })
      const beforeHash = sha256(before); const afterHash = sha256(after)
      const warnings: MobileCaptureResult['warnings'] = []
      if (geometry.warning) warnings.push(geometry.warning)
      if (beforeHash !== afterHash) warnings.push({ code: 'screen_unstable', message: '两次连续截图不一致，目标画面可能仍在变化' })
      if (!flattened.elements.length) warnings.push({ code: 'accessibility_tree_empty', message: '未取得可见 accessibility tree，结构证据不完整' })
      if (flattened.truncated) warnings.push({ code: 'structure_truncated', message: `可见原生元素超过 1,500 个，结构证据已截断（检测到 ${flattened.visibleCount} 个）` })
      return {
        screenshot: after,
        result: {
          captured_at: new Date().toISOString(),
          target: {
            kind: 'mobile', platform: command.platform, app_id: appID,
            app_name: optional(afterApp.name, beforeApp.name), app_version: optional(afterApp.version, beforeApp.version), orientation,
            device: { name: selected.name, type: selected.type, model: selected.model, os_version: selected.os_version },
            screen: geometry.screen, collector_version: COLLECTOR_VERSION, mobilewright_version: MOBILEWRIGHT_VERSION,
          },
          elements: flattened.elements, structure_truncated: flattened.truncated || !flattened.elements.length,
          warnings, screenshot_sha256_before: beforeHash, screenshot_sha256_after: afterHash,
        },
      }
    } finally { await device.close().catch(() => undefined) }
  }
}
