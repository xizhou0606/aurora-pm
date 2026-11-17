import { PhCaretDown, PhCaretRight } from '@/shared/ui/icons'
import { useEffect, useMemo, useState, type KeyboardEvent } from 'react'

import { copy } from '@/shared/copy'
import { Button, Checkbox, StatusPill } from '@/shared/ui'

import type { ToolCatalogItem } from '../model/contract'
import {
  buildToolTree,
  configurableToolNamesInTree,
  isToolConfigurable,
  toolNamesInTree,
  toolTreeSelection,
  type ToolTreeNode,
} from '../model/model'

function riskLabel(risk: ToolCatalogItem['risk']): string {
  if (risk === 'high_impact') return copy.mcpCli.risk.highImpact
  return risk === 'mutation' ? copy.mcpCli.risk.mutation : copy.mcpCli.risk.read
}

function riskTone(risk: ToolCatalogItem['risk']): 'neutral' | 'warning' | 'danger' {
  if (risk === 'high_impact') return 'danger'
  return risk === 'mutation' ? 'warning' : 'neutral'
}

function groupDOMId(id: string): string {
  return `pm-tool-group-${id.replace(/[^a-zA-Z0-9_-]/g, '-')}`
}

function collectGroupIds(nodes: readonly ToolTreeNode[], target = new Set<string>()): Set<string> {
  nodes.forEach((node) => {
    if (node.kind !== 'group') return
    target.add(node.id)
    collectGroupIds(node.children, target)
  })
  return target
}

function selectedToolCount(node: ToolTreeNode, selected: ReadonlySet<string>): number {
  if (node.kind === 'tool') return selected.has(node.tool.name) ? 1 : 0
  return node.children.reduce((count, child) => count + selectedToolCount(child, selected), 0)
}

function ToolLeaf({
  tool,
  selected,
  disabled,
  displayOnly,
  onSelectedChange,
}: {
  tool: ToolCatalogItem
  selected: boolean
  disabled: boolean
  displayOnly: boolean
  onSelectedChange: (toolNames: readonly string[], checked: boolean) => void
}) {
  const configurable = isToolConfigurable(tool)
  const controlDisabled = disabled || !configurable
  return (
    <li
      role="treeitem"
      aria-disabled={controlDisabled || undefined}
      className="list-none border-t border-border/70 first:border-t-0"
    >
      <div className={displayOnly
        ? 'min-w-0 px-3 py-2.5'
        : 'grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-2.5 px-3 py-2.5'}>
        {!displayOnly ? (
          <Checkbox
            className="mt-0.5"
            aria-label={copy.mcpCli.tools.selectTool(tool.name)}
            checked={selected}
            disabled={controlDisabled}
            onCheckedChange={(checked) => onSelectedChange([tool.name], checked === true)}
          />
        ) : null}
        <div className={configurable ? 'min-w-0' : 'min-w-0 opacity-60'}>
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <code className="min-w-0 break-all text-xs font-semibold text-foreground">{tool.name}</code>
            <StatusPill tone={riskTone(tool.risk)}>{riskLabel(tool.risk)}</StatusPill>
            <StatusPill tone={tool.lifecycle === 'active' ? 'success' : tool.lifecycle === 'deprecated' ? 'warning' : 'neutral'}>
              {tool.lifecycle === 'active'
                ? copy.mcpCli.lifecycle.active
                : tool.lifecycle === 'deprecated'
                  ? copy.mcpCli.lifecycle.deprecated
                  : copy.mcpCli.lifecycle.draft}
            </StatusPill>
          </div>
          <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{tool.summary}</p>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>{copy.mcpCli.tools.version(tool.version)}</span>
            <span>{copy.mcpCli.tools.scope(tool.grantScope)}</span>
            {!configurable ? <span>{copy.mcpCli.tools.unavailable}</span> : null}
          </div>
        </div>
      </div>
    </li>
  )
}

