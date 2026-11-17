import { INBOX_COPY } from '@/shared/copy'
import { Button } from '@/shared/ui'

export function FeedIdentity({
  itemKey,
  title,
  scope,
  meta,
  onOpen,
}: {
  itemKey: string
  title: string
  scope: string
  meta: string
  onOpen: () => void
}) {
  return (
    <div className="grid min-w-0 gap-0.5">
      <Button
        type="button"
        variant="link"
        size="sm"
        className="h-auto min-w-0 justify-start p-0 text-left"
        aria-label={INBOX_COPY.feed.openDetail(title)}
        onClick={onOpen}
      >
        <span className="shrink-0 font-mono text-xs text-muted-foreground">{itemKey}</span>
        <span className="min-w-0 truncate">{title}</span>
      </Button>
      <span className="truncate text-xs text-muted-foreground">
        {scope}{INBOX_COPY.format.separator}{meta}
      </span>
    </div>
  )
}
