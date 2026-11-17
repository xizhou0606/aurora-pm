import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { downloadBlob } from '@/shared/lib/download-blob'
import type { CommandBatch, ExportSnapshot, Field } from '../../model/types'
import { initialMapping, parseTransferFile, prepareImport } from '../../model/transfer/import'
import { TransferError, type Diagnostic, type TransferWorkbook } from '../../model/transfer/types'

export type TransferOptions = {
  contextKey: string; fields: readonly Field[]; schemaRevision: number; canImport: boolean; canExport: boolean
  onSubmit: (batch: CommandBatch) => Promise<unknown>
  exportSnapshot: (scope: 'view' | 'table') => Promise<ExportSnapshot>
}
type State =
  | { phase: 'idle' | 'parsing' | 'committing' | 'exporting' | 'success' | 'exported' }
  | { phase: 'preview' }
  | { phase: 'error'; diagnostic?: Diagnostic }

export function useTransfer(options: TransferOptions) {
  const [state, setState] = useState<State>({ phase: 'idle' })
  const [workbook, setWorkbook] = useState<TransferWorkbook | null>(null)
  const [sheetId, setSheetId] = useState(''), [header, setHeader] = useState(true)
  const [mapping, setMapping] = useState<string[]>([]), [exportDiagnostics, setExportDiagnostics] = useState<Diagnostic[]>([])
  const current = useRef(options); current.current = options
  const generation = useRef(0), busy = useRef(false), controller = useRef<AbortController | null>(null)
  const identity = `${options.contextKey}:${options.schemaRevision}:${options.canImport}:${options.canExport}`
  const identityRef = useRef(identity); identityRef.current = identity
  const reset = useCallback(() => {
    generation.current++; controller.current?.abort(); busy.current = false
    setWorkbook(null); setSheetId(''); setMapping([]); setHeader(true); setExportDiagnostics([]); setState({ phase: 'idle' })
  }, [])
  useEffect(() => { reset(); return () => { generation.current++; controller.current?.abort() } }, [identity, reset])
  const sheet = workbook?.sheets.find(item => item.id === sheetId)
  const preview = useMemo(() => sheet ? prepareImport(sheet, options.fields, mapping, header) : null, [sheet, options.fields, mapping, header])
  const diagnostics = [...(workbook?.diagnostics ?? []), ...(preview?.diagnostics ?? [])]
  const canSubmit = options.canImport && state.phase === 'preview' && !!preview?.commands.length && !diagnostics.some(item => item.severity === 'error')

  const chooseFile = useCallback(async (file: File) => {
    if (!current.current.canImport || busy.current) return
    const owner = identityRef.current, run = ++generation.current
    controller.current?.abort(); controller.current = new AbortController(); busy.current = true
    setWorkbook(null); setExportDiagnostics([]); setState({ phase: 'parsing' })
    try {
      const parsed = await parseTransferFile(file, controller.current.signal)
      if (run !== generation.current || owner !== identityRef.current) return
      setWorkbook(parsed); setSheetId(parsed.sheets[0]?.id ?? ''); setHeader(true)
      setMapping(parsed.sheets[0] ? initialMapping(parsed.sheets[0], current.current.fields, true) : []); setState({ phase: 'preview' })
    } catch (error) {
      if (run === generation.current && owner === identityRef.current) setState({ phase: 'error', diagnostic: { code: error instanceof TransferError ? error.code : 'invalid_file', severity: 'error' } })
    } finally { if (run === generation.current) busy.current = false }
  }, [])
  const selectSheet = (id: string) => {
    if (busy.current) return
    const selected = workbook?.sheets.find(item => item.id === id)
    if (!selected) return
    setSheetId(id); setMapping(initialMapping(selected, options.fields, header))
  }
  const changeHeader = (value: boolean) => { if (!busy.current && sheet) { setHeader(value); setMapping(initialMapping(sheet, options.fields, value)) } }
  const mapColumn = (column: number, id: string) => { if (!busy.current) setMapping(previous => previous.map((value, index) => index === column ? id : value)) }
  const submit = async () => {
    if (!canSubmit || busy.current || !preview || !current.current.canImport || options.contextKey !== current.current.contextKey || options.schemaRevision !== current.current.schemaRevision) return
    const owner = identityRef.current, run = ++generation.current
    busy.current = true; setState({ phase: 'committing' })
    try {
      await current.current.onSubmit({ operationId: crypto.randomUUID(), expectedSchemaRevision: options.schemaRevision, atomic: true, commands: preview.commands })
      if (run === generation.current && owner === identityRef.current) { setWorkbook(null); setState({ phase: 'success' }) }
    } catch {
      // Unknown mutation outcome is not automatically retried. Keep the preview for diagnosis.
      if (run === generation.current && owner === identityRef.current) setState({ phase: 'error' })
    } finally { if (run === generation.current) busy.current = false }
  }
  const exportFile = async (scope: 'view' | 'table', format: 'csv' | 'xlsx') => {
    if (!current.current.canExport || busy.current) return
    const owner = identityRef.current, run = ++generation.current
    busy.current = true; setState({ phase: 'exporting' }); setExportDiagnostics([])
    try {
      const snapshot = await current.current.exportSnapshot(scope)
      const { createExport } = await import('../../model/transfer/export')
      if (run !== generation.current || owner !== identityRef.current || !current.current.canExport) return
      const result = createExport(snapshot, format)
      downloadBlob(result.blob, `table-${scope}.${format}`); setExportDiagnostics(result.diagnostics); setState({ phase: 'exported' })
    } catch (error) {
      if (run === generation.current && owner === identityRef.current) setState({ phase: 'error', diagnostic: error instanceof TransferError ? { code: error.code, severity: 'error' } : undefined })
    } finally { if (run === generation.current) busy.current = false }
  }
  return { state, workbook, sheet, header, mapping, diagnostics, exportDiagnostics, canSubmit, chooseFile, selectSheet, changeHeader, mapColumn, submit, exportFile, reset }
}
