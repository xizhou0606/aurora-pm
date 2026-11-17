import { useEffect, useMemo, useState } from 'react'
import { PhBriefcase, PhChartBar, PhFileText, PhTray, PhListChecks, PhSlidersHorizontal, PhShieldCheck, PhSparkle, PhFlowArrow, type IconComponent as IconComponent } from '@/shared/ui/icons'

import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import {
  Button,
  Checkbox,
  EmptyState,
  LoadingSkeleton,
  OperationalPage,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/shared/ui'
import type { HostNavigationItem } from '../api/api'
import { useHostNavigationMutation, useHostNavigationQuery } from '../api/api'

const menuIcons: Record<string, IconComponent> = {
  inbox: PhTray,
  briefcase: PhBriefcase,
  'list-checks': PhListChecks,
  chart: PhChartBar,
  'shield-check': PhShieldCheck,
  'file-text': PhFileText,
  workflow: PhFlowArrow,
  sparkles: PhSparkle,
}

function reorder(items: HostNavigationItem[], from: number, to: number) {
  if (to < 0 || to >= items.length) return items
  const next = [...items]
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)
  return next.map((item, index) => ({ ...item, sort_order: (index + 1) * 10 }))
}

export function HostNavigationPage() {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const query = useHostNavigationQuery(workspaceId)
  const mutation = useHostNavigationMutation(workspaceId)
  const [items, setItems] = useState<HostNavigationItem[]>([])

  useEffect(() => {
    if (query.data) setItems(query.data.items)
  }, [query.data])

  const changed = useMemo(() => {
    if (!query.data || items.length !== query.data.items.length) return false
    return items.some((item, index) => {
      const original = query.data?.items[index]
      return item.key !== original?.key ||
        item.visible !== original.visible ||
        item.sort_order !== original.sort_order
    })
  }, [items, query.data])

  const canManage = query.data?.can_manage === true
  const save = () => {
    mutation.mutate({
      items: items.map(({ key, visible, sort_order }) => ({
        key,
        visible,
        sort_order,
      })),
    })
  }

  return (
    <OperationalPage
      title={copy.hostNavigation.title}
      actions={query.isSuccess ? (
        <Button
          type="button"
          disabled={!canManage || !changed || mutation.isPending}
          aria-busy={mutation.isPending || undefined}
          onClick={save}
        >
          {mutation.isPending ? copy.hostNavigation.saving : copy.hostNavigation.save}
        </Button>
      ) : undefined}
    >
      <div className="mx-auto grid w-full max-w-5xl gap-3 p-3 sm:p-5">
        <div className="flex gap-3 rounded-lg border border-border bg-card p-4 text-sm text-muted-foreground">
          <PhSlidersHorizontal aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-primary" />
          <p className="m-0 leading-5">{copy.hostNavigation.guidance}</p>
        </div>

        {query.isPending ? (
          <div className="rounded-lg border border-border bg-card p-4">
            <LoadingSkeleton rows={6} label={copy.ui.loading.label} />
          </div>
        ) : null}

        {query.isError ? (
          <div className="rounded-lg border border-border bg-card p-4">
            <EmptyState
              role="alert"
              title={copy.errors.genericTitle}
              action={(
                <Button type="button" variant="outline" onClick={() => query.refetch()}>
                  {copy.ui.retry}
                </Button>
              )}
            />
          </div>
        ) : null}

        {query.isSuccess ? (
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{copy.hostNavigation.menu}</TableHead>
                  <TableHead>{copy.hostNavigation.path}</TableHead>
                  <TableHead className="w-32 text-center">{copy.hostNavigation.visible}</TableHead>
                  <TableHead className="w-32 text-right">{copy.hostNavigation.order}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item, index) => {
                  const Icon = menuIcons[item.icon] ?? PhSlidersHorizontal
                  return (
                    <TableRow key={item.key}>
                      <TableCell>
                        <span className="flex items-center gap-2 font-medium">
                          <Icon aria-hidden="true" className="size-4 text-muted-foreground" />
                          {item.label}
                        </span>
                      </TableCell>
                      <TableCell>
                        <code className="text-xs text-muted-foreground">{item.path}</code>
                      </TableCell>
                      <TableCell className="text-center">
                        <div className="inline-flex min-h-11 items-center justify-center px-2">
                          <Checkbox
                            checked={item.visible}
                            disabled={!canManage || mutation.isPending}
                            aria-label={copy.hostNavigation.visibleLabel(item.label)}
                            onCheckedChange={(visible) => {
                              setItems((current) => current.map((row) => (
                                row.key === item.key ? { ...row, visible } : row
                              )))
                              mutation.reset()
                            }}
                          />
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button
                            type="button"
                            variant="ghost"
                            size="xs"
                            aria-label={copy.hostNavigation.moveUpLabel(item.label)}
                            disabled={!canManage || index === 0 || mutation.isPending}
                            onClick={() => {
                              setItems((current) => reorder(current, index, index - 1))
                              mutation.reset()
                            }}
                          >
                            {copy.hostNavigation.moveUp}
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            size="xs"
                            aria-label={copy.hostNavigation.moveDownLabel(item.label)}
                            disabled={!canManage || index === items.length - 1 || mutation.isPending}
                            onClick={() => {
                              setItems((current) => reorder(current, index, index + 1))
                              mutation.reset()
                            }}
                          >
                            {copy.hostNavigation.moveDown}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            {!canManage ? (
              <p className="m-0 border-t border-border px-4 py-3 text-sm text-muted-foreground">
                {copy.hostNavigation.readOnly}
              </p>
            ) : null}
          </div>
        ) : null}

        {mutation.isSuccess ? (
          <p role="status" className="m-0 text-sm text-emerald-700 dark:text-emerald-400">
            {copy.hostNavigation.saved}
          </p>
        ) : null}
        {mutation.isError ? (
          <p role="alert" className="m-0 text-sm text-destructive">
            {copy.hostNavigation.saveFailed}
          </p>
        ) : null}
      </div>
    </OperationalPage>
  )
}
