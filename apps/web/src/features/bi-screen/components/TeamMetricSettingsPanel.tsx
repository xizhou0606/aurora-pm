import { useEffect, useMemo, useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import {
  Button,
  LoadingSkeleton,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'

import type { TeamMetricSettings, TeamMetricSettingsUpdate } from '../types'

type SaveState = 'idle' | 'pending' | 'success' | 'error'
type LoadState = 'loading' | 'error' | 'ready'

export type TeamMetricSettingsPanelProps = {
  open: boolean
  onOpenChange(open: boolean): void
  settings?: TeamMetricSettings
  loadState: LoadState
  onRetry(): void
  onSave(body: TeamMetricSettingsUpdate): void
  saveState: SaveState
  canUpdate?: boolean
}

function initialSelection(settings?: TeamMetricSettings) {
  if (!settings) return { templateId: undefined, nodeId: undefined }
  const template = settings.options.find((option) => (
    option.template_id === settings.requirement_workflow_template_id
  ))
  const selectedNode = template?.approval_nodes.find((node) => (
    node.node_id === settings.approval_node_id
  ))
  return {
    templateId: template?.template_id ?? settings.options[0]?.template_id,
    nodeId: settings.valid ? selectedNode?.node_id : undefined,
  }
}

export function TeamMetricSettingsPanel({
  open,
  onOpenChange,
  settings,
  loadState,
  onRetry,
  onSave,
  saveState,
  canUpdate = true,
}: TeamMetricSettingsPanelProps) {
  const copy = BI_SCREEN_COPY.team.metricSettings
  const pending = saveState === 'pending'
  const [templateId, setTemplateId] = useState<number>()
  const [nodeId, setNodeId] = useState<string>()

  useEffect(() => {
    if (!open) return
    const selection = initialSelection(settings)
    setTemplateId(selection.templateId)
    setNodeId(selection.nodeId)
  }, [open, settings])

  const template = useMemo(() => settings?.options.find((option) => (
    option.template_id === templateId
  )), [settings, templateId])
  const changed = templateId !== settings?.requirement_workflow_template_id
    || nodeId !== settings?.approval_node_id
  const canSave = Boolean(
    canUpdate && settings?.can_manage && templateId && nodeId && changed && !pending,
  )
  const status = !settings?.configured
    ? copy.unconfigured
    : settings.valid
      ? copy.configured
      : copy.invalid

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      title={copy.title}
      description={copy.description}
      pending={pending}
      footer={canUpdate && settings?.can_manage ? (
        <Button
          type="button"
          disabled={!canSave}
          onClick={() => {
            if (!settings || !templateId || !nodeId) return
            onSave({
              expected_version: settings.version,
              requirement_workflow_template_id: templateId,
              approval_node_id: nodeId,
            })
          }}
        >
          {pending ? copy.saving : copy.save}
        </Button>
      ) : undefined}
    >
      {loadState === 'loading' ? <LoadingSkeleton rows={4} label={copy.loading} /> : null}
      {loadState === 'error' ? (
        <div className="grid justify-items-start gap-3" role="alert">
          <p className="m-0 text-sm text-destructive">{copy.loadError}</p>
          <Button type="button" variant="outline" onClick={onRetry}>{copy.retry}</Button>
        </div>
      ) : null}
      {loadState === 'ready' && settings ? (
        <div className="grid gap-4">
          <p
            className={settings.valid
              ? 'm-0 rounded-lg border border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950/40 px-3 py-2 text-sm text-emerald-800 dark:text-emerald-300'
              : 'm-0 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 px-3 py-2 text-sm text-amber-800'}
            role="status"
          >
            {status}
          </p>
          {!settings.can_manage ? (
            <p className="m-0 text-sm text-muted-foreground">{copy.readOnly}</p>
          ) : null}
          {settings.options.length === 0 ? (
            <p className="m-0 text-sm text-muted-foreground">{copy.noOptions}</p>
          ) : (
            <div className="grid gap-4">
              <label className="grid gap-1.5 text-sm font-medium">
                {copy.templateLabel}
                <Select<string>
                  items={settings.options.map((option) => ({
                    value: String(option.template_id),
                    label: option.template_name,
                  }))}
                  value={templateId ? String(templateId) : null}
                  disabled={!canUpdate || !settings.can_manage || pending}
                  onValueChange={(value) => {
                    setTemplateId(value ? Number(value) : undefined)
                    setNodeId(undefined)
                  }}
                >
                  <SelectLabel>{copy.templateLabel}</SelectLabel>
                  <SelectTrigger aria-label={copy.templateLabel}>
                    <SelectValue placeholder={copy.templatePlaceholder} />
                  </SelectTrigger>
                  <SelectContent>
                    {settings.options.map((option) => (
                      <SelectItem key={option.template_id} value={String(option.template_id)}>
                        {option.template_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {template ? (
                  <span className="text-xs font-normal text-muted-foreground">
                    {copy.publishedVersion(template.published_version)}
                  </span>
                ) : null}
              </label>
              <label className="grid gap-1.5 text-sm font-medium">
                {copy.nodeLabel}
                <Select<string>
                  items={(template?.approval_nodes ?? []).map((node) => ({
                    value: node.node_id,
                    label: node.node_name,
                  }))}
                  value={nodeId ?? null}
                  disabled={!canUpdate || !settings.can_manage || pending || !template}
                  onValueChange={(value) => setNodeId(value || undefined)}
                >
                  <SelectLabel>{copy.nodeLabel}</SelectLabel>
                  <SelectTrigger aria-label={copy.nodeLabel}>
                    <SelectValue placeholder={copy.nodePlaceholder} />
                  </SelectTrigger>
                  <SelectContent>
                    {(template?.approval_nodes ?? []).map((node) => (
                      <SelectItem key={node.node_id} value={node.node_id}>{node.node_name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>
            </div>
          )}
          {saveState === 'success' ? (
            <p className="m-0 text-sm text-emerald-700 dark:text-emerald-400" role="status">{copy.saved}</p>
          ) : null}
          {saveState === 'error' ? (
            <div className="flex flex-wrap items-center justify-between gap-2" role="alert">
              <p className="m-0 text-sm text-destructive">{copy.saveFailed}</p>
              <Button type="button" variant="outline" size="sm" onClick={onRetry}>
                {copy.retry}
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
    </ResponsivePanel>
  )
}
