import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, it, vi } from 'bun:test'
import { LazyFileUploader } from './lazy-file-uploader'
import { copy } from '@/shared/copy'

it('keeps upload failure inside the uploader and allows retry with one compact upload entry',async()=>{
 const upload=vi.fn().mockRejectedValueOnce(new Error('upload_failed')).mockResolvedValueOnce(true)
 const {container}=render(<LazyFileUploader label="File" compact={{dropLabel:'Upload'}} onUpload={upload} />)
 expect(screen.getAllByRole('button',{name:'Upload'})).toHaveLength(1)
 const input=container.querySelector('input')!
 fireEvent.change(input,{target:{files:[new File(['test'],'test.txt')]}})
 expect((await screen.findByRole('alert')).textContent).toBe(copy.ui.fileUploader.uploadFailed)
 fireEvent.change(input,{target:{files:[new File(['test'],'test.txt')]}})
 await waitFor(()=>expect(upload).toHaveBeenCalledTimes(2))
 await waitFor(()=>expect(screen.queryByRole('alert')).toBeNull())
})
