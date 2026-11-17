import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'
import { chmod, cp, mkdir, mkdtemp, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const NODE_VERSION = '24.14.0'
const NODE_CHECKSUMS = {
  arm64: 'a1a54f46a750d2523d628d924aab61758a51c9dad3e0238beb14141be9615dd3',
  x64: 'f2879eb810e25993a0578e5d878930266fd2eafcffe9f2839b3d8db354d4879e',
}
const PLATFORM_TOOLS_VERSION = '37.0.1'
const PLATFORM_TOOLS_CHECKSUM = 'ee39ad5967e95c2a07f04dbcbde96b1a0c916ba376096db5d2f498b7727a5d1d'
const ANDROID_AGENT_VERSION = '1.2.4'
const ANDROID_AGENT_ASSET_ID = '475408400'
const ANDROID_AGENT_CHECKSUM = '63b1111fbd3b986c7452bc7c28150b1e9c0d611b2ecd7f6917a0f50a84d0836b'
const collectorRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = resolve(collectorRoot, '../..')
const outputDir = resolve(collectorRoot, '../web/public/downloads')
const cacheDir = resolve(repositoryRoot, '.cache/design-review-mobile-collector')
const version = JSON.parse(await readFile(join(collectorRoot, 'package.json'), 'utf8')).version

function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const child = spawn(command, args, { stdio: 'inherit', ...options })
    child.on('error', reject)
    child.on('exit', (code) => code === 0 ? resolveRun() : reject(new Error(`${command} exited with ${code}`)))
  })
}

const sha256 = (value) => createHash('sha256').update(value).digest('hex')

async function cachedDownload(url, filename, expected) {
  await mkdir(cacheDir, { recursive: true })
  const target = join(cacheDir, filename)
  try {
    if (sha256(await readFile(target)) === expected) return target
  } catch {}
  const response = await fetch(url)
  if (!response.ok) throw new Error(`download failed: ${url} (${response.status})`)
  const bytes = Buffer.from(await response.arrayBuffer())
  const actual = sha256(bytes)
  if (actual !== expected) throw new Error(`${filename} checksum mismatch: ${actual}`)
  await writeFile(target, bytes)
  return target
}

async function downloadAndroidAgent() {
  await mkdir(cacheDir, { recursive: true })
  const filename = `devicekit-android-${ANDROID_AGENT_VERSION}.apk`
  const target = join(cacheDir, filename)
  try {
    if (sha256(await readFile(target)) === ANDROID_AGENT_CHECKSUM) return target
  } catch {}
  const sources = [
    {
      url: `https://github.com/mobile-next/devicekit-android/releases/download/${ANDROID_AGENT_VERSION}/devicekit.apk`,
      headers: [],
    },
    {
      url: `https://api.github.com/repos/mobile-next/devicekit-android/releases/assets/${ANDROID_AGENT_ASSET_ID}`,
      headers: ['Accept: application/octet-stream', 'X-GitHub-Api-Version: 2022-11-28'],
    },
  ]
  const failures = []
  for (const source of sources) {
    await rm(target, { force: true })
    try {
      await run('curl', [
        '--fail', '--location', '--show-error', '--retry', '3', '--retry-delay', '2', '--retry-connrefused',
        '--connect-timeout', '15', '--user-agent', `pm-design-review-mobile-collector/${version}`,
        ...source.headers.flatMap((header) => ['--header', header]), source.url, '--output', target,
      ])
      if (sha256(await readFile(target)) === ANDROID_AGENT_CHECKSUM) return target
      failures.push(`${source.url}: checksum mismatch`)
    } catch (error) {
      failures.push(`${source.url}: ${error instanceof Error ? error.message : String(error)}`)
    }
  }
  await rm(target, { force: true })
  throw new Error(`unable to download verified DeviceKit Android ${ANDROID_AGENT_VERSION}: ${failures.join('; ')}`)
}

async function listFiles(root) {
  const output = []
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const path = join(root, entry.name)
    if (entry.isDirectory()) output.push(...await listFiles(path))
    else output.push(path)
  }
  return output
}

