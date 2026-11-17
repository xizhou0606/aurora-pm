import { copy } from '@/shared/copy'

export type BulkFailure = { id: string | number; code: string; message: string }

export function BulkResult({
  succeeded,
  failures,
}: {
  succeeded: number
  failures: readonly BulkFailure[]
}) {
  return (
    <section
      aria-label={copy.ui.bulkResult.label}
      aria-live="polite"
      className="rounded-lg border border-border bg-card p-3 text-sm"
    >
      <p className="m-0 font-medium">{copy.ui.bulkResult.summary(succeeded, failures.length)}</p>
      {failures.length ? (
        <ul className="mt-2 mb-0 grid gap-1 pl-5 text-muted-foreground">
          {failures.map((failure) => (
            <li key={`${failure.id}:${failure.code}`}>
              {copy.ui.bulkResult.failure(failure.id, failure.code, failure.message)}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  )
}
