import { REQUIREMENT_DOCUMENTS_COPY as text } from '@/shared/copy/requirement-documents'
import { copy } from '@/shared/copy'
import { Button, EmptyState, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui'
import { useRequirementDocumentField } from '../hooks/useRequirementDocumentField'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'

export function RequirementDocumentField({ requirementId, projectId, projectName, value, url, id, error, disabled = false, onChange }: {
  requirementId?: number; projectId: number; projectName?: string; value?: RequirementDocumentChoice; url: string;
  id?: string; error?: string; disabled?: boolean; onChange: (value: RequirementDocumentChoice) => void
}) {
  const flow = useRequirementDocumentField(requirementId, value, url, true)
  const blocked = disabled || flow.legacyMultiple || Boolean(requirementId && flow.loading)
  return <section className="grid min-w-0 gap-1.5">
    <div className="flex flex-wrap items-center gap-3"><span className="text-sm font-medium">{text.title}</span>
      <div role="group" aria-label={text.sourceType} className="inline-flex rounded-md bg-muted p-0.5">{(['feishu', 'internal'] as const).map(type => <Button type="button" key={type} size="xs" variant="ghost" className={flow.choice.type === type ? 'bg-card shadow-sm' : 'text-muted-foreground'} aria-pressed={flow.choice.type === type} disabled={blocked} onClick={() => onChange(type === 'feishu' ? { type, url } : { type, document: flow.selected })}>{text[type]}</Button>)}</div>
    </div>
    {flow.choice.type === 'feishu'
      ? <Input id={id} type="url" aria-label={text.title} aria-invalid={Boolean(error) || undefined} aria-describedby={error && id ? `${id}-error` : undefined} placeholder={copy.aiPM.requirementPool.feishuPlaceholder} value={flow.choice.url} disabled={blocked} onChange={event => onChange({ type: 'feishu', url: event.target.value })} />
      : <Select items={flow.options.map(item => ({ value: item.resource_id, label: item.name }))} value={flow.selected?.resource_id ?? null} disabled={blocked || flow.loading || flow.error} onValueChange={id => onChange({ type: 'internal', document: flow.options.find(item => item.resource_id === id) ?? null })}>
        <SelectTrigger id={id} className="w-full min-w-0" aria-label={text.choose} aria-invalid={Boolean(error) || undefined} aria-describedby={error && id ? `${id}-error` : undefined}><SelectValue placeholder={text.choose} /></SelectTrigger>
        <SelectContent>{flow.options.map(item => <SelectItem key={item.resource_id} value={item.resource_id}>{item.name}</SelectItem>)}</SelectContent>
      </Select>}
    {flow.loading ? <p className="m-0 text-xs text-muted-foreground">{text.loading}</p> : null}
    {error ? <p id={id ? `${id}-error` : undefined} role="alert" className="m-0 text-xs text-destructive">{error}</p> : null}
    {flow.error ? <p role="alert" className="m-0 text-xs text-destructive">{text.loadFailed}<Button type="button" size="xs" variant="ghost" onClick={flow.reload}>{text.retry}</Button></p> : null}
    {flow.legacyMultiple ? <p role="alert" className="m-0 text-xs text-destructive">{text.legacyMultiple}</p> : null}
    {flow.choice.type === 'internal' && !flow.loading && !flow.error && !flow.options.length ? <EmptyState title={text.noCandidates} /> : null}
    {flow.selected ? <p className="m-0 text-xs leading-5 text-muted-foreground">{projectId ? text.scope(projectId, projectName) : text.chooseProjectScope}{flow.selected.private ? ` ${text.privateNotice}` : ''}</p> : null}
  </section>
}
