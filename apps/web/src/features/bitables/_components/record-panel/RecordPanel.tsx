import { useCallback, useId, useRef } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { AlertBanner, Button, CopyLinkButton, EmptyState, Label, LoadingSkeleton, ResponsivePanel, UserIdentity } from '@/shared/ui'
import { BitableInput as Input, BitableTextarea as Textarea } from '../controls'
import { displayCellValue } from '../../model/fields'
import type { RecordPanelProps, RecordPanelTab } from './types'

const tabs: readonly { id: RecordPanelTab; label: string }[] = [{ id: 'details', label: text.recordTitle }, { id: 'comments', label: text.comments }, { id: 'share', label: text.shareRecord }]
export function RecordPanel(props: RecordPanelProps) {
  const linkId = useId(), commentId = useId()
  const composer = useRef<HTMLTextAreaElement | null>(null)
  const focusComposer = useCallback((node: HTMLTextAreaElement | null) => { composer.current = node; if (node && props.open && props.tab === 'comments') node.focus({ preventScroll: true }) }, [props.open, props.tab])
  return <ResponsivePanel open={props.open} onOpenChange={open => { if (!open) props.onClose() }} onOpenChangeComplete={open => { if (open && props.tab === 'comments') composer.current?.focus({ preventScroll: true }) }} title={text.recordTitle} pending={props.phase !== 'idle'}>
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-1" role="tablist" aria-label={text.recordTitle}>
        {tabs.map(tab => <Button key={tab.id} role="tab" aria-selected={props.tab === tab.id} variant={props.tab === tab.id ? 'secondary' : 'ghost'} size="sm" onClick={() => props.onTabChange(tab.id)}>{tab.label}</Button>)}
      </div>
      {props.tab === 'share' ? <div className="grid min-w-0 gap-3">
        <Label htmlFor={linkId}>{text.recordLink}</Label>
        <Input id={linkId} readOnly value={props.href || props.path} onFocus={event => event.currentTarget.select()} />
        <p className="text-sm text-muted-foreground">{text.shareNotice}</p>
        {props.path ? <CopyLinkButton path={props.path} /> : null}
        {props.feedback ? <p role="status" className="text-sm text-muted-foreground">{props.feedback}</p> : null}
      </div> : props.tab === 'details' ? <>
        {props.state === 'loading' ? <LoadingSkeleton label={BITABLE_COPY.loading} /> : props.state === 'error' ? <><EmptyState title={text.recordUnavailable} /><AlertBanner tone="error">{props.error}</AlertBanner><Button variant="outline" onClick={props.onRetry}>{BITABLE_COPY.retry}</Button></> : <dl className="grid gap-4">
          {props.fields.map(field => <div key={field.id} className="grid gap-1 border-b border-border/60 pb-3"><dt className="text-xs text-muted-foreground">{field.name}</dt><dd className="whitespace-pre-wrap break-words text-sm">{props.record ? displayCellValue(field, props.record.values[field.id] ?? null) || text.emptyValue : text.emptyValue}</dd></div>)}
        </dl>}
      </> : <>
        {props.commentsState === 'loading' ? <LoadingSkeleton label={BITABLE_COPY.loading} /> : props.commentsState === 'error' ? <><AlertBanner tone="error">{props.commentsError}</AlertBanner><Button variant="outline" onClick={props.onRetry}>{BITABLE_COPY.retry}</Button></> : props.comments.length ? <div className="grid gap-4">{props.comments.map(comment => <article key={comment.id} className="grid gap-2 border-b border-border/60 pb-3">
          <div className="flex flex-wrap items-center justify-between gap-2"><UserIdentity name={comment.author.name} avatarUrl={comment.author.avatarUrl} size="sm" /><time dateTime={comment.createdAt} className="text-xs text-muted-foreground">{new Date(comment.createdAt).toLocaleString()}</time></div>
          <p className="whitespace-pre-wrap break-words text-sm">{comment.body}</p>
        </article>)}</div> : <EmptyState title={text.commentsEmpty} />}
        {props.total > 50 ? <div className="flex items-center justify-between gap-2"><Button variant="ghost" size="sm" disabled={props.offset === 0 || props.commentsState === 'loading'} onClick={() => props.onPageChange(props.offset - 50)}>{BITABLE_COPY.previous}</Button><span className="text-xs text-muted-foreground">{BITABLE_COPY.page(props.offset, props.comments.length, props.total)}</span><Button variant="ghost" size="sm" disabled={props.offset + 50 >= props.total || props.commentsState === 'loading'} onClick={() => props.onPageChange(props.offset + 50)}>{BITABLE_COPY.next}</Button></div> : null}
        {props.canComment ? <div className="grid gap-2"><Label htmlFor={commentId}>{text.newComment}</Label><Textarea ref={focusComposer} id={commentId} aria-label={text.newComment} value={props.draft} disabled={props.phase !== 'idle'} maxLength={4000} rows={3} onChange={event => props.onDraftChange(event.currentTarget.value)} /><div className="flex justify-end"><Button disabled={props.phase !== 'idle' || !props.draft.trim()} onClick={() => void props.onSend()}>{text.sendComment}</Button></div></div> : null}
        {props.submitError ? <AlertBanner tone="error">{props.submitError}</AlertBanner> : null}
        {props.phase === 'unknown' && props.canComment ? <Button variant="outline" onClick={() => void props.onRetrySend()}>{text.retryComment}</Button> : null}
      </>}
    </div>
  </ResponsivePanel>
}
