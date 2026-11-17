import { describe, expect, it } from 'bun:test'

import { nativeModuleIdForPath } from '.'

describe('native module public contracts', () => {
  it('matches only the seven typed module routes', () => {
    expect(nativeModuleIdForPath('/workspace/inbox')).toBe('inbox')
    expect(nativeModuleIdForPath('/workspace/work-items/31')).toBe('workItems')
    expect(nativeModuleIdForPath('/workspace/bi-screen')).toBe('biScreen')
    expect(nativeModuleIdForPath('/workspace/quality-management/bugs/new')).toBe('quality')
    expect(nativeModuleIdForPath('/workspace/quality-management/bugs/8')).toBe('quality')
    expect(nativeModuleIdForPath('/workspace/work-items/not-a-number')).toBeUndefined()
    expect(nativeModuleIdForPath('/workspace/quality-management/bugs/8/delete')).toBeUndefined()
  })

})
