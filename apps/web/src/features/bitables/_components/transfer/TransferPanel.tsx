import { useState } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_TRANSFER_COPY as copy } from '@/shared/copy/bitables-transfer'
import { AlertBanner } from '@/shared/ui/feedback/alert-banner'
import { EmptyState } from '@/shared/ui/feedback/empty-state'
import { ResponsivePanel } from '@/shared/ui/layout/responsive-panel'
import { Button } from '@/shared/ui/primitives/button'
import { BitableInput as Input } from '../controls'
import { Checkbox } from '@/shared/ui/primitives/checkbox'
import { Table, TableBody, TableCell, TableRow } from '@/shared/ui/data-display/table'
import { useTransfer, type TransferOptions } from '../../hooks/transfer/useTransfer'
import { isEditableField } from '../../model/fields'
import { TransferSelect } from './TransferSelect'
import { TransferDiagnostics } from './TransferDiagnostics'

export function TransferPanel(props: TransferOptions & { open: boolean; onOpenChange: (open: boolean) => void }) {
  const flow = useTransfer(props)
  const [scope, setScope] = useState<'view' | 'table'>('view'), [format, setFormat] = useState<'csv' | 'xlsx'>('csv')
  const pending = ['parsing', 'committing', 'exporting'].includes(flow.state.phase)
  const mappingFields = props.fields.filter(field => isEditableField(field) && field.type !== 'attachment')
  return <ResponsivePanel open={props.open} onOpenChange={open => { if (!pending) { if (!open) flow.reset(); props.onOpenChange(open) } }} title={copy.title} description={copy.hint} pending={pending}
    footer={<><Button variant="outline" disabled={pending} onClick={() => { flow.reset(); props.onOpenChange(false) }}>{BITABLE_COPY.close}</Button>
      {props.canImport && <Button disabled={!flow.canSubmit || pending} onClick={() => void flow.submit()}>{flow.state.phase === 'committing' ? copy.submitting : copy.confirm}</Button>}</>}>
    <div className="space-y-4">
      {props.canImport && <div className="space-y-2"><label className="block text-sm font-medium">{copy.file}<Input type="file" accept=".csv,.xlsx" disabled={pending} onChange={event => { const file = event.target.files?.[0]; if (file) void flow.chooseFile(file); event.target.value = '' }} /></label></div>}
      {flow.state.phase === 'parsing' && <AlertBanner tone="info" action={<Button variant="ghost" onClick={flow.reset}>{BITABLE_COPY.cancel}</Button>}>{copy.parsing}</AlertBanner>}
      {flow.state.phase === 'success' && <AlertBanner tone="success">{copy.success}</AlertBanner>}
      {flow.state.phase === 'exported' && <AlertBanner tone="success">{copy.exportSuccess}</AlertBanner>}
      {flow.state.phase === 'error' && (flow.state.diagnostic ? <TransferDiagnostics diagnostics={[flow.state.diagnostic]} /> : <AlertBanner tone="error">{copy.failure}</AlertBanner>)}
      {flow.workbook && flow.sheet && <section className="space-y-3" aria-label={copy.preview}>
        <TransferSelect label={copy.sheet} value={flow.sheet.id} options={flow.workbook.sheets.map(sheet => ({ value: sheet.id, label: sheet.name || copy.csv }))} onChange={flow.selectSheet} disabled={pending} />
        <label className="flex items-center gap-2 text-sm"><Checkbox checked={flow.header} onCheckedChange={value => flow.changeHeader(value === true)} disabled={pending} />{copy.header}</label>
        <div className="grid gap-2 sm:grid-cols-2">{flow.mapping.map((value, column) => <div key={column} className="space-y-1"><div className="truncate text-sm">{flow.header ? String(flow.sheet?.rows[0]?.[column] ?? copy.column(column + 1)) : copy.column(column + 1)}</div>
          <TransferSelect label={copy.mappingColumn(column + 1)} value={value || '__skip'} options={[{ value: '__skip', label: copy.skip }, ...mappingFields.map(field => ({ value: field.id, label: field.name }))]} disabled={pending} onChange={id => flow.mapColumn(column, id === '__skip' ? '' : id)} />
        </div>)}</div>
        <div className="text-sm">{copy.rows(Math.max(0, flow.sheet.rows.length - Number(flow.header)))}</div>
        <p className="text-xs text-muted-foreground">{copy.previewLimit}</p>
        {flow.sheet.rows.length ? <div className="max-h-48 overflow-auto rounded border"><Table className="text-xs"><TableBody>{flow.sheet.rows.slice(Number(flow.header), Number(flow.header) + 10).map((row, index) => <TableRow key={index}>{row.map((value, column) => <TableCell key={column} className="max-w-48 truncate border p-2">{String(value ?? '')}</TableCell>)}</TableRow>)}</TableBody></Table></div> : <EmptyState title={copy.empty} />}
        <TransferDiagnostics diagnostics={flow.diagnostics} />
      </section>}
      {props.canExport && <section className="space-y-2 border-t pt-4" aria-label={copy.export}>
        <TransferSelect label={copy.scope} value={scope} options={[{ value: 'view', label: copy.view }, { value: 'table', label: copy.table }]} onChange={value => setScope(value === 'table' ? 'table' : 'view')} disabled={pending} />
        <TransferSelect label={copy.format} value={format} options={[{ value: 'csv', label: copy.csv }, { value: 'xlsx', label: copy.xlsx }]} onChange={value => setFormat(value === 'xlsx' ? 'xlsx' : 'csv')} disabled={pending} />
        <Button variant="outline" disabled={pending} onClick={() => void flow.exportFile(scope, format)}>{flow.state.phase === 'exporting' ? copy.exporting : copy.export}</Button>
        <TransferDiagnostics diagnostics={flow.exportDiagnostics} />
      </section>}
    </div>
  </ResponsivePanel>
}
