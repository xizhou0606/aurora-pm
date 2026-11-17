import { render, screen } from '@testing-library/react'
import { expect, it } from 'bun:test'
import { FormField } from './form-field'

it('associates label, hint and external validation without changing the control value', () => {
  render(<FormField id="title" label="Title" hint="Hint" error="Required" describedBy="external" showLabel={false}>
    {(control) => <input {...control} defaultValue="Draft" />}
  </FormField>)
  const input = screen.getByRole('textbox', { name: 'Title' })
  expect(input.getAttribute('aria-describedby')).toBe('external title-hint title-error')
  expect(input.getAttribute('aria-invalid')).toBe('true')
  expect(screen.getByRole('alert').id).toBe('title-error')
  expect((input as HTMLInputElement).value).toBe('Draft')
})
