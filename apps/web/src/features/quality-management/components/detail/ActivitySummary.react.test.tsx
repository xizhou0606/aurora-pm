import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'bun:test'
import { QUALITY_COPY } from '@/shared/copy'
import { qualityActivitySummary } from '../../model/presenters'
import { ActivitySummary, activityActionLabel } from './ActivitySummary'

it.each([
 '<p>保留评论 @铁奥</p><p><img src="https://cdn.example.test/proof.png"></p>',
 JSON.stringify({type:'doc',content:[{type:'paragraph',content:[{type:'text',text:'保留评论 @铁奥'}]},{type:'image',attrs:{src:'https://cdn.example.test/proof.png'}}]}),
])('preserves rich comment content and image preview after adding the action title', (summary) => {
 const onPreview = vi.fn()
 const value = qualityActivitySummary({action:'comment',summary},activityActionLabel('comment'))
 render(<ActivitySummary value={value} onPreview={onPreview} />)
 expect(screen.getByText(/添加了评论/).textContent).toContain('保留评论 @铁奥')
 const preview = screen.getByRole('button',{name:QUALITY_COPY.upload.previewNamed('proof.png')})
 fireEvent.click(preview)
 expect(onPreview).toHaveBeenCalledWith('https://cdn.example.test/proof.png',preview)
 expect(screen.queryByText(/"type":"doc"/)).toBeNull()
})
