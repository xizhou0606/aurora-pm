import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { copy } from '@/shared/copy'
import { getPMRuntimeConfig } from '@/shared/config'
import type { ReleaseNoteEntry } from '@/shared/release-notes'
import {
  AlertBanner,
  BrandLockup,
  Button,
  EmptyState,
  LoadingSkeleton,
  StatusPill,
  ThemeAppearanceControl,
} from '@/shared/ui'
import { PhArrowsClockwise } from '@/shared/ui/icons'

import { fetchReleaseNotes } from '../api/release-notes'

function BulletList({ items }: { items: readonly string[] }) {
  return (
    <ul className="space-y-2">
      {items.map((item, index) => (
        <li
          key={`${index}:${item}`}
          className="flex gap-2.5 text-sm leading-6 text-foreground"
        >
          <span
            aria-hidden="true"
            className="mt-2 size-1.5 shrink-0 rounded-full bg-primary"
          />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  )
}

function ReleaseArticle({
  entry,
  isCurrent,
  isLatest,
}: {
  entry: ReleaseNoteEntry
  isCurrent: boolean
  isLatest: boolean
}) {
  return (
    <article className="py-8 first:pt-0 last:pb-0">
      <header className="flex flex-wrap items-center gap-2">
        <h2 className="m-0 font-mono text-base font-semibold tracking-tight text-foreground">
          {copy.app.shell.versionValue(entry.version)}
        </h2>
        {isCurrent ? (
          <StatusPill tone="accent">{copy.releaseNotes.currentVersion}</StatusPill>
        ) : isLatest ? (
          <StatusPill tone="info">{copy.releaseNotes.latest}</StatusPill>
        ) : null}
        {entry.date ? (
          <time
            dateTime={entry.date}
            className="ml-auto text-sm text-muted-foreground"
          >
            {entry.date}
          </time>
        ) : null}
      </header>
      {entry.summary.length ? (
        <div className="mt-4">
          <BulletList items={entry.summary} />
        </div>
      ) : null}
      {entry.sections.map((section) => (
        <section key={section.title} className="mt-5">
          <h3 className="m-0 text-sm font-medium text-muted-foreground">
            {section.title}
          </h3>
          <div className="mt-2">
            <BulletList items={section.items} />
          </div>
        </section>
      ))}
    </article>
  )
}

// Standalone public page (like the landing page it owns its full-page layout,
// outside the product shell) listing every release announcement served by the
// PM API, reachable at /release-notes.
export function ReleaseNotesPage() {
  const runtime = useMemo(() => getPMRuntimeConfig(), [])
  const releasesQuery = useQuery({
    queryKey: ['release-notes'],
    queryFn: ({ signal }) => fetchReleaseNotes(signal),
  })
  // 发版流程已服务端化:公告随部署自动入库,而构建版本号不再随发布 bump,
  // 因此"当前版本"以最新一期公告为准;接口为空/不可用时才退回构建版本号。
  const currentVersion = releasesQuery.data?.[0]?.version
    ?? runtime.appVersion.replace(/^v/i, '')

  return (
    <div
      data-slot="release-notes-page"
      className="min-h-dvh bg-background text-foreground"
    >
      <header className="sticky top-0 z-10 border-b border-border bg-background/90 backdrop-blur">
        <div className="mx-auto flex h-14 w-full max-w-3xl items-center justify-between gap-3 px-4 sm:px-6">
          <a
            href="/"
            aria-label={copy.releaseNotes.backToHome}
            className="rounded-md text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <BrandLockup name={runtime.shortName} />
          </a>
          <div className="flex items-center gap-2">
            <span className="font-mono text-xs text-muted-foreground">
              {copy.app.shell.versionValue(currentVersion)}
            </span>
            <ThemeAppearanceControl
              triggerClassName="text-muted-foreground hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>
        </div>
      </header>
      <main className="mx-auto w-full max-w-3xl px-4 pb-16 pt-10 sm:px-6 sm:pt-14">
        <h1 className="m-0 text-2xl font-semibold tracking-tight text-foreground">
          {copy.releaseNotes.title}
        </h1>
        <p className="mb-0 mt-2 text-sm text-muted-foreground">
          {copy.releaseNotes.subtitle}
        </p>
        {releasesQuery.isPending ? (
          <LoadingSkeleton className="mt-10" rows={6} />
        ) : releasesQuery.isError ? (
          <AlertBanner
            className="mt-10"
            tone="error"
            action={(
              <Button
                size="xs"
                variant="outline"
                onClick={() => void releasesQuery.refetch()}
              >
                <PhArrowsClockwise />
                {copy.ui.retry}
              </Button>
            )}
          >
            {copy.errors.genericDescription}
          </AlertBanner>
        ) : releasesQuery.data.length ? (
          <div className="mt-8 divide-y divide-border">
            {releasesQuery.data.map((entry, index) => (
              <ReleaseArticle
                key={entry.version}
                entry={entry}
                isCurrent={entry.version === currentVersion}
                isLatest={index === 0 && entry.version !== currentVersion}
              />
            ))}
          </div>
        ) : (
          <EmptyState className="mt-10" title={copy.releaseNotes.emptyTitle} />
        )}
      </main>
    </div>
  )
}
