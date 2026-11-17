import { REPORTING_COPY } from '@/shared/copy'

export function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg border border-border bg-card px-3 py-2 shadow-xs">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-semibold text-foreground">
        {value}
        {' '}
        {REPORTING_COPY.metrics.unit}
      </p>
    </div>
  )
}
