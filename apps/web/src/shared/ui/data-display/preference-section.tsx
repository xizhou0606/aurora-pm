import { useId } from 'react'

import { Button } from '../primitives/button'
import { PreferenceFieldList } from './preference-field-list'

export function PreferenceSection<TId extends string>({
  title,
  resetLabel,
  onReset,
  ...fields
}: Parameters<typeof PreferenceFieldList<TId>>[0] & {
  title: string
  resetLabel: string
  onReset: () => void
}) {
  const titleId = useId()
  return (
    <section aria-labelledby={titleId} className="min-w-0 rounded-lg border border-border bg-card p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 id={titleId} className="m-0 text-sm font-semibold">{title}</h3>
        <Button type="button" variant="link" size="xs" className="h-auto px-0" onClick={onReset}>
          {resetLabel}
        </Button>
      </div>
      <PreferenceFieldList {...fields} />
    </section>
  )
}