async function assertStandalone(app) {
  const files = await listFiles(app)
  const source = files.find((file) => /node_modules[/\\](?:\.pnpm[/\\][^/\\]+[/\\]node_modules[/\\])?@pm[/\\].+\.tsx?$/.test(file))
  if (source) throw new Error(`production package contains workspace TypeScript source: ${source}`)
  const entry = await readFile(join(app, 'dist/index.js'), 'utf8')
  if (/@ui-review|UI Lens|ui-lens/i.test(entry)) throw new Error('production package contains UI-review branding or dependency')
}

function machOArchitectures(data) {
  if (data.byteLength < 8) return []
  const architectures = new Set()
  const addCPUType = (cpuType) => {
    if (cpuType === 0x0100000c) architectures.add('arm64')
    else if (cpuType === 0x01000007) architectures.add('x86_64')
    else architectures.add(`unknown-0x${cpuType.toString(16)}`)
  }
  const magic = data.readUInt32BE(0)
  if (magic === 0xfeedface || magic === 0xfeedfacf) addCPUType(data.readUInt32BE(4))
  else if (magic === 0xcefaedfe || magic === 0xcffaedfe) addCPUType(data.readUInt32LE(4))
  else if (magic === 0xcafebabe || magic === 0xcafebabf) {
    const entrySize = magic === 0xcafebabf ? 32 : 20
    const count = data.readUInt32BE(4)
    for (let index = 0; index < count && 8 + index * entrySize + 4 <= data.byteLength; index += 1) addCPUType(data.readUInt32BE(8 + index * entrySize))
  } else if (magic === 0xbebafeca || magic === 0xbfbafeca) {
    const entrySize = magic === 0xbfbafeca ? 32 : 20
    const count = data.readUInt32LE(4)
    for (let index = 0; index < count && 8 + index * entrySize + 4 <= data.byteLength; index += 1) addCPUType(data.readUInt32LE(8 + index * entrySize))
  }
  return [...architectures]
}

async function prepareNativeTools(app, architecture) {
  const mobilecliDir = join(app, 'node_modules/mobilecli/bin')
  const keep = architecture === 'arm64' ? 'mobilecli-darwin-arm64' : 'mobilecli-darwin-amd64'
  await removeOtherMobileCLIArchitectures(app, keep)
  const keptBinary = join(mobilecliDir, keep)
  await chmod(keptBinary, 0o755)
  return keptBinary
}

async function removeOtherMobileCLIArchitectures(directory, keep) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const absolute = join(directory, entry.name)
    if (/^mobilecli-(?:darwin|linux|windows)-/.test(entry.name) && entry.name !== keep) {
      await rm(absolute, { force: true })
    } else if (entry.isDirectory()) {
      await removeOtherMobileCLIArchitectures(absolute, keep)
    }
  }
}

async function removeOtherSharpArchitecture(directory, architecture) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const absolute = join(directory, entry.name)
    const nativeSharp = /^sharp-(?:libvips-)?(?:darwin|linux|linuxmusl|win32|freebsd)-/.test(entry.name)
    if (nativeSharp && !entry.name.includes(`darwin-${architecture}`)) {
      await rm(absolute, { recursive: true, force: true })
    } else if (entry.isDirectory()) {
      await removeOtherSharpArchitecture(absolute, architecture)
    }
  }
}

