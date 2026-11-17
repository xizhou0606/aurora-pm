import { useNavigate, useParams, useRouterState } from '@tanstack/react-router'
import { DocumentsPage } from '@/features/documents'
import { BitableCreateButton, BitableDirectory, BitableSurface, useBitablePicker } from '@/features/bitables'
import { RichTextBitableProvider } from '@/shared/ui'

export function DocumentTableWorkspace() {
  const params = useParams({ strict: false }) as { resourceRef?: string }
  const navigate = useNavigate()
  const search = useRouterState({ select: state => state.location.searchStr })
  const query = new URLSearchParams(search)
  const picker = useBitablePicker()
  const open = (id: string) => { void navigate({ to: id ? `/workspace/bitables/${encodeURIComponent(id)}` : '/workspace/documents' }) }
  return <RichTextBitableProvider value={{ pick: picker.pick, render: ({ reference, readOnly }) => <BitableSurface {...reference} readOnly={readOnly} embedded /> }}><DocumentsPage
    resourceEntries={location => <BitableDirectory {...location} selected={params.resourceRef} onSelect={open} />}
    renderCreate={createDocument => <BitableCreateButton createDocument={createDocument} onCreated={open} />}
    alternativeContent={params.resourceRef ? <main className="min-h-0 min-w-0 flex-1 overflow-hidden"><BitableSurface resourceId={params.resourceRef} tableId={query.get('table') || undefined} viewId={query.get('view') || undefined} recordId={query.get('record') || undefined}
      onSelectionChange={({ tableId, viewId }) => { void navigate({ to: `/workspace/bitables/${encodeURIComponent(params.resourceRef!)}`, search: { table: tableId, ...(viewId ? { view: viewId } : {}) } }) }} /></main> : undefined}
  />{picker.panel}</RichTextBitableProvider>
}
