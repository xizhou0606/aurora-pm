import { act, fireEvent, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, expect, it, spyOn } from 'bun:test'
import * as api from '@/shared/api'
import { useMentionPreview } from './useMentionPreview'
import type { ReactNode } from 'react'
afterEach(()=>{ document.querySelectorAll('[data-test-mention]').forEach(e=>e.remove()) })
function setup() {
 const client = new QueryClient({defaultOptions:{queries:{retry:false}}})
 const wrapper=({children}:{children:ReactNode})=><QueryClientProvider client={client}>{children}</QueryClientProvider>
 const hook=renderHook(()=>useMentionPreview(2),{wrapper})
 const element=document.createElement('span');element.dataset.commentMention='';element.dataset.userId='47';element.dataset.testMention='';element.textContent='@铁奥';document.body.append(element)
 return {...hook,element}
}
it('loads on hover, suppresses brief loading and caches the exact user', async()=>{
 const request=spyOn(api,'searchPlatformUsers').mockResolvedValue({items:[{id:47,username:'teo',email:'teo@example.com'}],total:1})
 const {result,element,unmount}=setup()
 try {
 expect(request).not.toHaveBeenCalled()
 fireEvent.pointerOver(element)
 await waitFor(()=>expect(result.current.user?.id).toBe(47))
 expect(request.mock.calls[0][0]).toEqual({ids:[47]})
 expect(result.current.loadingVisible).toBe(false)
 act(()=>result.current.close());fireEvent.pointerOver(element)
 expect(request).toHaveBeenCalledTimes(1)
 } finally {unmount();request.mockRestore()}
})
it('shows delayed loading and never substitutes another user for an unresolved ID', async()=>{
 let resolve!:(value:api.PlatformUserPage)=>void
 const request=spyOn(api,'searchPlatformUsers').mockImplementation(()=>new Promise(r=>{resolve=r}))
 const {result,element,unmount}=setup()
 try {
 fireEvent.pointerOver(element)
 expect(result.current.loadingVisible).toBe(false)
 await waitFor(()=>expect(result.current.loadingVisible).toBe(true))
 await act(async()=>resolve({items:[{id:99,username:'other'}],total:1}))
 await waitFor(()=>expect(result.current.loading).toBe(false))
 expect(result.current.user).toBeNull()
 } finally {unmount();request.mockRestore()}
})
