import { PageHeader } from '@/shared/ui'
import type { ReactNode } from 'react'

export function Section(props: { title: ReactNode; description: string; children: ReactNode; actions?: ReactNode; featured?: boolean }) {
  return (
    <section className={props.featured
      ? 'rounded-lg border border-primary/40 bg-primary/5 p-4 sm:p-5'
      : 'rounded-lg border border-border bg-card p-4 sm:p-5'}>
      <PageHeader variant="inline" headingLevel="h2" title={props.title} description={props.description} actions={props.actions} />
      <div className="mt-4">{props.children}</div>
    </section>
  )
}