import { useMemo } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { PhDeviceMobile as Smartphone, PhPlus as Plus } from '@/shared/ui/icons'

import { copy } from '../copy'
import type { DesignReviewListSearch } from '@/shared/router'
import { useWorkspaceStore } from '@/shared/state'
import {
  AlertBanner,
  Button,
  DataTable,
  LoadingSkeleton,
  OperationalPage,
  Pagination,
  StatusPill,
} from '@/shared/ui'
import { ReviewFilters } from '../components/ReviewFilters'
import {
  reviewConclusionPresentation,
  reviewStatusPresentation,
  type DesignReviewSummary,
  type ReviewListInput,
} from '../model'
import { useDesignReviewAccess, useReviewCapabilities, useReviewList, useReviewRequirements } from '../hooks'

export function ReviewListPage() {
  const search = useSearch({ from: '/workspace/design-management' })
  const navigate = useNavigate({ from: '/workspace/design-management' })
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const capabilities = useReviewCapabilities(workspaceId)
  const access = useDesignReviewAccess(capabilities.data)
  const requirements = useReviewRequirements(workspaceId, access.canView)
  const input = useMemo<ReviewListInput>(() => ({
    keyword: search.keyword,
    project_set_id: search.project_set_id,
    plan_unit_id: search.plan_unit_id,
    requirement_id: search.requirement_id,
    status: search.status,
    conclusion: search.conclusion,
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }), [search])
  const query = useReviewList(workspaceId, input, access.canView)
  const columns = useMemo<ColumnDef<DesignReviewSummary>[]>(() => reviewColumns(), [])
  const rows = query.data?.items ?? []
  const filtered = Boolean(search.keyword || search.project_set_id || search.plan_unit_id || search.requirement_id || search.status || search.conclusion)
  const updateFilter = (key: string, value: string | number | undefined) => void navigate({
    search: (current: DesignReviewListSearch) => ({ ...current, [key]: value, page: undefined }), replace: true,
  })

  if (capabilities.isPending || access.status === 'loading') {
    return <OperationalPage title={copy.designManagement.title}><div className="p-5"><LoadingSkeleton rows={8} /></div></OperationalPage>
  }
  if (capabilities.isError || access.status === 'error') {
    return <OperationalPage title={copy.designManagement.title}><div className="p-5"><AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={() => { access.retry(); void capabilities.refetch() }}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.list.loadFailed}</AlertBanner></div></OperationalPage>
  }
  if (!access.canView) {
    return <OperationalPage title={copy.designManagement.title}><div className="p-5"><AlertBanner tone="warning">{copy.designManagement.list.viewForbidden}</AlertBanner></div></OperationalPage>
  }
  return (
    <OperationalPage
      title={copy.designManagement.title}

      refresh={query.isFetching && query.data ? 'refreshing' : query.isError && query.data ? 'stale_error' : 'idle'}
      actions={<>
        {access.canManageCollectors ? <Button variant="outline" onClick={() => void navigate({ to: '/workspace/design-management/mobile-collectors' })}><Smartphone />{copy.designManagement.list.collectors}</Button> : null}
        {access.canCreate ? <Button disabled={!access.canCreateNow} title={!access.canCreateNow ? copy.designManagement.newReview.createUnavailable : undefined} onClick={() => void navigate({ to: '/workspace/design-management/reviews/new' })}><Plus />{copy.designManagement.list.newReview}</Button> : null}
      </>}
      context={<ReviewFilters
        value={search}
        requirements={requirements.data ?? []}
        onChange={updateFilter}
        onReset={() => void navigate({ search: {}, replace: true })}
      />}
    >
      <div className="relative grid min-w-0 gap-4 p-3 sm:p-5">
        {capabilities.data?.unavailable_reasons.length ? <AlertBanner tone="warning">{capabilities.data.unavailable_reasons.join('；')}</AlertBanner> : null}
        {requirements.isError ? <AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={() => void requirements.refetch()}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.list.loadFailed}</AlertBanner> : null}
        <DataTable
          aria-label={copy.designManagement.list.table}
          columns={columns}
          state={query.isPending
            ? { status: 'loading', skeletonRows: 8 }
            : query.isError && !query.data
              ? { status: 'error', onRetry: () => void query.refetch() }
              : rows.length
                ? { status: 'ready', rows, refreshing: query.isFetching }
                : { status: 'empty', title: filtered ? copy.designManagement.list.filteredEmptyTitle : copy.designManagement.list.emptyTitle }}
          rowAction={{
            label: (review) => copy.designManagement.list.openReview(review.requirement_title),
            onActivate: (review) => void navigate({ to: '/workspace/design-management/reviews/$reviewId', params: { reviewId: review.id } }),
          }}
        />
        <Pagination
          page={input.page}
          pageSize={input.page_size}
          total={query.data?.total ?? 0}
          onPageChange={(page) => void navigate({ search: (current: DesignReviewListSearch) => ({ ...current, page: page > 1 ? page : undefined }) })}
          onPageSizeChange={(pageSize) => void navigate({ search: (current: DesignReviewListSearch) => ({ ...current, page: undefined, page_size: pageSize }) })}
        />
      </div>
    </OperationalPage>
  )
}

function reviewColumns(): ColumnDef<DesignReviewSummary>[] {
  const list = copy.designManagement.list
  return [
    { id: 'requirement', header: list.requirement, cell: ({ row }) => <div className="min-w-48"><strong className="block truncate text-sm">{row.original.requirement_title}</strong><span className="text-xs text-muted-foreground">{copy.designManagement.common.path(row.original.project_set_name, row.original.plan_unit_title)}</span></div> },
    { id: 'design', header: list.design, cell: ({ row }) => <div className="max-w-56"><span className="block truncate text-sm">{row.original.figma_node_name || row.original.figma_url}</span><span className="text-xs text-muted-foreground">{copy.designManagement.common.figma}</span></div> },
    { id: 'implementation', header: list.implementation, cell: ({ row }) => <div className="max-w-64"><span className="block truncate text-sm">{row.original.implementation_target.kind === 'web' ? row.original.implementation_target.original_url : row.original.implementation_target.app_name || row.original.implementation_target.app_id}</span><span className="text-xs text-muted-foreground">{row.original.implementation_target.kind === 'web' ? copy.designManagement.newReview.web : copy.designManagement.newReview.mobile}</span></div> },
    { accessorKey: 'round_number', header: list.round, cell: ({ getValue }) => <span className="text-sm">{Number(getValue())}</span> },
    { accessorKey: 'status', header: list.status, cell: ({ row }) => { const item = reviewStatusPresentation(row.original.status); return <StatusPill tone={item.tone}>{item.label}</StatusPill> } },
    { accessorKey: 'conclusion', header: list.conclusion, cell: ({ row }) => { const item = reviewConclusionPresentation(row.original.conclusion); return <StatusPill tone={item.tone}>{item.label}</StatusPill> } },
    { id: 'differences', header: list.differences, cell: ({ row }) => <span className="text-sm font-medium">{copy.designManagement.common.ratio(row.original.unresolved_difference_count, row.original.difference_count)}</span> },
    { id: 'creator', header: list.creator, cell: ({ row }) => <span className="text-sm">{row.original.created_by_name || `#${row.original.created_by_user_id}`}</span> },
    { id: 'created_at', header: list.createdAt, cell: ({ row }) => <span className="whitespace-nowrap text-xs text-muted-foreground">{new Date(row.original.created_at).toLocaleString('zh-CN')}</span> },
  ]
}
