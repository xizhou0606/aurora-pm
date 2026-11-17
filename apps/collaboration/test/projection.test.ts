import { describe, expect, it } from 'bun:test'
import { documentFromProjection, projectAndValidate, projectionHash, sha256 } from '../src/projection.js'

const bitable = {
  type: 'bitable',
  attrs: {
    schemaVersion: 1,
    resourceId: '00000000-0000-4000-8000-000000000001',
    tableId: '00000000-0000-4000-8000-000000000002',
    viewId: '00000000-0000-4000-8000-000000000003',
    height: 420,
  },
}

function project(content: Record<string, unknown>[]) {
  return projectAndValidate(documentFromProjection({ type: 'doc', content }))
}

describe('projection authority hash contract', () => {
  it('matches Go canonical JSON ordering and separator escaping', () => {
    const projection = { z: 1, a: '<tag>&\u2028\u2029' }
    expect(projectionHash(projection)).toBe(sha256('{"a":"<tag>&\\u2028\\u2029","z":1}'))
  })

  it('roundtrips the real document image and bitable nodes with canonical references', () => {
    const result = project([
      { type: 'image', attrs: { src: '/api/v1/platform/documents/7/assets/31/content', alt: 'proof', title: null, assetId: 31 } },
      bitable,
      { type: 'image', attrs: { src: '/api/v1/platform/documents/7/assets/31/content', assetId: 31 } },
    ])

    expect(result.references).toEqual({
      assetIds: ['31'],
      bitableResourceIds: [bitable.attrs.resourceId],
    })
    expect(result.projection.content?.map((node) => node.type)).toEqual(['image', 'bitable', 'image'])
    expect(result.projection.content?.[0]).toMatchObject({ type: 'image', attrs: { assetId: 31 } })
    expect(result.projection.content?.[1]).toEqual(bitable)
    expect(projectionHash(projectAndValidate(documentFromProjection(result.projection)).projection))
      .toBe(projectionHash(result.projection))
  })

  it('accepts every production document node family used by the web V3 editor', () => {
    const result = project([
      { type: 'heading', attrs: { level: 6 }, content: [{ type: 'text', marks: [{ type: 'bold' }], text: 'Heading' }] },
      { type: 'taskList', content: [{ type: 'taskItem', attrs: { checked: false }, content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Task' }] }] }] },
      { type: 'table', content: [{ type: 'tableRow', content: [
        { type: 'tableHeader', attrs: { colspan: 1, rowspan: 1, colwidth: null }, content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Header' }] }] },
        { type: 'tableCell', attrs: { colspan: 1, rowspan: 1, colwidth: null }, content: [{ type: 'paragraph' }] },
      ] }] },
      { type: 'callout', attrs: { tone: 'warning' }, content: [{ type: 'mention', attrs: { id: '7', label: 'Alice', username: 'alice' } }] },
      { type: 'tableOfContents', attrs: { items: [{ anchor: 'section-1', level: 1, text: 'Heading' }] } },
      { type: 'unsupportedBlock', attrs: { original: '{"type":"future"}', originalType: 'future' } },
      bitable,
    ])

    expect(result.projection.content?.map((node) => node.type)).toEqual([
      'heading', 'taskList', 'table', 'callout', 'tableOfContents', 'unsupportedBlock', 'bitable',
    ])
    expect(result.references).toEqual({ assetIds: [], bitableResourceIds: [bitable.attrs.resourceId] })
  })

  it('allows external images without inventing an asset reference', () => {
    expect(project([{ type: 'image', attrs: { src: 'https://cdn.example.test/proof.png', assetId: null } }]).references)
      .toEqual({ assetIds: [], bitableResourceIds: [] })
  })

  it.each([0, -1, 1.5, '31', Number.MAX_SAFE_INTEGER + 1])(
    'rejects malicious image assetId %j',
    (assetId) => {
      expect(() => project([{ type: 'image', attrs: { src: '/private/content', assetId } }]))
        .toThrowError(expect.objectContaining({ reason: 'invalid-asset-reference' }))
    },
  )

  it.each([
    { ...bitable.attrs, schemaVersion: 2 },
    { ...bitable.attrs, resourceId: '../other-resource' },
    { ...bitable.attrs, tableId: 'table/other' },
    { ...bitable.attrs, viewId: 'view?token=secret' },
    { ...bitable.attrs, height: 901 },
  ])('rejects a malicious bitable reference %#', (attrs) => {
    expect(() => project([{ type: 'bitable', attrs }]))
      .toThrowError(expect.objectContaining({ reason: 'invalid-bitable-reference' }))
  })

  it('does not retain obsolete fixture nodes or credential-shaped extension attributes', () => {
    expect(() => documentFromProjection({
      type: 'doc',
      content: [{ type: 'assetReference', attrs: { assetId: 'asset-1' } }],
    })).toThrow()

    const result = project([{ type: 'bitable', attrs: { ...bitable.attrs, token: 'never-store', rows: [] } }])
    expect(result.projection).not.toHaveProperty('token')
    expect(JSON.stringify(result.projection)).not.toContain('never-store')
    expect(result.projection.content?.[0]).toEqual(bitable)
  })
})
