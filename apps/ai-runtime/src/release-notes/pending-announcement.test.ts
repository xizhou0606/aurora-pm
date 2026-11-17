import { execFileSync } from 'node:child_process'
import { createHmac } from 'node:crypto'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { AddressInfo } from 'node:net'

import { describe, expect, it } from 'bun:test'

import {
  ensureGitSafeDirectory,
  fetchLatestPublishedNote,
  runReleaseNotesOnStart,
  shouldAutoPublish,
  signReleasePayload,
} from './pending-announcement.js'

function productionEnv(overrides: Record<string, string | undefined> = {}): NodeJS.ProcessEnv {
  return {
    PM_ENV: 'production',
    AI_INTERNAL_SECRET: 'a'.repeat(48),
    PM_RELEASE_API_URL: 'http://pm-api:8080',
    ...overrides,
  }
}

describe('release notes on-start', () => {
  it('runs for a fully configured production container', () => {
    expect(shouldAutoPublish(productionEnv()).run).toBe(true)
  })

  it('skips non-production environments and the explicit off switch', () => {
    expect(shouldAutoPublish(productionEnv({ PM_ENV: 'development' })).reason)
      .toContain('非生产环境')
    expect(shouldAutoPublish(productionEnv({ PM_RELEASE_NOTES_AUTO: 'false' })).reason)
      .toContain('PM_RELEASE_NOTES_AUTO=false')
  })

  it('skips when the signing secret or upload target is missing', () => {
    const noSecret = productionEnv()
    delete noSecret.AI_INTERNAL_SECRET
    expect(shouldAutoPublish(noSecret).reason).toContain('AI_INTERNAL_SECRET')
    expect(shouldAutoPublish(productionEnv({ PM_RELEASE_API_URL: undefined })).reason)
      .toContain('PM_RELEASE_API_URL')
  })

  it('signs exactly the timestamp-payload pair the pm-api contract verifies', () => {
    const secret = 'shared-secret'
    const timestampMs = '1789031100126'
    const payloadJson = '{"version":"1.7.0","contentMd":"### x"}'
    const expected = createHmac('sha256', secret)
      .update(`${timestampMs}.${payloadJson}`)
      .digest('hex')
    expect(signReleasePayload(secret, timestampMs, payloadJson)).toBe(expected)
  })

  it('exposes the mounted repo as a trusted git directory via a global config file', () => {
    const env: NodeJS.ProcessEnv = {}
    const configPath = ensureGitSafeDirectory('/repo', env)
    expect(env.GIT_CONFIG_GLOBAL).toBe(configPath)
    expect(readFileSync(configPath, 'utf8')).toContain('directory = /repo')
  })
})

// ---- 已入库公告优先的锚点/版本推导(容器启动器端到端) ----

const TEST_SECRET = 'shared-secret-at-least-32-characters!!'

const CHANGELOG_TEMPLATE = `# 更新日志

## [1.6.0] - 2026-09-09 <!-- pm:auto release=MARKER_PLACEHOLDER -->

### 更新摘要

- 初始版本
`

type FakeApi = {
  url: string
  posts: Array<Record<string, unknown>>
  close: () => Promise<void>
}

