import { hostname } from 'node:os'
import { configureManagedAndroidEnvironment } from './android-geometry'
import { pairCollector, runCollector } from './client'
import { loadConfig, saveConfig, validateServerURL } from './config'
import { MobilewrightAdapter } from './mobilewright-adapter'
import { ANDROID_PLATFORM_TOOLS_VERSION, COLLECTOR_VERSION, MOBILECLI_VERSION, MOBILEWRIGHT_VERSION, NODE_VERSION } from './runtime-versions'

process.env.MOBILEWRIGHT_DISABLE_TELEMETRY = '1'
configureManagedAndroidEnvironment()

async function main() {
  const [command = 'start', ...args] = process.argv.slice(2)
  if (command === 'version') {
    console.log(JSON.stringify({ collectorVersion: COLLECTOR_VERSION, mobilewrightVersion: MOBILEWRIGHT_VERSION, mobilecliVersion: MOBILECLI_VERSION, requiredNodeVersion: NODE_VERSION, androidPlatformToolsVersion: ANDROID_PLATFORM_TOOLS_VERSION, nodeVersion: process.versions.node, architecture: process.arch, platform: process.platform }))
    return
  }
  const adapter = new MobilewrightAdapter()
  if (command === 'pair') {
    const [serverURL, code, name = hostname()] = args
    if (!serverURL || !code) throw new Error('用法：pm-design-review-mobile-collector pair <PM 地址> <配对码> [采集器名称]')
    const credentials = await pairCollector(validateServerURL(serverURL), code, name, adapter)
    await saveConfig(credentials)
    console.log(`已配对 PM 移动采集器 ${credentials.collector_id}`)
    return
  }
  if (command === 'doctor') {
    const platform = args[0]
    if (platform && platform !== 'ios' && platform !== 'android') throw new Error('Doctor 平台只能是 ios 或 android')
    const devices = (await adapter.listDevices()).filter((device) => !platform || device.platform === platform)
    console.log(devices.length ? `Mobilewright 已就绪，检测到 ${devices.length} 台在线设备` : 'Mobilewright 可运行，但未检测到在线设备')
    for (const device of devices) console.log(`- ${device.platform} · ${device.name} · ${device.os_version ?? '未知系统版本'}`)
    process.exitCode = devices.length ? 0 : 1
    return
  }
  if (command !== 'start') throw new Error(`未知命令：${command}`)
  const controller = new AbortController()
  process.on('SIGTERM', () => controller.abort())
  process.on('SIGINT', () => controller.abort())
  console.log('PM 设计走查移动采集器正在运行')
  await runCollector(await loadConfig(), adapter, controller.signal)
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : '移动采集器运行失败')
  process.exitCode = 1
})
