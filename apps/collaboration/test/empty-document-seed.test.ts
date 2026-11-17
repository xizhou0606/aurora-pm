import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'bun:test'
import * as Y from 'yjs'
import { documentFromProjection, projectAndValidate, projectionHash, sha256 } from '../src/projection.js'

describe('Go embedded empty document seed', () => {
  it('is a decodable engine seed with the exact default schema and hashes', () => {
    const artifact = JSON.parse(readFileSync(new URL('../../../backend/internal/module/collaboration/empty-document-seed.json', import.meta.url), 'utf8'))
    const doc = new Y.Doc()
    const snapshot = Buffer.from(artifact.snapshot, 'base64')
    Y.applyUpdate(doc, snapshot)
    const vector = Y.encodeStateVector(doc)
    const { projection, references } = projectAndValidate(doc)
    const regenerated = documentFromProjection({ type: 'doc', content: [{ type: 'paragraph' }] })
    expect(projection).toEqual(projectAndValidate(regenerated).projection)
    expect(projection).toEqual(artifact.projection)
    expect(sha256(snapshot)).toBe(artifact.snapshot_hash)
    expect(Buffer.from(vector).toString('base64')).toBe(artifact.state_vector)
    expect(sha256(vector)).toBe(artifact.state_vector_hash)
    expect(projectionHash(projection)).toBe(artifact.projection_hash)
    expect(references.assetIds).toHaveLength(0)
    expect(references.bitableResourceIds).toHaveLength(0)
    doc.destroy()
    regenerated.destroy()
  })
})
