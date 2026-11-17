import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'bun:test'

import type { WorkItem } from '../../model/types'
import { useWorkItemSelection } from './useSelection'

let root: Root | undefined
let container: HTMLDivElement | undefined
;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean })
  .IS_REACT_ACT_ENVIRONMENT = true

afterEach(async () => {
  if (root) await act(async () => root?.unmount())
  container?.remove()
  root = undefined
  container = undefined
})

const row: WorkItem = {
  id: 31,
  rowId: 'task:31',
  title: 'Selectable task',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: false,
    canComment: false,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: ['in_dev'],
  },
}

const secondRow: WorkItem = { ...row, id: 32, rowId: 'task:32', title: 'Second task' }

function Harness({ enabled }: { enabled: boolean }) {
  const selection = useWorkItemSelection([row, secondRow], [], enabled)
  return (
    <div>
      <output>{selection.selectedRows.length}</output>
      <button type="button" onClick={() => selection.toggleSelected(row)}>toggle</button>
      <button type="button" onClick={selection.selectAll}>all</button>
      <button type="button" onClick={selection.invertSelection}>invert</button>
    </div>
  )
}

describe('useWorkItemSelection', () => {
  it('clears table selection when the active layout no longer supports selection', async () => {
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)

    await act(async () => root?.render(<Harness enabled />))
    const toggle = [...container.querySelectorAll('button')].find((button) => button.textContent === 'toggle') as HTMLButtonElement
    const output = container.querySelector('output') as HTMLOutputElement
    await act(async () => toggle.click())
    expect(output.textContent).toBe('1')

    await act(async () => root?.render(<Harness enabled={false} />))
    expect(output.textContent).toBe('0')
  })

  it('selects every capable row and inverts against the current page', async () => {
    container = document.createElement('div')
    document.body.append(container)
    root = createRoot(container)

    await act(async () => root?.render(<Harness enabled />))
    const buttons = [...container.querySelectorAll('button')]
    const output = container.querySelector('output') as HTMLOutputElement
    const all = buttons.find((button) => button.textContent === 'all') as HTMLButtonElement
    const invert = buttons.find((button) => button.textContent === 'invert') as HTMLButtonElement
    const toggle = buttons.find((button) => button.textContent === 'toggle') as HTMLButtonElement

    await act(async () => all.click())
    expect(output.textContent).toBe('2')
    await act(async () => invert.click())
    expect(output.textContent).toBe('0')
    await act(async () => toggle.click())
    await act(async () => invert.click())
    expect(output.textContent).toBe('1')
  })
})
