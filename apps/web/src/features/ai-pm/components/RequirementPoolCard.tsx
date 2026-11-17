import { useEffect, useRef, useState, type ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { Button, Skeleton } from '@/shared/ui'
import { useRequirementCover } from '../api/api'
import { RequirementCoverFallback } from './requirement-cover-fallback'
import { requirementCurrentNode } from '../utils/requirement-pool-workflow'
import { RequirementRoleOwnersSummary } from '../utils/requirement-role-owners'
import {
  ExternalDeliverable,
  poolDesignItems,
} from './RequirementPoolTable/ExternalDeliverable'
import { TestCaseDeliverable } from './RequirementPoolTable/TestCaseDeliverable'
import type { RequirementTableRow } from './RequirementPoolTable/types'
import type { WorkflowAssignee } from '../model/types'

const poolCopy = copy.aiPM.requirementPool

function useVisibleOnce<Element extends HTMLElement>() {
  const ref = useRef<Element>(null)
  const [visible, setVisible] = useState(() => typeof IntersectionObserver === 'undefined')
  useEffect(() => {
    if (visible || !ref.current || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return
      setVisible(true)
      observer.disconnect()
    }, { rootMargin: '160px' })
    observer.observe(ref.current)
    return () => observer.disconnect()
  }, [visible])
  return { ref, visible }
}

function RequirementCover({
  requirement,
  workspaceId,
}: {
  requirement: RequirementTableRow
  workspaceId: number
}) {
  const { ref, visible } = useVisibleOnce<HTMLDivElement>()
  const coverState = requirement.deliverable_overview?.requirement_cover?.status
  const generation = requirement.deliverable_overview?.requirement_cover?.generation
  const cover = useRequirementCover(
    workspaceId,
    requirement.id,
    generation,
    visible && coverState === 'ready',
  )
  const [renderedCover, setRenderedCover] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setRenderedCover(null)
    if (!cover.data || typeof URL.createObjectURL !== 'function') {
      return
    }
    const nextURL = URL.createObjectURL(cover.data)
    setRenderedCover(nextURL)
    return () => {
      URL.revokeObjectURL(nextURL)
    }
  }, [cover.data])

  const source = renderedCover || ''
  useEffect(() => setFailed(false), [source])
  const isPending = coverState === 'pending'

  return (
    <div
      ref={ref}
      data-cover-state={source && !failed ? 'ready' : isPending ? 'pending' : cover.isError ? 'error' : 'fallback'}
      className="relative aspect-[2/1] overflow-hidden border-b border-border bg-muted"
    >
      <RequirementCoverFallback seed={requirement.id} />
      {isPending ? <Skeleton className="absolute inset-0 rounded-none bg-muted/80" /> : null}
      {source && !failed ? (
        <img
          src={source}
          alt=""
          loading="eager"
          decoding="async"
          referrerPolicy="no-referrer"
          className="absolute inset-0 size-full object-cover"
          onError={() => setFailed(true)}
        />
      ) : null}
    </div>
  )
}

function WorkflowNodeRibbon({ requirement }: { requirement: RequirementTableRow }) {
  const tag = requirementCurrentNode(requirement)
  return (
    <span
      className="pm-requirement-node-ribbon"
      data-tone={tag.tone}
      title={poolCopy.workflowNodeTag(tag.label)}
      aria-label={poolCopy.workflowNodeTag(tag.label)}
    >
      <span>{tag.label}</span>
    </span>
  )
}

export function RequirementCard<Row extends RequirementTableRow>({
  requirement,
  workspaceId,
  people,
  onOpen,
  renderOverflow,
}: {
  requirement: Row
  workspaceId: number
  people?: readonly WorkflowAssignee[]
  onOpen: (requirement: Row) => void
  renderOverflow?: (requirement: Row) => ReactNode
}) {
  return (
    <article className="group relative flex h-full min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-card transition-colors hover:border-primary/35">
      {/* eslint-disable shadcn/no-restyle -- 卡片整面覆盖按钮：蓝色焦点环与信息色卡片主题呼应，非按钮身份色 */}
      <Button
        type="button"
        variant="ghost"
        aria-label={poolCopy.openRow(requirement.title)}
        className="absolute inset-0 z-0 h-auto w-auto p-0 hover:bg-transparent active:translate-y-0 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-inset"
        onClick={() => onOpen(requirement)}
      />
      {/* eslint-enable shadcn/no-restyle */}
      <WorkflowNodeRibbon requirement={requirement} />
      {renderOverflow ? (
        <div className="pointer-events-none absolute top-2 left-2 z-30 group-hover:pointer-events-auto group-focus-within:pointer-events-auto has-aria-expanded:pointer-events-auto has-data-popup-open:pointer-events-auto [@media(hover:none)]:pointer-events-auto">
          {renderOverflow(requirement)}
        </div>
      ) : null}
      <div className="pointer-events-none relative z-10 flex min-h-0 flex-1 flex-col">
        <RequirementCover requirement={requirement} workspaceId={workspaceId} />
        <div className="flex flex-1 flex-col gap-2 p-3">
          <h2 className="m-0 line-clamp-2 min-h-10 break-words text-sm leading-5 font-semibold text-foreground" title={requirement.title}>
            {requirement.title}
          </h2>
          <RequirementRoleOwnersSummary requirement={requirement} people={people} />
          <div
            role="group"
            className="pointer-events-auto relative z-20 mt-auto flex min-w-0 items-center gap-2 pt-1"
            aria-label={poolCopy.deliverableOverview}
          >
            <ExternalDeliverable
              kind="document"
              label={poolCopy.deliverableDocument}
              url={requirement.deliverable_overview?.requirement_document_url || requirement.product_doc_url}
            />
            <ExternalDeliverable
              kind="design"
              label={poolCopy.deliverableDesign}
              items={poolDesignItems(requirement)}
              requirementId={requirement.id}
            />
            <TestCaseDeliverable requirement={requirement} />
          </div>
        </div>
      </div>
    </article>
  )
}

export function RequirementCardSkeleton() {
  return (
    <li className="overflow-hidden rounded-lg border border-border bg-card">
      <Skeleton className="aspect-[2/1] rounded-none" />
      <div className="grid gap-2 p-3">
        <Skeleton className="h-5 w-4/5" />
        <Skeleton className="h-5 w-3/5" />
        <Skeleton className="h-8 w-full" />
        <div className="flex gap-2 pt-1">
          <Skeleton className="size-7" />
          <Skeleton className="size-7" />
          <Skeleton className="size-7" />
        </div>
      </div>
    </li>
  )
}