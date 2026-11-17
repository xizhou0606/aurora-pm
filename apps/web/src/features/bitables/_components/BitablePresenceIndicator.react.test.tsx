import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { BITABLE_COLLABORATION_COPY as copy } from '@/shared/copy'
import { BitablePresenceIndicator } from './BitablePresenceIndicator'

describe('BitablePresenceIndicator', () => {
  it('shows the server-projected online roster', () => {
    render(<BitablePresenceIndicator presence={{
      people: [{ id: 'public-1', name: '甲' }, { id: 'public-2', name: '乙' }],
      occupiedCells: [],
    }} />)
    expect(screen.getByLabelText(copy.onlineRoster(2))).toBeTruthy()
    expect(screen.getByTitle('甲')).toBeTruthy()
    expect(screen.getByTitle('乙')).toBeTruthy()
  })

  it('does not render an empty roster placeholder', () => {
    const { container } = render(<BitablePresenceIndicator presence={{ people: [], occupiedCells: [] }} />)
    expect(container.innerHTML).toBe('')
  })
})
