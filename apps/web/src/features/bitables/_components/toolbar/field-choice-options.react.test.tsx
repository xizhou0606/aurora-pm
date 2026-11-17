import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'
import { useBitableFieldChoices } from '../../hooks/useBitableFieldChoices'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { FieldChoiceOptions } from './field-choice-options'
function Harness() {
  const flow = useBitableFieldChoices(false)
  return <FieldChoiceOptions choices={flow.choices} selected={flow.selected} multiple={false} disabled={false} onAdd={flow.add} onUpdate={flow.update} onRemove={flow.remove} onMove={flow.move} onSelect={flow.select} onClear={flow.clear} />
}
describe('structured field options', () => {
  it('adds, removes and keyboard reorders named inputs', () => {
    render(<Harness />)
    fireEvent.change(screen.getByRole('textbox', { name: text.choiceName(1) }), { target: { value: 'First' } })
    fireEvent.change(screen.getByRole('textbox', { name: text.choiceName(2) }), { target: { value: 'Second' } })
    fireEvent.keyDown(screen.getByRole('button', { name: text.moveChoice(1) }), { key: 'ArrowDown', altKey: true })
    expect((screen.getByRole('textbox', { name: text.choiceName(1) }) as HTMLInputElement).value).toBe('Second')
    fireEvent.click(screen.getByRole('button', { name: text.addChoice }))
    expect(screen.getByRole('textbox', { name: text.choiceName(3) })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: text.removeChoice(3) }))
    expect(screen.queryByRole('textbox', { name: text.choiceName(3) })).toBeNull()
  })
  it('searches named options for the default value', async () => {
    render(<Harness />)
    fireEvent.change(screen.getByRole('textbox', { name: text.choiceName(1) }), { target: { value: 'First' } })
    fireEvent.change(screen.getByRole('textbox', { name: text.choiceName(2) }), { target: { value: 'Second' } })
    fireEvent.click(screen.getByRole('button', { name: text.defaultValue }))
    fireEvent.change(await screen.findByRole('textbox', { name: text.choiceSearch }), { target: { value: 'Second' } })
    expect(screen.queryByRole('option', { name: 'First' })).toBeNull()
    fireEvent.click(screen.getByRole('option', { name: 'Second' }))
    expect(within(screen.getByRole('button', { name: text.defaultValue })).getByText('Second')).toBeTruthy()
  })
})
