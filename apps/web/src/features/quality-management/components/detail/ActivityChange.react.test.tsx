import { render, screen, fireEvent } from '@testing-library/react'
import { expect, it, vi } from 'bun:test'
import { QualityActivityChange } from './ActivityChange'
import { QUALITY_COPY } from '@/shared/copy'

it.each([['triage','warning'],['todo','success'],['in_progress','info'],['reopened','danger'],['review','accent'],['done','neutral']])('uses quality semantics for %s', (state,tone) => {
  const { container } = render(<QualityActivityChange activity={{id:1,action:'status_transition',fieldKey:'status',oldValue:'triage',newValue:state}} fallback="old summary" onPreview={()=>{}} />)
  expect(container.querySelectorAll('[data-slot="status-pill"]')[1].getAttribute('data-tone')).toBe(tone)
  expect(container.textContent).toContain(QUALITY_COPY.enums.status[state])
})
it('preserves severity labels and notes with attachment preview', () => {
  const preview=vi.fn()
  const {container}=render(<QualityActivityChange activity={{id:1,fieldKey:'severity_key',oldValue:'p2',newValue:'p0',summary:'备注：回归失败 https://cdn.example.test/proof.png'}} fallback="old summary" onPreview={preview} />)
  expect(screen.getByText(QUALITY_COPY.enums.severity.p0)).toBeTruthy()
  expect(container.querySelectorAll('[data-slot="status-pill"]')[1].getAttribute('data-tone')).toBe('danger')
  fireEvent.click(screen.getByRole('button',{name:QUALITY_COPY.upload.previewNamed('proof.png')}))
  expect(preview).toHaveBeenCalledTimes(1)
})
