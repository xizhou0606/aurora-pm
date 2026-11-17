export type MobilePlatform = 'ios' | 'android'

export type MobileDevice = {
  id: string
  name: string
  platform: MobilePlatform
  type: 'real' | 'simulator' | 'emulator'
  model: string | null
  os_version: string | null
}

export type MobileCaptureCommand = {
  id: string
  platform: MobilePlatform
  device_id: string
}

export type NativeVisualElement = {
  id: string
  parent_id: string | null
  native_type: string
  role: string | null
  label: string | null
  text: string | null
  value: string | null
  identifier: string | null
  resource_id: string | null
  bounds: { x: number; y: number; width: number; height: number }
  state: { visible: boolean; enabled: boolean; selected: boolean | null; focused: boolean | null; checked: boolean | null }
  clipped: boolean
  structural_fingerprint: string
}

export type MobileCaptureResult = {
  captured_at: string
  target: {
    kind: 'mobile'
    platform: MobilePlatform
    app_id: string
    app_name: string | null
    app_version: string | null
    orientation: 'portrait' | 'landscape'
    device: Omit<MobileDevice, 'id' | 'platform'>
    screen: { width: number; height: number; scale: number }
    collector_version: string
    mobilewright_version: string
  }
  elements: NativeVisualElement[]
  structure_truncated: boolean
  warnings: Array<{ code: string; message: string }>
  screenshot_sha256_before: string
  screenshot_sha256_after: string
}

export type CollectorConfig = { server_url: string; collector_id: string }
export type CollectorCredentials = CollectorConfig & { token: string }
