import { describe, expect, it } from 'bun:test'
import { validateServerURL } from './config'

describe('collector configuration', () => {
  it('requires HTTPS except for loopback development', () => {
    expect(validateServerURL('https://pm.example.test/path')).toBe('https://pm.example.test')
    expect(validateServerURL('http://127.0.0.1:5175')).toBe('http://127.0.0.1:5175')
    expect(() => validateServerURL('http://pm.example.test')).toThrow('HTTPS')
  })
})
