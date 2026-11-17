import { BITABLE_TRANSFER_COPY as copy } from '@/shared/copy/bitables-transfer'
import { AlertBanner } from '@/shared/ui/feedback/alert-banner'
import type { Diagnostic } from '../../model/transfer/types'

export function TransferDiagnostics({ diagnostics }: { diagnostics: readonly Diagnostic[] }) {
  return <div className="space-y-2">{diagnostics.length > 30 && <p className="text-sm text-muted-foreground">{copy.diagnosticSummary(diagnostics.length)}</p>}{diagnostics.slice(0, 30).map((item, index) => <AlertBanner key={index} tone={item.severity}>
    {copy.location(item.row, item.column)} {copy.diagnostics[item.code]} {item.detail}
  </AlertBanner>)}</div>
}