// 复刻 pm-api 内部契约:HMAC 覆盖 `${timestamp}.${rawBody}`(GET 空 body),
// 最新公告返回 {data},入库 POST 返回 200。
async function startFakePmApi(
  options: { latest: { version: string; release_sha: string } | null; latestStatus?: number },
): Promise<FakeApi> {
  const posts: Array<Record<string, unknown>> = []
  const server = createServer((req, res) => {
    const chunks: Buffer[] = []
    req.on('data', (chunk: Buffer) => chunks.push(chunk))
    req.on('end', () => {
      const raw = Buffer.concat(chunks)
      const timestamp = req.headers['x-pm-timestamp']
      const signature = req.headers['x-pm-signature']
      const expected = signature
        ? createHmac('sha256', TEST_SECRET).update(`${timestamp}.${raw}`).digest('hex')
        : ''
      if (expected !== signature) {
        res.writeHead(401).end()
        return
      }
      if (req.method === 'GET' && req.url === '/internal/release-notes/latest') {
        if (options.latestStatus) {
          res.writeHead(options.latestStatus).end()
          return
        }
        res.writeHead(200, { 'content-type': 'application/json' })
        res.end(JSON.stringify({ data: options.latest }))
        return
      }
      if (req.method === 'POST' && req.url === '/internal/release-notes') {
        posts.push(JSON.parse(raw.toString('utf8')) as Record<string, unknown>)
        res.writeHead(200, { 'content-type': 'application/json' })
        res.end(JSON.stringify({ data: {} }))
        return
      }
      res.writeHead(404).end()
    })
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address() as AddressInfo
  return {
    url: `http://127.0.0.1:${port}`,
    posts,
    close: () => new Promise((resolve) => server.close(() => resolve())),
  }
}

type FixtureRepo = { dir: string; headSha: string; featASha: string }

function commit(repoDir: string, message: string): string {
  execFileSync('git', ['-C', repoDir, '-c', 'user.name=ci', '-c', 'user.email=ci@example.com',
    'commit', '-q', '--allow-empty', '-m', message])
  return execFileSync('git', ['-C', repoDir, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
}

// 锚点链:1.6.0 区块标记指向仓库里真实存在的提交(docs 提交不进公告窗口)。
function initFixtureRepo(): FixtureRepo {
  const dir = mkdtempSync(join(tmpdir(), 'pm-release-notes-'))
  execFileSync('git', ['-C', dir, 'init', '-q', '-b', 'main'])
  writeFileSync(join(dir, 'CHANGELOG.md'), CHANGELOG_TEMPLATE.replace('MARKER_PLACEHOLDER', '0'.repeat(40)))
  execFileSync('git', ['-C', dir, 'add', '.'])
  const baseSha = commit(dir, 'docs(changelog): 发布 1.6.0 版本公告')
  writeFileSync(join(dir, 'CHANGELOG.md'), CHANGELOG_TEMPLATE.replace('MARKER_PLACEHOLDER', baseSha))
  execFileSync('git', ['-C', dir, 'add', '.'])
  commit(dir, 'docs(changelog): 修正 1.6.0 区块锚点标记')
  const featASha = commit(dir, 'feat(reporting): 汇报页新增岗位焦点面板')
  commit(dir, 'feat(documents): 隔离个人目录位置')
  const headSha = commit(dir, 'fix(quality): 修复状态流转粘贴图片未同步')
  return { dir, headSha, featASha }
}

describe('release notes on-start anchoring', () => {
  it('queries the latest published note with the shared empty-body HMAC contract', async () => {
    const api = await startFakePmApi({ latest: { version: '1.7.0', release_sha: 'a'.repeat(40) } })
    try {
      await expect(fetchLatestPublishedNote(api.url, TEST_SECRET)).resolves.toEqual({
        version: '1.7.0', releaseSha: 'a'.repeat(40),
      })
      await expect(fetchLatestPublishedNote(api.url, 'wrong-secret!'.repeat(3))).rejects.toThrow()
    } finally {
      await api.close()
    }
  })

  it('anchors on the newest published note so the next deploy derives a new version', async () => {
    const repo = initFixtureRepo()
    // 模拟 #78 已入库 1.7.0(releaseSha = 当时的 HEAD,即首个 feat 提交)。
    const api = await startFakePmApi({
      latest: { version: '1.7.0', release_sha: repo.featASha },
    })
    try {
      const outcome = await runReleaseNotesOnStart({
        repoDir: repo.dir, apiUrl: api.url, secret: TEST_SECRET, useAi: false, env: {},
      })
      expect(outcome).toMatchObject({ action: 'uploaded', version: '1.8.0' })
      expect(api.posts).toHaveLength(1)
      expect(api.posts[0]).toMatchObject({ version: '1.8.0', releaseSha: repo.headSha })
      const contentMd = String(api.posts[0].contentMd)
      expect(contentMd).toContain('隔离个人目录位置(documents)')
      expect(contentMd).toContain('修复状态流转粘贴图片未同步(quality)')
      // 锚点之前的提交不得再次进入公告。
      expect(contentMd).not.toContain('汇报页新增岗位焦点面板')
    } finally {
      await api.close()
    }
  }, 20_000)

  it('falls back to the CHANGELOG anchor when pm-api has no published notes', async () => {
    const repo = initFixtureRepo()
    const api = await startFakePmApi({ latest: null })
    try {
      const outcome = await runReleaseNotesOnStart({
        repoDir: repo.dir, apiUrl: api.url, secret: TEST_SECRET, useAi: false, env: {},
      })
      expect(outcome).toMatchObject({ action: 'uploaded', version: '1.7.0' })
      const contentMd = String(api.posts[0].contentMd)
      expect(contentMd).toContain('汇报页新增岗位焦点面板(reporting)')
    } finally {
      await api.close()
    }
  }, 20_000)

  it('skips when the published anchor is already HEAD', async () => {
    const repo = initFixtureRepo()
    const api = await startFakePmApi({
      latest: { version: '1.8.0', release_sha: repo.headSha },
    })
    try {
      const outcome = await runReleaseNotesOnStart({
        repoDir: repo.dir, apiUrl: api.url, secret: TEST_SECRET, useAi: false, env: {},
      })
      expect(outcome.action).toBe('skip')
      expect(outcome.reason).toContain('没有可归档的常规提交')
      expect(api.posts).toHaveLength(0)
    } finally {
      await api.close()
    }
  }, 20_000)

  it('fails closed when the latest-note query errors instead of using a frozen anchor', async () => {
    const repo = initFixtureRepo()
    const api = await startFakePmApi({ latest: null, latestStatus: 500 })
    try {
      const outcome = await runReleaseNotesOnStart({
        repoDir: repo.dir, apiUrl: api.url, secret: TEST_SECRET, useAi: false, env: {},
      })
      expect(outcome.action).toBe('error')
      expect(outcome.reason).toContain('HTTP 500')
      expect(api.posts).toHaveLength(0)
    } finally {
      await api.close()
    }
  }, 20_000)
})
