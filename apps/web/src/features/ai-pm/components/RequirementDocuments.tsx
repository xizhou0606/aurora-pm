import { REQUIREMENT_DOCUMENTS_COPY as text } from '@/shared/copy/requirement-documents'
import { copy } from '@/shared/copy'
import { Button, EmptyState, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { useRequirementDocuments } from '../hooks/useRequirementDocuments'

export function RequirementDocuments({ workspaceId, requirementId, readOnly = false }: { workspaceId: number; requirementId: number; readOnly?: boolean }) {
  const flow = useRequirementDocuments(workspaceId, requirementId, readOnly)
  return <section aria-label={text.title} className="grid min-w-0 gap-3 rounded-lg border border-border bg-card p-3">
    <header className="flex flex-wrap items-center gap-3"><h3 className="m-0 text-sm font-semibold">{text.title}</h3><div role="group" aria-label={text.title} className="flex items-center gap-1 rounded-md bg-muted p-0.5">{(['feishu', 'internal'] as const).map(type => <Button key={type} size="sm" variant="ghost" className={`h-6 px-2 text-xs ${flow.sourceType === type ? 'bg-card shadow-sm' : 'text-muted-foreground'}`} aria-pressed={flow.sourceType === type} disabled={flow.mutation.isPending} onClick={() => flow.setSourceType(type)}>{text[type]}</Button>)}</div></header>
    {flow.list.isPending ? <p className="m-0 text-xs text-muted-foreground">{text.loading}</p> : flow.list.isError ? <p role="alert" className="m-0 text-xs text-destructive">{text.loadFailed}<Button size="sm" variant="ghost" onClick={flow.reload}>{text.retry}</Button></p> : <>
      {flow.sourceType === 'feishu' ? <div className="grid min-w-0 gap-2">
        <Input type="url" aria-label={text.title} placeholder={copy.aiPM.requirementPool.feishuPlaceholder} value={flow.url} disabled={!flow.editable || flow.mutation.isPending} onChange={event => flow.setURL(event.target.value)} />
        {flow.items.length > 0 ? <p className="m-0 text-xs text-muted-foreground">{flow.items.length > 1 ? text.legacyMultiple : text.replaceInternal}</p> : null}
        {flow.editable ? <div className="flex justify-end gap-2"><Button size="sm" variant="ghost" disabled={!flow.canValidate || flow.validation.isPending} onClick={flow.validate}>{copy.aiPM.requirementDetail.validate}</Button><Button size="sm" disabled={!flow.canSaveURL || flow.mutation.isPending} onClick={flow.saveURL}>{flow.mutation.isPending ? text.saving : text.save}</Button></div> : null}
        {flow.validation.isSuccess ? <p role="status" className="m-0 text-xs text-muted-foreground">{flow.validation.data.fetch_enabled ? copy.aiPM.requirementDetail.validEnabled : copy.aiPM.requirementDetail.validDisabled}</p> : null}
        {flow.validation.isError ? <p role="alert" className="m-0 text-xs text-destructive">{text.failed}</p> : null}
      </div> : <>
      {flow.items.length === 0 ? <EmptyState title={text.empty} /> : <ul className="m-0 grid list-none gap-2 p-0">{flow.items.map(item => <li key={item.resource_id} className="flex min-w-0 items-center gap-2">
        <a className="min-w-0 flex-1 truncate text-sm text-primary hover:underline" href={`/workspace/documents/${item.document_id}`} title={item.name}>{item.name}</a>
        {flow.editable && item.can_manage ? <Button size="sm" variant="ghost" disabled={flow.mutation.isPending} onClick={() => flow.detach(item)}>{text.detach}</Button> : null}
      </li>)}</ul>}
      {flow.editable && flow.items.length === 0 ? <div className="grid min-w-0 gap-2">
        <Select items={flow.options.map(item => ({ value: item.resource_id, label: item.name }))} value={flow.selected?.resource_id ?? null} disabled={flow.mutation.isPending || flow.candidates.isPending || flow.candidates.isError} onValueChange={value => flow.select(String(value ?? ''))}>
          <SelectTrigger className="w-full min-w-0" aria-label={text.choose}><SelectValue placeholder={text.choose}>{flow.selected?.name}</SelectValue></SelectTrigger>
          <SelectContent>{flow.options.map(item => <SelectItem key={item.resource_id} value={item.resource_id}>{item.name}</SelectItem>)}</SelectContent>
        </Select>
        {flow.candidates.isError ? <p role="alert" className="m-0 text-xs text-destructive">{text.loadFailed}<Button size="sm" variant="ghost" onClick={flow.reload}>{text.retry}</Button></p> : !flow.candidates.isPending && flow.options.length === 0 ? <p className="m-0 text-xs text-muted-foreground">{text.noCandidates}</p> : null}
        {flow.selected && flow.list.data ? <p className="m-0 text-xs leading-5 text-muted-foreground">{text.scope(flow.list.data.project_set_id, flow.projectSetName)}{flow.selected.private ? ` ${text.privateNotice}` : ''}</p> : null}
        {flow.list.data?.product_doc_url ? <p className="m-0 text-xs text-muted-foreground">{text.replaceFeishu}</p> : null}
        <Button size="sm" className="justify-self-end" disabled={!flow.selected || flow.mutation.isPending} onClick={flow.attach}>{flow.mutation.isPending ? text.saving : text.attach}</Button>
      </div> : null}
      </>}
    </>}
    {flow.mutation.isError ? <p role="alert" className="m-0 text-xs text-destructive">{text.failed}</p> : null}
  </section>
}