const installScript = `#!/bin/zsh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
TARGET="$HOME/Library/Application Support/PM Design Review Mobile Collector"
PLIST="$HOME/Library/LaunchAgents/com.one2all.pm.design-review-mobile-collector.plist"

clear_quarantine() {
  /usr/bin/xattr -dr com.apple.quarantine "$1" 2>/dev/null || true
}

clear_quarantine "$ROOT"
if [[ "\${1:-}" == "--verify-quarantine" ]]; then exit 0; fi

pair_collector() {
local suggested_server="$1"
local server code
server="$(/usr/bin/osascript - "$suggested_server" <<'APPLESCRIPT'
on run argv
  return text returned of (display dialog "请输入当前浏览器中的 PM 页面地址" with title "连接 PM 设计管理" default answer (item 1 of argv) buttons {"取消", "继续"} default button "继续" cancel button "取消")
end run
APPLESCRIPT
)" || return 1
code="$(/usr/bin/osascript <<'APPLESCRIPT'
text returned of (display dialog "请输入设计管理页面生成的十分钟一次性配对码" with title "配对移动采集器" default answer "" buttons {"取消", "配对"} default button "配对" cancel button "取消")
APPLESCRIPT
)" || return 1
"$ROOT/runtime/bin/node" "$ROOT/app/dist/index.js" pair "$server" "$code"
}

if [[ -f "$TARGET/config.json" ]]; then
  CURRENT_SERVER="$(/usr/bin/plutil -extract server_url raw -o - "$TARGET/config.json" 2>/dev/null || true)"
  CHOICE="$(/usr/bin/osascript - "\${CURRENT_SERVER:-未知地址}" <<'APPLESCRIPT'
on run argv
  return button returned of (display dialog "此 Mac 已有 PM 移动采集器配对：" & return & return & (item 1 of argv) with title "升级移动采集器" buttons {"取消安装", "重新连接", "保留并升级"} default button "保留并升级" cancel button "取消安装")
end run
APPLESCRIPT
)" || exit 0
  if [[ "$CHOICE" == "重新连接" ]]; then pair_collector "\${CURRENT_SERVER:-https://}" || exit 0; fi
else
  pair_collector "https://" || exit 0
fi

mkdir -p "$TARGET" "$HOME/Library/LaunchAgents"
rm -rf "$TARGET/runtime" "$TARGET/app" "$TARGET/agents" "$TARGET/tools"
cp -R "$ROOT/runtime" "$TARGET/runtime"
cp -R "$ROOT/app" "$TARGET/app"
cp -R "$ROOT/agents" "$TARGET/agents"
cp -R "$ROOT/tools" "$TARGET/tools"
cp "$ROOT/Doctor.command" "$TARGET/Doctor.command"
cp "$ROOT/Uninstall.command" "$TARGET/Uninstall.command"
chmod 700 "$TARGET/runtime/bin/node" "$TARGET/app/node_modules/mobilecli/bin/"mobilecli-darwin-* "$TARGET/tools/android/platform-tools/adb" "$TARGET/Doctor.command" "$TARGET/Uninstall.command"
clear_quarantine "$TARGET"

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.one2all.pm.design-review-mobile-collector</string>
<key>ProgramArguments</key><array><string>$TARGET/runtime/bin/node</string><string>$TARGET/app/dist/index.js</string><string>start</string></array>
<key>EnvironmentVariables</key><dict><key>MOBILEWRIGHT_DISABLE_TELEMETRY</key><string>1</string><key>ANDROID_HOME</key><string>$TARGET/tools/android</string><key>ANDROID_SDK_ROOT</key><string>$TARGET/tools/android</string><key>PATH</key><string>$TARGET/runtime/bin:$TARGET/tools/android/platform-tools:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
<key>StandardOutPath</key><string>$TARGET/collector.log</string><key>StandardErrorPath</key><string>$TARGET/collector-error.log</string>
</dict></plist>
PLIST
launchctl bootout "gui/$UID" "$PLIST" 2>/dev/null || true
launchctl bootstrap "gui/$UID" "$PLIST"
"$TARGET/Doctor.command" || true
echo "PM 设计走查移动采集器 v${version} 已安装。"
`

const doctorScript = `#!/bin/zsh
set -uo pipefail
TARGET="$HOME/Library/Application Support/PM Design Review Mobile Collector"
NODE="$TARGET/runtime/bin/node"
APP="$TARGET/app/dist/index.js"
export ANDROID_HOME="$TARGET/tools/android"
export ANDROID_SDK_ROOT="$TARGET/tools/android"
export PATH="$TARGET/runtime/bin:$TARGET/tools/android/platform-tools:/usr/bin:/bin:/usr/sbin:/sbin"
if [[ ! -x "$NODE" || ! -f "$APP" ]]; then echo "尚未安装 PM 移动采集器。"; exit 1; fi
echo "固定运行时：Node ${NODE_VERSION} · Mobilewright 0.0.53 · mobilecli 1.0.0 · Android Platform-Tools ${PLATFORM_TOOLS_VERSION}"
if [[ ! -x "$TARGET/tools/android/platform-tools/adb" ]]; then echo "Android Platform-Tools 缺失，请重新安装。"; exit 1; fi
if [[ ! -f "$TARGET/agents/devicekit-android-${ANDROID_AGENT_VERSION}.apk" ]]; then echo "DeviceKit Android 辅助组件缺失，请重新安装。"; exit 1; fi
"$NODE" "$APP" version
"$NODE" "$APP" doctor "\${1:-}"
`

