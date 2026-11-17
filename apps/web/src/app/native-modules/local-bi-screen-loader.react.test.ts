import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'

const feature = ((): { BiScreenPage: unknown } => ({
  BiScreenPage: vi.fn(() => null),
}))()

mock.module('@/features/bi-screen', () => feature)

import {
  loadLocalBiScreen,
  resetLocalBiScreenLoaderForTests,
} from './local-bi-screen-loader'

describe('local BI Screen loader', () => {
  beforeEach(() => {
    feature.BiScreenPage = vi.fn(() => null)
    resetLocalBiScreenLoaderForTests()
  })

  it('loads and caches the PM-owned feature without a remote manifest', async () => {
    const first = await loadLocalBiScreen()
    const second = await loadLocalBiScreen()

    expect(first).toBe(feature.BiScreenPage as never)
    expect(second).toBe(first)
  })

  it('rejects an incompatible PM-owned feature export', async () => {
    // bun 的 mock.module 对工厂结果做快照，重注册才能让加载器读到不兼容导出。
    mock.module('@/features/bi-screen', () => ({ BiScreenPage: undefined }))

    await expect(loadLocalBiScreen()).rejects.toThrow(
      'Local BI Screen feature contract is incompatible',
    )
  })
})
