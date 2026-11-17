import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui'
import { DocumentRailToggle, documentRailMotion } from '../DocumentRailToggle'
import type { DocumentOutlineItem, DocumentOutlineTarget } from '../../model/outline'

export type DocumentOutlineContentProps = {
  items: readonly DocumentOutlineItem[]
  activeTargetId?: string
  onNavigate: (target: DocumentOutlineTarget) => void
  className?: string
}

export type DocumentOutlineRailProps = DocumentOutlineContentProps & {
  collapsed: boolean
  onCollapsedChange: (collapsed: boolean) => void
}

function OutlineItems({
  items,
  depth,
  activeTargetId,
  onNavigate,
}: Omit<DocumentOutlineContentProps, 'className'> & { depth: number }) {
  return (
    <ol className={cn('space-y-0.5', depth > 0 && 'mt-0.5')}>
      {items.map((item) => {
        const active = item.target.id === activeTargetId
        return (
          <li key={item.target.id}>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              aria-label={DOCUMENTS_COPY.outline.navigate(item.target.text)}
              aria-current={active ? 'location' : undefined}
              data-level={item.target.level}
              className={cn(
                'h-7 w-full justify-start overflow-hidden pr-2 text-left text-xs font-normal text-muted-foreground',
                'hover:bg-muted hover:text-foreground aria-current:bg-muted aria-current:font-medium aria-current:text-foreground',
                depth >= 5
                  ? 'pl-14'
                  : depth === 4
                    ? 'pl-12'
                    : depth === 3
                      ? 'pl-10'
                      : depth === 2
                        ? 'pl-8'
                        : depth === 1
                          ? 'pl-5'
                          : 'pl-2',
              )}
              onClick={() => onNavigate(item.target)}
            >
              <span className="truncate">{item.target.text}</span>
            </Button>
            {item.children.length > 0 ? (
              <OutlineItems
                items={item.children}
                depth={depth + 1}
                activeTargetId={activeTargetId}
                onNavigate={onNavigate}
              />
            ) : null}
          </li>
        )
      })}
    </ol>
  )
}

export function DocumentOutlineContent({
  items,
  activeTargetId,
  onNavigate,
  className,
}: DocumentOutlineContentProps) {
  if (items.length === 0) return null
  return (
    <nav
      aria-label={DOCUMENTS_COPY.outline.title}
      className={cn('min-h-0 overflow-y-auto overscroll-contain py-1', className)}
    >
      <OutlineItems
        items={items}
        depth={0}
        activeTargetId={activeTargetId}
        onNavigate={onNavigate}
      />
    </nav>
  )
}

export function DocumentOutlineRail({
  items,
  activeTargetId,
  onNavigate,
  collapsed,
  onCollapsedChange,
  className,
}: DocumentOutlineRailProps) {
  if (items.length === 0) return null
  return (
    <aside
      aria-label={DOCUMENTS_COPY.outline.title}
      className={cn(
        'hidden h-full min-h-0 shrink-0 flex-col overflow-hidden border-r border-border bg-background lg:flex',
        documentRailMotion,
        collapsed ? 'w-10 items-center' : 'w-56',
        className,
      )}
    >
      {collapsed ? (
        <div className="flex h-10 shrink-0 items-center justify-center">
          <DocumentRailToggle collapsed label={DOCUMENTS_COPY.outline.expand} onClick={() => onCollapsedChange(false)} />
        </div>
      ) : (
        <>
          <div className="flex h-10 shrink-0 items-center justify-between gap-2 px-2">
            <span className="truncate text-sm font-medium text-foreground">
              {DOCUMENTS_COPY.outline.title}
            </span>
            <DocumentRailToggle collapsed={false} label={DOCUMENTS_COPY.outline.collapse} onClick={() => onCollapsedChange(true)} />
          </div>
          <DocumentOutlineContent
            items={items}
            activeTargetId={activeTargetId}
            onNavigate={onNavigate}
            className="flex-1 px-1 pb-3"
          />
        </>
      )}
    </aside>
  )
}
