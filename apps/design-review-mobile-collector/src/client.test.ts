import { describe, expect, it, vi } from 'bun:test'
import { pairCollector } from './client'

describe('collector client', () => {
  it('pairs without placing the one-time code in the URL', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ data: { collector: { id: 'collector-id' }, token: 'secret-token' } }), { status: 201, headers: { 'content-type': 'application/json' } }))
    const adapter = { listDevices: async () => [], capture: async () => { throw new Error('unused') } }
    const result = await pairCollector('https://pm.example.test', '123456', 'mac', adapter)
    expect(result.collector_id).toBe('collector-id')
    expect(String(fetchMock.mock.calls[0]?.[0])).not.toContain('123456')
    expect(fetchMock.mock.calls[0]?.[1]?.body).toContain('123456')
    fetchMock.mockRestore()
  })
})
