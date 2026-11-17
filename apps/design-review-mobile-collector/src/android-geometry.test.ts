import { describe, expect, it } from 'bun:test'
import { managedAndroidSDKRoot, normalizeAndroidScreenGeometry, parseAndroidDensity, pngDimensions } from './android-geometry'

function png(width: number, height: number) {
  const result = Buffer.alloc(24)
  Buffer.from('89504e470d0a1a0a', 'hex').copy(result)
  result.writeUInt32BE(width, 16); result.writeUInt32BE(height, 20)
  return result
}

describe('Android geometry', () => {
  it('uses the PM managed SDK directory', () => {
    expect(managedAndroidSDKRoot('/Users/demo/tools/android')
  })
  it('parses density and physical PNG dimensions', () => {
    expect(parseAndroidDensity('Physical density: 420\nOverride density: 450')).toBe(450)
    expect(pngDimensions(png(1080, 2340))).toEqual({ width: 1080, height: 2340 })
  })
  it('normalizes physical Android coordinates', () => {
    expect(normalizeAndroidScreenGeometry({ width: 1080, height: 2340, scale: 1 }, { width: 1080, height: 2340 }, 450).screen)
      .toEqual({ width: 384, height: 832, scale: 2.8125 })
  })
})
