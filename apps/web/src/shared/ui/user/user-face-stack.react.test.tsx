import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { UserFaceStack } from './user-face-stack'

const stackCopy = copy.ui.userFaceStack

describe('UserFaceStack', () => {
  it('keeps a single person as avatar plus name', () => {
    render(<UserFaceStack people={[{ name: '西洲', avatarUrl: '/xi.png' }]} />)

    expect(screen.getByText('西洲')).toBeTruthy()
    expect(screen.queryByRole('button', { name: stackCopy.openRoster('西洲') })).toBeNull()
  })

  it('stacks faces and opens the full roster on hover', async () => {
    render(
      <UserFaceStack
        people={[
          { id: 1, name: 'Bill', avatarUrl: '/bill.png' },
          { id: 2, name: '子轩' },
          { id: 3, name: '小龙' },
        ]}
      />,
    )

    const trigger = screen.getByRole('button', { name: stackCopy.openRoster('Bill、子轩、小龙') })
    expect(trigger.querySelectorAll('[data-slot="user-preview-avatar"]').length).toBe(2)
    expect(trigger.querySelector('[data-slot="user-face-stack-extra"]')?.textContent).toBe(stackCopy.extra(1))
    expect(screen.queryByRole('list')).toBeNull()

    fireEvent.pointerEnter(trigger)
    expect(await screen.findByRole('list')).toBeTruthy()
    expect(screen.getByText('子轩')).toBeTruthy()
    expect(screen.getByText('小龙')).toBeTruthy()
  })

  it('renders the empty placeholder without a roster', () => {
    const { container } = render(<UserFaceStack people={[]} emptyLabel="未指定" />)

    expect(container.querySelector('[data-slot="user-face-stack-empty"]')).toBeTruthy()
    expect(screen.getByText('未指定')).toBeTruthy()
  })

  it('shows the real overflow count above nine people', () => {
    const people = Array.from({ length: 12 }, (_, index) => ({ id: index + 1, name: `成员${index + 1}` }))
    render(<UserFaceStack people={people} interactive={false} />)

    expect(screen.getByText(stackCopy.extra(10))).toBeTruthy()
  })
})