const uninstallScript = `#!/bin/zsh
set -euo pipefail
TARGET="$HOME/Library/Application Support/PM Design Review Mobile Collector"
PLIST="$HOME/Library/LaunchAgents/com.one2all.pm.design-review-mobile-collector.plist"
COLLECTOR_ID="$(/usr/bin/plutil -extract collector_id raw -o - "$TARGET/config.json" 2>/dev/null || true)"
launchctl bootout "gui/$UID" "$PLIST" 2>/dev/null || true
rm -f "$PLIST"
if [[ -n "$COLLECTOR_ID" ]]; then /usr/bin/security delete-generic-password -a "$COLLECTOR_ID" -s com.one2all.pm.design-review-mobile-collector >/dev/null 2>&1 || true; fi
rm -rf "$TARGET"
echo "PM 移动采集器及本机 Keychain 凭证已移除。"
`

await mkdir(outputDir, { recursive: true })
const platformTools = await cachedDownload(
  `https://dl.google.com/android/repository/platform-tools_r${PLATFORM_TOOLS_VERSION}-darwin.zip`,
  `platform-tools_r${PLATFORM_TOOLS_VERSION}-darwin.zip`,
  PLATFORM_TOOLS_CHECKSUM,
)
const androidAgent = await downloadAndroidAgent()
const manifest = {
  version,
  protocolVersion: '0.1.0',
  mobilewrightVersion: '0.0.53',
  mobilecliVersion: '1.0.0',
  nodeVersion: NODE_VERSION,
  androidPlatformToolsVersion: PLATFORM_TOOLS_VERSION,
  androidPlatformToolsSha256: PLATFORM_TOOLS_CHECKSUM,
  androidAgentVersion: ANDROID_AGENT_VERSION,
  androidAgentSha256: ANDROID_AGENT_CHECKSUM,
  generatedAt: new Date().toISOString(),
  artifacts: {},
}

