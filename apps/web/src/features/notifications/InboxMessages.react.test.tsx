import {act, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'bun:test'
import {NOTIFICATIONS_COPY as copy} from '@/shared/copy/notifications'
import {InboxMessages} from './InboxMessages'
const state=(()=>({read:vi.fn(), category:vi.fn(), failed:false, approval:false, pending:false}))()
vi.mock('./useMentions',()=>({
 useMentions:(_open:boolean,category:string)=>{state.category(category);return {data:state.pending?undefined:{pages:[{unread_count:5,unread_by_category:{mention:1,access:4,message:2},high_watermark:8,items:[{id:7,scene:state.approval?'project.access_requested':'quality_bug.comment_mentioned',resource_type:state.approval?'plan_unit':'quality_bug',resource_id:state.approval?'20/58':'6233',request_id:state.approval?8:undefined,request_status:'approved',comment_id:91,occurred_at:'2026-09-16T00:00:00Z'}]}]},isPending:state.pending,isError:false}},
 useNotificationRead:()=>({mutate:state.read,isError:state.failed,isPending:false}),
}))
describe('notification inbox',()=>{
 beforeEach(()=>{state.read.mockClear();state.failed=false;state.approval=false;state.pending=false})
 afterEach(()=>vi.useRealTimers())
 it('suppresses fast tab loading without exposing the previous category or read-all action',()=>{
  vi.useFakeTimers()
  const view=render(<InboxMessages category="mention" />)
  expect(screen.getByRole('link')).toBeTruthy()
  state.pending=true
  view.rerender(<InboxMessages category="message" />)
  expect(screen.queryByRole('link')).toBeNull()
  expect(screen.queryByText(copy.loading)).toBeNull()
  expect(screen.queryByText(copy.empty)).toBeNull()
  expect(screen.getByText(`${copy.unread} ${copy.unreadUnknown}`)).toBeTruthy()
  expect(screen.getByRole('button',{name:copy.readAll})).toHaveProperty('disabled',true)
  act(()=>vi.advanceTimersByTime(100))
  state.pending=false
  view.rerender(<InboxMessages category="message" />)
  act(()=>vi.advanceTimersByTime(200))
  expect(screen.queryByText(copy.loading)).toBeNull()
  expect(screen.getByText(`${copy.unread} 2`)).toBeTruthy()
 })
 it('shows loading for a slow category and resets the delay on another tab',()=>{
  vi.useFakeTimers()
  state.pending=true
  const view=render(<InboxMessages category="mention" />)
  act(()=>vi.advanceTimersByTime(200))
  expect(screen.getByRole('status').textContent).toBe(copy.loading)
  view.rerender(<InboxMessages category="message" />)
  expect(screen.queryByText(copy.loading)).toBeNull()
  act(()=>vi.advanceTimersByTime(200))
  expect(screen.getByRole('status').textContent).toBe(copy.loading)
 })
 it('requests only the selected category and uses the observed read boundary',()=>{
  render(<InboxMessages category="mention" />)
  expect(state.category).toHaveBeenCalledWith('mention')
  expect(screen.getByText(`${copy.unread} 1`)).toBeTruthy()
  expect(screen.getByRole('link').getAttribute('href')).toBe('/workspace/quality-management/bugs/6233#comment-91')
  fireEvent.click(screen.getByRole('button',{name:copy.readAll}))
  expect(state.read).toHaveBeenCalledWith({id:8,all:true,category:'mention'})
 })
 it('isolates message unread count and read-all scope',()=>{
  render(<InboxMessages category="message" />)
  expect(state.category).toHaveBeenCalledWith('message')
  expect(screen.getByText(`${copy.unread} 2`)).toBeTruthy()
  fireEvent.click(screen.getByRole('button',{name:copy.readAll}))
  expect(state.read).toHaveBeenCalledWith({id:8,all:true,category:'message'})
 })
 it('keeps approval items in the list and marks them read without navigation',()=>{
  state.approval=true
  const onOpen=vi.fn()
  render(<InboxMessages category="access" onOpen={onOpen} />)
  expect(screen.queryByRole('link')).toBeNull()
  expect(screen.getByText(`${copy.unread} 4`)).toBeTruthy()
  fireEvent.click(screen.getByRole('button',{name:/申请/}))
  expect(state.read).toHaveBeenCalledWith({id:7})
  expect(onOpen).not.toHaveBeenCalled()
 })
 it('does not close or navigate before marking a business message read succeeds',()=>{
  const onOpen=vi.fn()
  render(<InboxMessages onOpen={onOpen} />)
  fireEvent.click(screen.getByRole('link'))
  expect(state.read).toHaveBeenCalledWith({id:7},{onSuccess:expect.any(Function)})
  expect(onOpen).not.toHaveBeenCalled()
 })
 it('surfaces read failures without reporting success',()=>{
  state.failed=true
  render(<InboxMessages />)
  expect(screen.getByRole('alert').textContent).toBe(copy.updateError)
 })
})
