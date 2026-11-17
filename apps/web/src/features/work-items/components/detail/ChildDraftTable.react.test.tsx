import { fireEvent, render, screen } from '@testing-library/react'
import { useCallback, useState } from 'react'
import { describe, expect, it } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { createEmptyChildDraft } from '../../model/child'
import type { WorkItemChildDraft } from '../../model/types'
import { ChildDraftTable } from './ChildDraftTable'

const statuses = [
  { value: 'draft', label: '草稿' },
  { value: 'in_dev', label: '开发中' },
]
const readyUsers = { status: 'ready' as const, data: [] }

function Harness() {
  const [rows, setRows] = useState<WorkItemChildDraft[]>([
    createEmptyChildDraft('child-draft-1'),
  ])
  const update = useCallback((clientId: string, key: keyof Omit<WorkItemChildDraft, 'clientId'>, value: string | number | undefined) => {
    setRows((current) => current.map((row) => row.clientId === clientId
      ? { ...row, [key]: value }
      : row))
  }, [])
  const remove = useCallback(() => undefined, [])
  return (
    <ChildDraftTable
      rows={rows}
      errors={{}}
      disabled={false}
      users={readyUsers}
      statuses={statuses}
      onUpdate={update}
      onRemove={remove}
    />
  )
}

describe('ChildDraftTable', () => {
  it('keeps the active title editor mounted across controlled draft updates', () => {
    render(<Harness />)
    const input = screen.getByRole('textbox', {
      name: WORK_ITEMS_COPY.detail.childFieldLabel(WORK_ITEMS_COPY.detail.childName, 1),
    })
    input.focus()

    fireEvent.change(input, { target: { value: 'a' } })
    expect(document.activeElement).toBe(input)
    fireEvent.change(input, { target: { value: 'ab' } })
    expect(document.activeElement).toBe(input)
    expect((input as HTMLInputElement).value).toBe('ab')
  })

  it('places a readable status column before priority', () => {
    render(<Harness />)
    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)
    expect(headers.indexOf(WORK_ITEMS_COPY.detail.childStatus))
      .toBeLessThan(headers.indexOf(WORK_ITEMS_COPY.detail.childPriority))
  })
})
