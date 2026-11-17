import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { DocumentCollaborationPresence } from './DocumentCollaborationPresence'

describe('DocumentCollaborationPresence', () => {
  it('shows only the workflow-projected actor roster', () => {
    render(<DocumentCollaborationPresence members={[
      { clientId: 5, actorId: 'actor-5', actorName: '服务端成员', hasCursor: true },
    ]} />)
    expect(screen.getByLabelText(DOCUMENTS_COPY.editor.collaborationOnline(1))).toBeTruthy()
    const roster = screen.getByLabelText(DOCUMENTS_COPY.editor.collaborationOnline(1))
    expect(roster.textContent).toContain('服务端成员')
  })
})
