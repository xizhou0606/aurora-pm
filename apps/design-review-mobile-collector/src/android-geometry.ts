import { execFile } from 'node:child_process'
import { homedir } from 'node:os'
import { delimiter, dirname, join } from 'node:path'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const BASELINE_ANDROID_DENSITY = 160
const DIMENSION_TOLERANCE = 2
type ScreenGeometry = { width: number; height: number; scale: number }
type PixelDimensions = { width: number; height: number }

function dimensionsNear(left: PixelDimensions, right: PixelDimensions) {
  return Math.abs(left.width - right.width) <= DIMENSION_TOLERANCE && Math.abs(left.height - right.height) <= DIMENSION_TOLERANCE
}

export function pngDimensions(png: Buffer): PixelDimensions {
  if (png.byteLength < 24 || png.subarray(0, 8).toString('hex') !== '89504e470d0a1a0a') throw new Error('移动截图不是有效的 PNG')
  const width = png.readUInt32BE(16)
  const height = png.readUInt32BE(20)
  if (width <= 0 || height <= 0) throw new Error('移动截图尺寸无效')
  return { width, height }
}

export function parseAndroidDensity(output: string) {
  const value = output.match(/Override density:\s*(\d+)/i)?.[1] ?? output.match(/Physical density:\s*(\d+)/i)?.[1] ?? output.match(/\b(\d{2,4})\b/)?.[1]
  const density = Number(value)
  return Number.isFinite(density) && density >= 72 && density <= 1280 ? density : null
}

export function managedAndroidSDKRoot(home = homedir()) {
  return join(home, 'Library', 'Application Support', 'PM Design Review Mobile Collector', 'tools', 'android')
}

export function configureManagedAndroidEnvironment(environment: NodeJS.ProcessEnv = process.env, home = homedir(), nodeExecutable = process.execPath) {
  const sdkRoot = managedAndroidSDKRoot(home)
  environment.ANDROID_HOME = sdkRoot
  environment.ANDROID_SDK_ROOT = sdkRoot
  environment.PATH = [dirname(nodeExecutable), join(sdkRoot, 'platform-tools'), '/usr/bin', '/bin', '/usr/sbin', '/sbin'].join(delimiter)
  return sdkRoot
}

export async function readAndroidDensity(deviceID: string) {
  try {
    const { stdout } = await execFileAsync(join(managedAndroidSDKRoot(), 'platform-tools', 'adb'), ['-s', deviceID, 'shell', 'wm', 'density'], { encoding: 'utf8', timeout: 10_000, maxBuffer: 64 * 1024 })
    return parseAndroidDensity(stdout)
  } catch { return null }
}

export function normalizeAndroidScreenGeometry(reported: ScreenGeometry, pixels: PixelDimensions, densityDPI: number | null) {
  const physical = { width: reported.width * reported.scale, height: reported.height * reported.scale }
  if (reported.scale > 1 && dimensionsNear(physical, pixels)) return { screen: reported, coordinateScale: 1, warning: null }
  const densityScale = densityDPI ? densityDPI / BASELINE_ANDROID_DENSITY : null
  if (densityScale && densityScale > 1 && densityScale <= 8) {
    const screen = { width: Math.round(pixels.width / densityScale), height: Math.round(pixels.height / densityScale), scale: densityScale }
    return { screen, coordinateScale: dimensionsNear(reported, screen) ? 1 : densityScale, warning: null }
  }
  return {
    screen: reported,
    coordinateScale: 1,
    warning: { code: 'screenshot_scale_mismatch', message: '未取得有效的 Android 屏幕密度，截图或原生元素可能仍使用物理像素坐标' },
  }
}
