import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'

import { sameOriginApi } from '@/shared/api'

import {
  fetchReleaseNotes,
  RELEASE_NOTES_ENDPOINT,
} from './release-notes'

const originalAdapter = sameOriginApi.defaults.adapter

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

const CONTENT_MD = [
  '### 更新摘要',
  '',
  '- 摘要一',
  '- 摘要二',
  '',
  '### 新增',
  '',
  '- 新增一',
].join('\n')

describe('release notes public API', () => {
  afterEach(() => {
    sameOriginApi.defaults.adapter = originalAdapter
  })

  it('parses contentMd blocks and keeps records newest first', async () => {
    sameOriginApi.defaults.adapter = async (config) => {
      expect(config.url).toBe(RELEASE_NOTES_ENDPOINT)
      return response(config, { data: [
        {
          version: '1.1.0',
          date: '2026-09-03',
          contentMd: CONTENT_MD,
          releaseSha: 'abc123',
        },
        { version: '1.0.0', date: '2026-08-01', contentMd: '', releaseSha: 'def456' },
      ] })
    }

    const entries = await fetchReleaseNotes()
    expect(entries).toEqual([
      {
        version: '1.1.0',
        date: '2026-09-03',
        summary: ['摘要一', '摘要二'],
        sections: [{ title: '新增', items: ['新增一'] }],
      },
      { version: '1.0.0', date: '2026-08-01', summary: [], sections: [] },
    ])
  })

  it('accepts the snake_case field variants served by the PM API', async () => {
    sameOriginApi.defaults.adapter = async (config) => (
      response(config, { data: [
        {
          version: '2.0.0',
          date: '2026-09-09',
          content_md: CONTENT_MD,
          release_sha: 'aaa111',
        },
      ] })
    )

    const entries = await fetchReleaseNotes()
    expect(entries).toEqual([
      {
        version: '2.0.0',
        date: '2026-09-09',
        summary: ['摘要一', '摘要二'],
        sections: [{ title: '新增', items: ['新增一'] }],
      },
    ])
  })

  it('accepts a direct array payload without an envelope', async () => {
    sameOriginApi.defaults.adapter = async (config) => (
      response(config, [
        { version: '2.0.0', date: '2026-09-09', contentMd: '### 更新摘要\n\n- 直发', },
      ])
    )

    const entries = await fetchReleaseNotes()
    expect(entries).toEqual([
      { version: '2.0.0', date: '2026-09-09', summary: ['直发'], sections: [] },
    ])
  })

  it('drops rows without a usable version', async () => {
    sameOriginApi.defaults.adapter = async (config) => (
      response(config, { data: [
        { version: '  ', date: '2026-09-03', contentMd: '' },
        { date: '2026-09-02', contentMd: '' },
        { version: '1.0.0', date: '2026-08-01', contentMd: '' },
      ] })
    )

    const entries = await fetchReleaseNotes()
    expect(entries.map((entry) => entry.version)).toEqual(['1.0.0'])
  })

  it('fails closed when the payload is not an array', async () => {
    sameOriginApi.defaults.adapter = async (config) => (
      response(config, { data: { version: '1.0.0' } })
    )

    await expect(fetchReleaseNotes()).rejects.toMatchObject({
      name: 'PMSafeApiError',
      code: 'RELEASE_NOTES_INVALID',
    })
  })

  it('maps transport failures onto a safe error code', async () => {
    sameOriginApi.defaults.adapter = async () => {
      throw new Error('network down')
    }

    await expect(fetchReleaseNotes()).rejects.toMatchObject({
      name: 'PMSafeApiError',
      code: 'RELEASE_NOTES_LOAD_FAILED',
    })
  })
})