function GroupNode({
  node,
  collapsedIds,
  selected,
  disabled,
  displayOnly,
  onToggle,
  onSelectedChange,
}: {
  node: Extract<ToolTreeNode, { kind: 'group' }>
  collapsedIds: ReadonlySet<string>
  selected: ReadonlySet<string>
  disabled: boolean
  displayOnly: boolean
  onToggle: (id: string, expanded: boolean) => void
  onSelectedChange: (toolNames: readonly string[], checked: boolean) => void
}) {
  const expanded = !collapsedIds.has(node.id)
  const regionId = groupDOMId(node.id)
  const groupToolNames = configurableToolNamesInTree(node)
  const totalToolCount = toolNamesInTree(node).length
  const selection = toolTreeSelection(node, selected)
  const selectedCount = selectedToolCount(node, selected)
  const toggle = () => onToggle(node.id, !expanded)
  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'ArrowRight' && !expanded) {
      event.preventDefault()
      onToggle(node.id, true)
    } else if (event.key === 'ArrowLeft' && expanded) {
      event.preventDefault()
      onToggle(node.id, false)
    }
  }

  return (
    <li role="treeitem" aria-expanded={expanded} className="list-none">
      <div className={displayOnly
        ? 'min-h-9 rounded-md px-2 py-1 hover:bg-muted focus-within:bg-muted'
        : 'grid min-h-9 grid-cols-[auto_minmax(0,1fr)] items-center gap-1 rounded-md px-2 py-1 hover:bg-muted focus-within:bg-muted'}>
        {!displayOnly ? (
          <Checkbox
            aria-label={selection === true
              ? copy.mcpCli.tools.disableGroup(node.label)
              : copy.mcpCli.tools.enableGroup(node.label)}
            aria-controls={regionId}
            checked={selection === true}
            indeterminate={selection === 'mixed'}
            disabled={disabled || groupToolNames.length === 0}
            onCheckedChange={() => onSelectedChange(groupToolNames, selection !== true)}
          />
        ) : null}
        <Button
          type="button"
          variant="ghost"
          aria-expanded={expanded}
          aria-controls={regionId}
          aria-label={expanded
            ? copy.mcpCli.tools.collapseGroup(node.label)
            : copy.mcpCli.tools.expandGroup(node.label)}
          className="h-auto min-h-8 w-full min-w-0 justify-start gap-2 px-1.5 text-left"
          onClick={toggle}
          onKeyDown={handleKeyDown}
        >
          {expanded
            ? <PhCaretDown aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
            : <PhCaretRight aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />}
          <code className="min-w-0 flex-1 truncate text-xs font-semibold">{node.label}</code>
          <span className="shrink-0 text-xs font-normal text-muted-foreground">
            {displayOnly
              ? copy.mcpCli.tools.availableGroupCount(totalToolCount)
              : copy.mcpCli.tools.groupCount(totalToolCount, groupToolNames.length, selectedCount)}
          </span>
        </Button>
      </div>
      {expanded ? (
        <ul
          id={regionId}
          role="group"
          className="m-0 ml-4 list-none border-l border-border/80 p-0 pl-2"
        >
          {node.children.map((child) => child.kind === 'group'
            ? (
                <GroupNode
                  key={child.id}
                  node={child}
                  collapsedIds={collapsedIds}
                  selected={selected}
                  disabled={disabled}
                  displayOnly={displayOnly}
                  onToggle={onToggle}
                  onSelectedChange={onSelectedChange}
                />
              )
            : (
                <ToolLeaf
                  key={child.id}
                  tool={child.tool}
                  selected={selected.has(child.tool.name)}
                  disabled={disabled}
                  displayOnly={displayOnly}
                  onSelectedChange={onSelectedChange}
                />
              ))}
        </ul>
      ) : null}
    </li>
  )
}

export function ToolTree({
  tools,
  selected,
  disabled,
  displayOnly = false,
  onSelectedChange,
}: {
  tools: readonly ToolCatalogItem[]
  selected: readonly string[]
  disabled: boolean
  displayOnly?: boolean
  onSelectedChange: (toolNames: readonly string[], checked: boolean) => void
}) {
  const tree = useMemo(() => buildToolTree(tools), [tools])
  const selectedNames = useMemo(() => new Set(selected), [selected])
  const groupIds = useMemo(() => collectGroupIds(tree), [tree])
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(new Set())

  useEffect(() => {
    setCollapsedIds((current) => {
      const next = new Set([...current].filter((id) => groupIds.has(id)))
      return next.size === current.size ? current : next
    })
  }, [groupIds])

  const toggle = (id: string, expanded: boolean) => {
    setCollapsedIds((current) => {
      const next = new Set(current)
      if (expanded) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div className="space-y-2">
      <div className="flex justify-end">
        <Button type="button" variant="ghost" size="xs" disabled={groupIds.size === 0}
          onClick={() => setCollapsedIds(collapsedIds.size === groupIds.size ? new Set() : new Set(groupIds))}>
          {collapsedIds.size === groupIds.size ? copy.mcpCli.tools.expandAll : copy.mcpCli.tools.collapseAll}
        </Button>
      </div>
    <ul
      role="tree"
      aria-label={displayOnly ? copy.mcpCli.tools.effectiveTreeLabel : copy.mcpCli.tools.treeLabel}
      className="m-0 max-h-[34rem] list-none overflow-y-auto overscroll-contain rounded-lg border border-border bg-background p-1"
    >
      {tree.map((node) => node.kind === 'group'
        ? (
            <GroupNode
              key={node.id}
              node={node}
              collapsedIds={collapsedIds}
              selected={selectedNames}
              disabled={disabled}
              displayOnly={displayOnly}
              onToggle={toggle}
              onSelectedChange={onSelectedChange}
            />
          )
        : (
            <ToolLeaf
              key={node.id}
              tool={node.tool}
              selected={selectedNames.has(node.tool.name)}
              disabled={disabled}
              displayOnly={displayOnly}
              onSelectedChange={onSelectedChange}
            />
          ))}
    </ul>
    </div>
  )
}
