import { render, screen } from '@testing-library/react'
import { expect, it } from 'bun:test'
import { ActivitySummary, StatusChange, activityIcon } from './ActivitySummary'

it('renders canonical states, legacy aliases and unknown states with shared tones', () => {
  const { container } = render(<><StatusChange from="todo" to="doing" /><StatusChange from="pending_test" to="done" /><StatusChange from="cancelled" to="custom_state" /></>)
  expect([...container.querySelectorAll('[data-slot="status-pill"]')].map(e => e.getAttribute('data-tone'))).toEqual(['warning','info','warning','success','neutral','neutral'])
  expect(screen.getByText('custom_state')).toBeTruthy()
})
it('uses structured status values when historical summary is stale', () => {
  render(<ActivitySummary row={{ id:'1', primary:'stale', activity:{id:1,summary:'stale',fieldKey:'status',oldValue:'in_dev',newValue:'completed'} }} />)
  expect(screen.queryByText('stale')).toBeNull()
  expect(screen.getByText('已完成')).toBeTruthy()
})
it('uses distinct icons for creation, comments, dates, people and status', () => {
  const { container } = render(<>{[{action:'create'}, {action:'comment'}, {fieldKey:'due_at'}, {fieldKey:'assignee_id'}, {fieldKey:'status'}].map((activity,i)=><span key={i}>{activityIcon({id:String(i),primary:'',activity:{id:i,summary:'',...activity}})}</span>)}</>)
  expect(new Set([...container.querySelectorAll('svg')].map(e=>e.innerHTML)).size).toBe(5)
})
it('compacts schedule day boundaries but preserves full timestamps in the title', () => {
  render(<ActivitySummary row={{id:'date',primary:'old',activity:{id:4,summary:'',fieldKey:'due_at',oldValue:'2026-09-02T23:59:59',newValue:'2026-09-30T23:59:59'}}} />)
  expect(screen.getByText('2026/09/30').getAttribute('title')).toBe('2026/09/30 23:59:59')
})
