import { afterEach, describe, expect, it } from 'bun:test'
import { platformApi } from '@/shared/api'
import { fetchAuthorizationRecords, revokeAuthorizationRecord } from './authorization-records'
const adapter = platformApi.defaults.adapter
afterEach(() => { platformApi.defaults.adapter = adapter })
describe('authorization records API', () => {
  it('requests server pagination and preserves the server total', async () => {
    platformApi.defaults.adapter = async (config) => {
      expect(config.url).toBe('/external-tooling/authorization-records')
      expect(config.params).toEqual({ page: 3, page_size: 10 })
      return { config, status: 200, statusText: 'OK', headers: {}, data: { data: { items: [], total: 22, page: 3, page_size: 10 } } }
    }
    expect((await fetchAuthorizationRecords(3, 10)).total).toBe(22)
  })
  it('revokes only the selected record and propagates rejected writes', async () => {
    platformApi.defaults.adapter = async (config) => {
      expect(config.url).toBe('/external-tooling/authorization-records/device%3A17/revoke')
      expect(config.method).toBe('post')
      throw { response: { status: 403, data: { error: { code: 'FORBIDDEN' } } } }
    }
    await expect(revokeAuthorizationRecord('device:17')).rejects.toBeDefined()
  })
})
