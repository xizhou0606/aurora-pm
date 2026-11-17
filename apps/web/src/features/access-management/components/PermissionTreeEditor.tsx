import { PhCaretRight } from '@/shared/ui/icons'
import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { PM_ACTION_CODES, pmGrantingActions, type PMActionCode } from '@/generated/access-catalog'
import {
  Button,
  Checkbox,
  EmptyState,
  LoadingSkeleton,
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
  StatusPill,
} from '@/shared/ui'
import { actionCodes, type PermissionNode } from '../model'

const PM_ACTION_CODE_SET = new Set<string>(PM_ACTION_CODES)

function implyingSelectedCodes(code: string, selected: ReadonlySet<string>): string[] {
  if (!PM_ACTION_CODE_SET.has(code)) return []
  return pmGrantingActions(code as PMActionCode).filter((candidate) => (
    candidate !== code && selected.has(candidate)
  ))
}

type TreeNodeProps = {
  node: PermissionNode
  selected: ReadonlySet<string>
  disabled: boolean
  collapsed: ReadonlySet<string>
  onCollapse: (code: string) => void
  onToggle: (codes: string[], checked: boolean) => void
}

function ActionToolsPopover({ actionName, tools }: { actionName: string; tools: readonly string[] }) {
  const [open, setOpen] = useState(false)
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const cancelClose = useCallback(() => {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    closeTimer.current = null
  }, [])
  const scheduleClose = useCallback(() => {
    cancelClose()
    closeTimer.current = setTimeout(() => setOpen(false), 120)
  }, [cancelClose])

  useEffect(() => () => cancelClose(), [cancelClose])

  const hoverProps = {
    onPointerEnter: (event: PointerEvent) => {
      if (event.pointerType === 'mouse') {
        cancelClose()
        setOpen(true)
      }
    },
    onPointerLeave: (event: PointerEvent) => {
      if (event.pointerType === 'mouse') scheduleClose()
    },
  }

  if (tools.length === 0) return null

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        aria-label={COPY.permissionTree.openToolList(actionName, tools.length)}
        aria-expanded={open}
        render={<Button type="button" variant="outline" size="xs" className="shrink-0 rounded-full" />}
        onClick={(event) => event.stopPropagation()}
        onFocus={() => {
          cancelClose()
          setOpen(true)
        }}
        onKeyDown={(event) => event.stopPropagation()}
        {...hoverProps}
      >
        {COPY.permissionTree.toolCount(tools.length)}
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-[min(22rem,calc(100vw-1rem))] p-0"
        onClick={(event) => event.stopPropagation()}
        {...hoverProps}
      >
        <div className="border-b border-border px-3 py-2.5">
          <PopoverTitle>{COPY.permissionTree.toolListTitle}</PopoverTitle>
          <PopoverDescription>{COPY.permissionTree.toolListDescription(actionName)}</PopoverDescription>
        </div>
        <ul className="max-h-64 overflow-y-auto overscroll-contain p-2" aria-label={COPY.permissionTree.toolListLabel(actionName)}>
          {tools.map((tool) => (
            <li key={tool} className="rounded px-2 py-1.5 font-mono text-xs text-foreground break-all">
              {tool}
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}

function TreeNode({ node, selected, disabled, collapsed, onCollapse, onToggle }: TreeNodeProps) {
  const leaves = actionCodes([node])
  const selectedCount = leaves.filter((code) => (
    selected.has(code) || implyingSelectedCodes(code, selected).length > 0
  )).length
  const checked = leaves.length > 0 && selectedCount === leaves.length
  const indeterminate = selectedCount > 0 && !checked
  const implied = node.type === 'action' && !selected.has(node.code)
    && implyingSelectedCodes(node.code, selected).length > 0
  const levelLabel = COPY.permissionTree[`${node.type}Level`]
  const collapsible = node.children.length > 0
  const isCollapsed = collapsed.has(node.code)
  return (
    <li className="grid gap-2">
      <div className="flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/60">
        {collapsible ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={isCollapsed ? COPY.permissionTree.expand(node.name) : COPY.permissionTree.collapse(node.name)}
            aria-expanded={!isCollapsed}
            onClick={() => onCollapse(node.code)}
          >
            <PhCaretRight className={`size-4 transition-transform ${isCollapsed ? '' : 'rotate-90'}`} aria-hidden="true" />
          </Button>
        ) : <span className="size-6 shrink-0" aria-hidden="true" />}
        <Checkbox
          checked={checked}
          indeterminate={indeterminate}
          disabled={disabled || leaves.length === 0 || implied}
          onCheckedChange={(value) => onToggle(leaves, value === true)}
          aria-label={node.name}
        />
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <span className="min-w-0 truncate text-sm font-medium">{node.name}</span>
          {node.type === 'action' && node.objectPolicy?.kind === 'creator_override' ? (
            <StatusPill className="shrink-0">{COPY.permissionTree.creatorOverride}</StatusPill>
          ) : null}
          {implied ? (
            <StatusPill className="shrink-0">{COPY.permissionTree.impliedByBroaderAction}</StatusPill>
          ) : null}
          {node.type === 'action' ? <ActionToolsPopover actionName={node.name} tools={node.tools} /> : null}
        </div>
        {node.type === 'action' && !node.grantable ? (
          <StatusPill className="min-w-0">{COPY.permissionTree.notIndependentlyGrantable}</StatusPill>
        ) : null}
        <StatusPill className="min-w-0">{levelLabel}</StatusPill>
      </div>
      {collapsible && !isCollapsed ? (
        <ul className="ml-4 grid gap-1 border-l border-border pl-3">
          {node.children.map((child) => (
            <TreeNode key={child.code} node={child} selected={selected} disabled={disabled} collapsed={collapsed} onCollapse={onCollapse} onToggle={onToggle} />
          ))}
        </ul>
      ) : null}
    </li>
  )
}

function branchCodes(nodes: readonly PermissionNode[]): string[] {
  return nodes.flatMap((node) => node.children.length
    ? [node.code, ...branchCodes(node.children)]
    : [])
}

type PermissionTreeEditorProps = {
  subjectName: string
  nodes: PermissionNode[]
  selectedCodes: string[]
  loading: boolean
  loadError: boolean
  pending: boolean
  saveError: boolean
  disabled: boolean
  readOnlyReason?: string
  onRetry: () => void
  onChange: (codes: string[]) => void
  onSave: () => void
}

export function PermissionTreeEditor({
  subjectName,
  nodes,
  selectedCodes,
  loading,
  loadError,
  pending,
  saveError,
  disabled,
  readOnlyReason,
  onRetry,
  onChange,
  onSave,
}: PermissionTreeEditorProps) {
  const selected = useMemo(() => new Set(selectedCodes), [selectedCodes])
  const branches = useMemo(() => branchCodes(nodes), [nodes])
  const allGrantableCodes = useMemo(() => actionCodes(nodes), [nodes])
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set())
  const allCollapsed = branches.length > 0 && branches.every((code) => collapsed.has(code))
  const allSelected = allGrantableCodes.length > 0
    && allGrantableCodes.every((code) => selected.has(code))
  const toggle = (codes: string[], checked: boolean) => {
    const next = new Set(selected)
    codes.forEach((code) => checked ? next.add(code) : next.delete(code))
    onChange([...next].sort())
  }
  const toggleCollapsed = (code: string) => {
    setCollapsed((current) => {
      const next = new Set(current)
      if (next.has(code)) next.delete(code)
      else next.add(code)
      return next
    })
  }

  return (
    <section className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
      <header className="flex shrink-0 min-w-0 flex-wrap items-start justify-between gap-3 border-b border-border bg-card px-4 py-3">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold">{COPY.permissionTree.title(subjectName)}</h2>
          <p className="mt-1 text-xs text-muted-foreground">{COPY.permissionTree.description}</p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={disabled || loading || pending || allGrantableCodes.length === 0}
            onClick={() => onChange(allSelected ? [] : [...allGrantableCodes].sort())}
          >
            {allSelected ? COPY.permissionTree.clearAll : COPY.permissionTree.selectAll}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={loading || branches.length === 0}
            onClick={() => setCollapsed(allCollapsed ? new Set() : new Set(branches))}
          >
            {allCollapsed ? COPY.permissionTree.expandAll : COPY.permissionTree.collapseAll}
          </Button>
          <Button type="button" disabled={disabled || loading || pending} onClick={onSave}>
            {pending ? COPY.common.saving : COPY.permissionTree.save}
          </Button>
        </div>
        {saveError ? <p role="alert" className="basis-full text-sm text-destructive">{COPY.permissionTree.saveError}</p> : null}
        {readOnlyReason ? <p className="basis-full text-xs text-muted-foreground">{readOnlyReason}</p> : null}
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
        {loading ? <LoadingSkeleton rows={6} label={COPY.permissionTree.loading} /> : null}
        {loadError ? (
          <EmptyState
            title={COPY.permissionTree.errorTitle}
            action={<Button type="button" variant="outline" onClick={onRetry}>{COPY.common.retry}</Button>}
          />
        ) : null}
        {!loading && !loadError && nodes.length === 0 ? (
          <EmptyState title={COPY.permissionTree.emptyTitle} />
        ) : null}
        {!loading && !loadError && nodes.length > 0 ? (
          <ul className="grid gap-2" aria-label={COPY.permissionTree.description}>
            {nodes.map((node) => (
              <TreeNode key={node.code} node={node} selected={selected} disabled={disabled || pending} collapsed={collapsed} onCollapse={toggleCollapsed} onToggle={toggle} />
            ))}
          </ul>
        ) : null}
      </div>
    </section>
  )
}
