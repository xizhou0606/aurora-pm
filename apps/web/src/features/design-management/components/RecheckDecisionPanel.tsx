import {
  PhArrowCounterClockwise as RotateCcw,
  PhClockCounterClockwise as History,
  PhLink as Link2,
  PhPlus as Plus,
} from '@/shared/ui/icons'

import { copy } from '../copy'
import {
  AlertBanner,
  Button,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
} from '@/shared/ui'
import type { HistoricalRecheckStatus, PendingRecheckMatch, RecheckVerification } from '../model'

const statuses: HistoricalRecheckStatus[] = ['still_present', 'resolved', 'uncertain', 'not_applicable']

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  matches: PendingRecheckMatch[]
  verifications: RecheckVerification[]
  canManage: boolean
  operationState: 'idle' | 'pending'
  message?: string
  onResolveMatch: (id: string, resolution: 'inherit' | 'new') => void
  onOverrideVerification: (id: string, status: HistoricalRecheckStatus | null) => void
}

export function RecheckDecisionPanel(props: Props) {
  return (
    <ResponsivePanel
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={copy.designManagement.detail.recheckDecisions}
      description={copy.designManagement.detail.recheckDecisionDescription}
      pending={props.operationState === 'pending'}
      footer={<Button variant="outline" onClick={() => props.onOpenChange(false)}>{copy.designManagement.common.close}</Button>}
    >
      <div className="grid gap-6">
        {props.message ? <AlertBanner tone="error">{props.message}</AlertBanner> : null}
        <section className="grid gap-3">
          <div className="flex items-center gap-2 border-b border-border pb-2"><Link2 className="size-4" /><h3 className="text-sm font-medium">{copy.designManagement.detail.pendingMatches}</h3><StatusPill tone={props.matches.length ? 'warning' : 'neutral'}>{String(props.matches.length)}</StatusPill></div>
          {props.matches.length === 0 ? <p className="text-xs text-muted-foreground">{copy.designManagement.detail.noPendingMatches}</p> : props.matches.map((match) => (
            <article key={match.id} className="grid gap-2 border-b border-border pb-3 last:border-b-0">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0"><p className="truncate text-sm font-medium">{match.candidate.title}</p><p className="text-xs text-muted-foreground">{copy.designManagement.differences.confidenceValue(match.confidence)}</p></div>
                <StatusPill tone="info">{copy.designManagement.recheck[match.discovery_source === 'history_recovered' ? 'still_present' : 'new']}</StatusPill>
              </div>
              <p className="text-xs text-foreground">{copy.designManagement.common.labeled(copy.designManagement.detail.suggestedLineage, match.suggested_lineage_title)}</p>
              <p className="text-xs text-muted-foreground">{match.reason}</p>
              {props.canManage && match.can_resolve ? <div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" disabled={props.operationState === 'pending'} onClick={() => props.onResolveMatch(match.id, 'inherit')}><Link2 />{copy.designManagement.detail.inheritLineage}</Button><Button size="sm" variant="outline" disabled={props.operationState === 'pending'} onClick={() => props.onResolveMatch(match.id, 'new')}><Plus />{copy.designManagement.detail.createLineage}</Button></div> : null}
            </article>
          ))}
        </section>

        <section className="grid gap-3">
          <div className="flex items-center gap-2 border-b border-border pb-2"><History className="size-4" /><h3 className="text-sm font-medium">{copy.designManagement.detail.historicalVerifications}</h3><StatusPill tone="neutral">{String(props.verifications.length)}</StatusPill></div>
          {props.verifications.length === 0 ? <p className="text-xs text-muted-foreground">{copy.designManagement.detail.noHistoricalVerifications}</p> : props.verifications.map((verification) => (
            <article key={verification.id} className="grid gap-2 border-b border-border pb-3 last:border-b-0">
              <div className="flex flex-wrap items-center justify-between gap-2"><p className="min-w-0 truncate text-sm font-medium">{verification.historical_title}</p><StatusPill tone={statusTone(verification.effective_status)}>{copy.designManagement.recheck[verification.effective_status]}</StatusPill></div>
              <p className="text-xs text-muted-foreground">{verification.reason}</p>
              <p className="text-xs text-foreground">{verification.evidence_summary}</p>
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"><span>{copy.designManagement.common.labeled(copy.designManagement.detail.aiDecision, copy.designManagement.recheck[verification.ai_status])}</span><span>{copy.designManagement.differences.confidenceValue(verification.confidence)}</span></div>
              {props.canManage && verification.can_override ? <div className="flex flex-wrap items-center gap-2"><Select items={statuses.map(status => ({ value: status, label: copy.designManagement.recheck[status] }))} value={verification.effective_status} onValueChange={(value) => value && props.onOverrideVerification(verification.id, value as HistoricalRecheckStatus)}><SelectTrigger className="w-40" aria-label={copy.designManagement.detail.effectiveDecision}><SelectValue /></SelectTrigger><SelectContent>{statuses.map((status) => <SelectItem key={status} value={status}>{copy.designManagement.recheck[status]}</SelectItem>)}</SelectContent></Select>{verification.manual_status_override ? <Button size="sm" variant="ghost" disabled={props.operationState === 'pending'} onClick={() => props.onOverrideVerification(verification.id, null)}><RotateCcw />{copy.designManagement.detail.restoreAIDecision}</Button> : null}</div> : null}
            </article>
          ))}
        </section>
      </div>
    </ResponsivePanel>
  )
}

function statusTone(status: HistoricalRecheckStatus) {
  if (status === 'resolved' || status === 'not_applicable') return 'success' as const
  if (status === 'still_present') return 'danger' as const
  return 'warning' as const
}
