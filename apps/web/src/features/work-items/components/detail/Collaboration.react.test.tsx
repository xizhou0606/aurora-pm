import { render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'
import { WORK_ITEMS_COPY, QUALITY_COPY } from '@/shared/copy'
import { Collaboration } from './Collaboration'
import { Collaboration as QualityCollaboration } from '@/features/quality-management/components/detail/Collaboration'

afterEach(() => window.history.replaceState({}, '', '/'))
it('opens task activity directly from a status notification', async () => {
 window.history.replaceState({}, '', '/#activity')
 render(<Collaboration taskId={7}
  activities={{id:'activities',title:'Activities',emptyLabel:'No activity',state:{status:'ready',data:[{id:'2',primary:'state changed'}],refresh:'idle'}}}
  comments={{id:'comments',title:'Comments',emptyLabel:'No comments',state:{status:'ready',data:[],refresh:'idle'}}}
  transitions={{status:'ready',data:[],refresh:'idle'}} commentState={{status:'idle'}} commentUsers={{status:'hidden'}} currentUserId={1}
  canComment={false} canAttach={false} canUploadBinary={false} onComment={vi.fn()} onUploadCommentImage={vi.fn()} onDownloadAttachment={vi.fn()} />)
 expect(await screen.findByText('state changed')).toBeTruthy()
 expect(screen.getByRole('tab',{name:WORK_ITEMS_COPY.detail.tabs.activity}).getAttribute('aria-selected')).toBe('true')
})
it('opens quality comments directly from a comment notification', async () => {
 window.history.replaceState({}, '', '/#comment-19')
 render(<QualityCollaboration bugId={7} currentUserId={1} activity={<p>Activity list</p>}
  comments={{status:'ready',rows:[{id:19,body:'A comment',userId:1,userName:'Teo',canDelete:false}],refresh:'idle',retry:vi.fn()}}
  users={{status:'ready',rows:[]}} commentMutation={{status:'idle',action:'comment'}}
  canViewComments={true} canCreateComment={false} canDeleteComment={()=>false} canUpload={false}
  onComment={vi.fn()} onDeleteComment={vi.fn()} onUploadImage={vi.fn()} />)
 expect(await screen.findByText('A comment')).toBeTruthy()
 expect(screen.getByRole('tab',{name:QUALITY_COPY.detail.comments}).getAttribute('aria-selected')).toBe('true')
})
