import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { AlertBanner, Button, Popover, PopoverContent, PopoverDescription, PopoverTitle, PopoverTrigger } from '@/shared/ui'
import { PhDotsThree as More, PhDownloadSimple as Download } from '@/shared/ui/icons'
import type { DocumentExportWorkflow } from '../hooks/useDocumentExportWorkflow'

export function DocumentExportMenu({ workflow }: { workflow: DocumentExportWorkflow }) {
  const copy = DOCUMENTS_COPY.markdownExport
  const { state } = workflow
  const options = [
    { format: 'markdown', label: copy.markdown },
    { format: 'zip', label: copy.zip },
  ] as const
  const message = state.status === 'error' ? copy.errors[state.reason]
    : state.status === 'success' ? copy.success(state.fileName)
      : state.status === 'downloading' ? copy.downloading(state.completed)
        : state.status === 'preparing' ? copy.preparing : undefined
  return (
    <Popover>
      <PopoverTrigger render={<Button variant="ghost" size="sm" />}
        aria-label={copy.more} title={copy.more}>
        <More aria-hidden="true" />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-3">
        <PopoverTitle className="text-sm font-medium">{copy.title}</PopoverTitle>
        <PopoverDescription className="mt-1 mb-3 text-xs leading-5 text-muted-foreground">
          {copy.description}
        </PopoverDescription>
        <div className="grid gap-1">
          {options.map(({ format, label }) => (
            <Button key={format} type="button" variant="ghost" size="sm"
              className="justify-start" disabled={workflow.pending || Boolean(workflow.blocked)}
              onClick={() => { void workflow.run(format) }}>
              <Download aria-hidden="true" />{label}
            </Button>
          ))}
        </div>
        {workflow.blocked ? (
          <p role="status" className="mt-2 mb-0 text-xs text-muted-foreground">{copy.errors[workflow.blocked]}</p>
        ) : null}
        {message ? (
          <div className="mt-3 grid gap-2" role={state.status === 'error' ? 'alert' : 'status'}>
            <AlertBanner tone={state.status === 'error' ? 'error' : 'info'}>{message}</AlertBanner>
            {state.status === 'success' ? state.warnings.map((warning) => (
              <AlertBanner key={warning} tone="warning">{copy.warnings[warning]}</AlertBanner>
            )) : null}
            {!workflow.pending ? (
              <Button type="button" size="xs" variant="ghost" onClick={workflow.dismiss}>{copy.dismiss}</Button>
            ) : null}
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
