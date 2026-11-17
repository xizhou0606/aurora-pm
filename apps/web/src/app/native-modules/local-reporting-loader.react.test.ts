import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'

const pageComponent = (() => vi.fn(() => null))()
const feature = ((() => ({
  ReportingPage: pageComponent,
}))()) as { ReportingPage: unknown }

mock.module('@/features/reporting', () => feature)

import {
  loadLocalReporting,
  resetLocalReportingLoaderForTests,
} from './local-reporting-loader'

describe('local Reporting loader', () => {
  beforeEach(() => {
    feature.ReportingPage = pageComponent
    resetLocalReportingLoaderForTests()
  })

  it('loads and caches the PM-owned feature without a remote manifest', async () => {
    const first = await loadLocalReporting()
    const second = await loadLocalReporting()

    expect(first).toBe(pageComponent)
    expect(second).toBe(first)
  })

  it('rejects an incompatible PM-owned feature export', async () => {
    // bun 的 mock.module 对工厂结果做快照，重注册才能让加载器读到不兼容导出。
    mock.module('@/features/reporting', () => ({ ReportingPage: undefined }))

    await expect(loadLocalReporting()).rejects.toThrow(
      'Local Reporting feature contract is incompatible',
    )
  })
})
