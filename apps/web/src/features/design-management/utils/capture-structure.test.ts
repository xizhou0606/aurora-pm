import { describe, expect, it } from 'bun:test'

import type { DesignReviewTarget, MobileCaptureDraft } from '../model'
import { mobileStructureFile, webCaptureStructure, webStructureFiles } from './capture-structure'

async function fileJSON(file: File): Promise<Record<string, unknown>> {
  return JSON.parse(await file.text()) as Record<string, unknown>
}

const webElement = {
  id: 'web-1', parent_id: null, tag: 'button', role: 'button', text: '保存',
  bounds: { x: 12, y: 20, width: 100, height: 40 }, styles: {}, clipped: false,
  structural_fingerprint: 'stable-button', control_state: null, allowed_operations: [],
}

describe('design review capture structure evidence', () => {
  it('keeps normalized and original web inventories distinct', () => {
    const capture = webCaptureStructure({
      actual_url: 'https://captured.example.test/page',
      elements: [{ ...webElement, text: '已归一化' }],
      structure_truncated: false,
      inventory: { elements: [webElement], truncated: true },
      warnings: [{ code: 'partial_style', message: '部分样式不可读取' }, { code: 1 }],
    })
    expect(capture).toMatchObject({
      actualUrl: 'https://captured.example.test/page', structureTruncated: false, originalStructureTruncated: true,
      warnings: [{ code: 'partial_style', message: '部分样式不可读取' }],
    })
    expect(capture?.elements).not.toBe(capture?.originalElements)
  })

  it('offsets web element coordinates and viewport for the iOS topbar', async () => {
    const capture = webCaptureStructure({ actual_url: 'https://captured.example.test/page', elements: [webElement], structure_truncated: false })
    const files = webStructureFiles(capture, {
      actualUrl: 'https://requested.example.test/page', viewportWidth: 390, viewportHeight: 844,
      topbar: { enabled: true, platform: 'ios', height: 44 },
    })
    const current = await fileJSON(files.current)
    expect(files.current.name).toBe('implementation-structure.json')
    expect(files.current.type).toBe('application/json')
    expect(current.actualUrl).toBe('https://captured.example.test/page')
    expect(current.contentViewport).toEqual({ width: 390, height: 800 })
    expect(current.finalViewport).toEqual({ width: 390, height: 844 })
    expect(current.elements).toEqual([{ ...webElement, bounds: { ...webElement.bounds, y: 64 } }])
  })

  it('marks manual image fallback structure as explicitly incomplete', async () => {
    const { current, original } = webStructureFiles(undefined, {
      actualUrl: 'https://requested.example.test/page', viewportWidth: 1440, viewportHeight: 900,
      topbar: { enabled: false },
    })
    expect(original).toBeUndefined()
    expect(await fileJSON(current)).toMatchObject({
      source: 'web', actualUrl: 'https://requested.example.test/page', elements: [], structureTruncated: true,
      contentViewport: { width: 1440, height: 900 }, finalViewport: { width: 1440, height: 900 },
      warnings: [{ code: 'structure_unavailable' }],
    })
  })

  it('serializes native elements and nullable app metadata without inventing values', async () => {
    const target = {
      kind: 'mobile', platform: 'ios', app_id: 'com.example.app', app_name: undefined, app_version: undefined,
      orientation: 'portrait', device: {}, screen: { width: 390, height: 844, scale: 3 },
      collector_version: '0.1.0', mobilewright_version: '0.0.53',
    } satisfies DesignReviewTarget
    const capture = {
      id: 'capture-1', collector_id: 'collector-1', status: 'succeeded', platform: 'ios', device_id: 'device-1',
      target, elements: [{ id: 'native-1' }], structure_truncated: false, warnings: [], screenshot_available: true,
      expires_at: '2026-09-04T00:00:00Z',
    } satisfies MobileCaptureDraft
    const structure = await fileJSON(mobileStructureFile(capture, target, 390, 844))
    expect(structure).toMatchObject({
      source: 'mobile', target: {
        kind: 'mobile', platform: 'ios', app_id: 'com.example.app', orientation: 'portrait', device: {},
        screen: { width: 390, height: 844, scale: 3 }, collector_version: '0.1.0', mobilewright_version: '0.0.53',
      }, elements: [{ id: 'native-1' }], structureTruncated: false,
      syntheticMobileTopbar: null, contentViewport: { width: 390, height: 844 }, finalViewport: { width: 390, height: 844 },
    })
    expect((structure.target as Record<string, unknown>).app_name).toBeUndefined()
  })
})
