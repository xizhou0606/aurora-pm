import { beforeEach, describe, expect, it, mock, vi } from 'bun:test'

const pageComponent = (() => vi.fn(() => null))()
const feature = ((() => ({
  QualityManagementPage: pageComponent,
}))()) as { QualityManagementPage: unknown }

mock.module('@/features/quality-management', () => feature)

import {
  loadLocalQuality,
  resetLocalQualityLoaderForTests,
} from './local-quality-loader'

describe('local Quality loader', () => {
  beforeEach(() => {
    feature.QualityManagementPage = pageComponent
    resetLocalQualityLoaderForTests()
  })

  it('loads and caches the PM-owned feature without a remote manifest', async () => {
    const first = await loadLocalQuality()
    const second = await loadLocalQuality()

    expect(first).toBe(pageComponent)
    expect(second).toBe(first)
  })

  it('rejects an incompatible PM-owned feature export', async () => {
    mock.module('@/features/quality-management', () => ({ QualityManagementPage: undefined }))

    await expect(loadLocalQuality()).rejects.toThrow(
      'Local Quality feature contract is incompatible',
    )
  })
})
