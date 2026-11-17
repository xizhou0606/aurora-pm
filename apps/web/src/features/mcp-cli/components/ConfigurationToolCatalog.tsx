import { EmptyState, LoadingSkeleton, Button, ViewTabs } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { ToolTree } from './ToolTree'
import { type CollectionState, type ToolRiskCategory, toolRiskCategory } from '../model/model'
import type { ToolCatalogItem } from '../model/contract'
import { useState } from 'react'

export function ToolCatalog({
  state,
  selected,
  disabled,
  displayOnly = false,
  onSelectedChange,
}: {
  state: CollectionState<ToolCatalogItem>
  selected: readonly string[]
  disabled: boolean
  displayOnly?: boolean
  onSelectedChange: (toolNames: readonly string[], checked: boolean) => void
}) {
  const [riskCategory, setRiskCategory] = useState<ToolRiskCategory>('regular')
  if (state.status === 'denied') {
    return <EmptyState title={copy.app.pageAccess.deniedTitle} />
  }
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={5} label={copy.mcpCli.tools.loading} />
  }
  if (state.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={copy.mcpCli.tools.loadFailedTitle}
        action={<Button type="button" variant="outline" onClick={state.retry}>{copy.mcpCli.tools.retry}</Button>}
      />
    )
  }
  if (state.status === 'empty') {
    return <EmptyState
      title={displayOnly ? copy.mcpCli.tools.effectiveEmptyTitle : copy.mcpCli.tools.emptyTitle}
    />
  }
  const regularTools = state.items.filter((tool) => toolRiskCategory(tool) === 'regular')
  const highRiskTools = state.items.filter((tool) => toolRiskCategory(tool) === 'high_risk')
  const selectedNames = new Set(selected)
  const tabTools = riskCategory === 'high_risk' ? highRiskTools : regularTools
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="m-0 text-sm font-semibold text-foreground">
          {displayOnly ? copy.mcpCli.effectiveAccess.toolsTitle : copy.mcpCli.tools.selectionTitle}
        </h4>
        <ViewTabs
          value={riskCategory}
          label={copy.mcpCli.tools.selectionTitle}
          density="compact"
          tabs={[
            { id: 'regular', label: copy.mcpCli.tools.riskTabs.regularCount(regularTools.length) },
            { id: 'high_risk', label: copy.mcpCli.tools.riskTabs.highRiskCount(highRiskTools.length) },
          ]}
          onValueChange={(value) => {
            if (value === 'regular' || value === 'high_risk') setRiskCategory(value)
          }}
        />
      </div>
      <ToolTree
        key={riskCategory}
        tools={tabTools}
        selected={[...selectedNames]}
        disabled={disabled}
        displayOnly={displayOnly}
        onSelectedChange={onSelectedChange}
      />
      {riskCategory === 'high_risk' && highRiskTools.length > 0 && (
        <p className="text-xs text-muted-foreground">{copy.mcpCli.tools.riskTabs.highRiskDescription}</p>
      )}
    </div>
  )
}