for (const architecture of ['arm64', 'x64']) {
  const temporary = await mkdtemp(join(tmpdir(), `pm-design-review-mobile-${architecture}-`))
  try {
    const rootName = `PM Design Review Mobile Collector ${version}`
    const root = join(temporary, rootName)
    const app = join(root, 'app')
    const runtime = join(root, 'runtime')
    const tools = join(root, 'tools/android')
    const agents = join(root, 'agents')
    await run('pnpm', ['--filter', '@pm/design-review-mobile-collector', 'deploy', '--prod', '--legacy', app], { cwd: repositoryRoot })
    await cp(join(collectorRoot, 'dist'), join(app, 'dist'), { recursive: true, force: true })
    await assertStandalone(app)
    const mobilecli = await prepareNativeTools(app, architecture)
    await removeOtherSharpArchitecture(app, architecture)

    const nodeArchive = await cachedDownload(
      `https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-darwin-${architecture}.tar.gz`,
      `node-v${NODE_VERSION}-darwin-${architecture}.tar.gz`,
      NODE_CHECKSUMS[architecture],
    )
    await mkdir(runtime, { recursive: true })
    await run('tar', ['-xzf', nodeArchive, '-C', runtime, '--strip-components=1', `node-v${NODE_VERSION}-darwin-${architecture}/bin/node`])
    await mkdir(tools, { recursive: true })
    await run('unzip', ['-q', '-o', platformTools, '-d', tools])
    await mkdir(agents, { recursive: true })
    await cp(androidAgent, join(agents, `devicekit-android-${ANDROID_AGENT_VERSION}.apk`))
    const adb = join(tools, 'platform-tools/adb')
    const sourceProperties = await readFile(join(tools, 'platform-tools/source.properties'), 'utf8')
    const packagedPlatformToolsVersion = sourceProperties.match(/^Pkg\.Revision=(.+)$/m)?.[1]?.trim()
    if (packagedPlatformToolsVersion !== PLATFORM_TOOLS_VERSION) throw new Error(`Android Platform-Tools version mismatch: ${packagedPlatformToolsVersion ?? 'missing'}`)
    await stat(join(tools, 'platform-tools/NOTICE.txt'))
    await chmod(adb, 0o755)
    await Promise.all([
      writeFile(join(root, 'Install.command'), installScript, { mode: 0o755 }),
      writeFile(join(root, 'Doctor.command'), doctorScript, { mode: 0o755 }),
      writeFile(join(root, 'Uninstall.command'), uninstallScript, { mode: 0o755 }),
      chmod(join(runtime, 'bin/node'), 0o755),
    ])
    await run('zsh', ['-n', join(root, 'Install.command')])
    await run('zsh', ['-n', join(root, 'Doctor.command')])
    await run('zsh', ['-n', join(root, 'Uninstall.command')])
    const expectedArchitecture = architecture === 'arm64' ? 'arm64' : 'x86_64'
    for (const binary of [join(runtime, 'bin/node'), mobilecli]) {
      const architectures = machOArchitectures(await readFile(binary))
      if (!architectures.includes(expectedArchitecture)) throw new Error(`${basename(binary)} architecture mismatch: ${architectures.join(', ')}`)
    }
    const adbArchitectures = machOArchitectures(await readFile(adb))
    if (!adbArchitectures.includes('arm64') || !adbArchitectures.includes('x86_64')) throw new Error(`adb must be Universal Mach-O: ${adbArchitectures.join(', ')}`)
    const versionOutput = await new Promise((resolveVersion, reject) => {
      const child = spawn(join(runtime, 'bin/node'), [join(app, 'dist/index.js'), 'version'], { stdio: ['ignore', 'pipe', 'inherit'] })
      let output = ''; child.stdout.on('data', (chunk) => { output += chunk })
      child.on('error', reject); child.on('exit', (code) => code === 0 ? resolveVersion(output) : reject(new Error('packaged collector smoke test failed')))
    })
    const parsedVersion = JSON.parse(versionOutput)
    if (parsedVersion.collectorVersion !== version || parsedVersion.nodeVersion !== NODE_VERSION || parsedVersion.architecture !== architecture) {
      throw new Error(`packaged ${architecture} runtime version mismatch`)
    }

    const filename = `pm-design-review-mobile-collector-darwin-${architecture}-${version}.zip`
    const output = join(outputDir, filename)
    await rm(output, { force: true })
    await run('zip', ['-q', '-y', '-r', '-X', output, rootName], { cwd: temporary })
    await run('unzip', ['-tq', output])
    const bytes = await readFile(output)
    const digest = sha256(bytes)
    await writeFile(`${output}.sha256`, `${digest}  ${filename}\n`)
    manifest.artifacts[architecture] = { file: filename, sha256: digest, byteSize: bytes.byteLength }
  } finally {
    await rm(temporary, { recursive: true, force: true })
  }
}

const extensionName = `pm-design-review-extension-${version}.zip`
try {
  const extension = await readFile(join(outputDir, extensionName))
  manifest.artifacts.extension = { file: extensionName, sha256: sha256(extension), byteSize: extension.byteLength }
} catch {}
const manifestName = `pm-design-review-clients-${version}.json`
const manifestBody = `${JSON.stringify(manifest, null, 2)}\n`
await writeFile(join(outputDir, manifestName), manifestBody)
await writeFile(join(outputDir, `${manifestName}.sha256`), `${sha256(manifestBody)}  ${manifestName}\n`)
for (const artifact of Object.values(manifest.artifacts)) await stat(join(outputDir, artifact.file))
console.log(`Packaged PM design-review clients ${version} -> ${outputDir}`)
