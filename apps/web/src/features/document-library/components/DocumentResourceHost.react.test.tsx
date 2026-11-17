import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { DOCUMENT_LIBRARY_COPY as copy } from '@/shared/copy'

import { DocumentResourceHost } from './DocumentResourceHost'
import {
  createDefaultDocumentResourceRendererRegistry,
  createDocumentResourceRendererRegistry,
} from '../model'

describe('DocumentResourceHost', () => {
  it('registers rich text and bitable without branching inside the host', () => {
    const registry = createDefaultDocumentResourceRendererRegistry({
      RichTextRenderer: ({ resource }) => <div>{`rich:${resource.payloadRef}`}</div>,
      BitableRenderer: ({ resource }) => <div>{`table:${resource.payloadRef}`}</div>,
    })
    const { rerender } = render(
      <DocumentResourceHost
        registry={registry}
        view={{ status: 'ready', readOnly: false, resource: {
          resourceId: 'resource-1', typeKey: 'rich_text', payloadRef: 'document:11',
        } }}
      />,
    )
    expect(screen.getByText('rich:document:11')).toBeTruthy()
    rerender(
      <DocumentResourceHost
        registry={registry}
        view={{ status: 'ready', readOnly: true, resource: {
          resourceId: 'resource-2', typeKey: 'bitable', payloadRef: 'bitable:abc',
        } }}
      />,
    )
    expect(screen.getByText('table:bitable:abc')).toBeTruthy()
  })

  it('adds a third type through registry data only', () => {
    const registry = createDocumentResourceRendererRegistry([{
      typeKey: 'test_canvas',
      Renderer: ({ resource }) => <div>{resource.payloadRef}</div>,
    }])
    render(
      <DocumentResourceHost
        registry={registry}
        view={{ status: 'ready', readOnly: false, resource: {
          resourceId: 'resource-3', typeKey: 'test_canvas', payloadRef: 'canvas:1',
        } }}
      />,
    )
    expect(screen.getByText('canvas:1')).toBeTruthy()
  })

  it('fails visibly for unknown types and missing unified resource IDs', () => {
    const registry = createDocumentResourceRendererRegistry([])
    const { rerender } = render(
      <DocumentResourceHost
        registry={registry}
        view={{ status: 'ready', readOnly: false, resource: {
          resourceId: 'resource-1', typeKey: 'unknown', payloadRef: 'unknown:1',
        } }}
      />,
    )
    expect(screen.getByText(copy.resource.unavailableReason)).toBeTruthy()
    rerender(
      <DocumentResourceHost
        registry={registry}
        view={{ status: 'ready', readOnly: false, resource: {
          resourceId: '', typeKey: 'rich_text', payloadRef: 'document:11',
        } }}
      />,
    )
    expect(screen.getByText(copy.resource.invalidReason)).toBeTruthy()
  })
})
