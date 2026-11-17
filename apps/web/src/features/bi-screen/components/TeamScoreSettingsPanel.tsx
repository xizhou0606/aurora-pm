import { useEffect, useId, useMemo, useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { Button, Checkbox, Input, ResponsivePanel, ViewTabs } from '@/shared/ui'

import type { TeamMetricRow, TeamScoreMetric, TeamScoreProfile, TeamScoreProfileUpdate } from '../types'

type DraftMetric = TeamScoreMetric & { weightText: string; sampleText: string }

const ROLE_ORDER: TeamMetricRow['role_key'][] = ['product', 'testing', 'development']

function profileDraft(profile: TeamScoreProfile): DraftMetric[] {
  return profile.items.map((item) => ({
    ...item,
    weightText: String(item.weight_bps / 100),
    sampleText: String(item.minimum_samples),
  }))
}

export function TeamScoreSettingsPanel({
  open,
  onOpenChange,
  profile,
  onSave,
  onReset,
  saveState,
  canUpdate = true,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile: TeamScoreProfile
  onSave: (body: TeamScoreProfileUpdate) => void
  onReset: (body: TeamScoreProfileUpdate) => void
  saveState: 'idle' | 'pending' | 'success' | 'error'
  canUpdate?: boolean
}) {
  const copy = BI_SCREEN_COPY.team
  const metricInputPrefix = useId()
  const pending = saveState === 'pending'
  const canManage = canUpdate && profile.can_manage
  const [role, setRole] = useState<TeamMetricRow['role_key']>('product')
  const [draft, setDraft] = useState(() => profileDraft(profile))
  useEffect(() => {
    if (open) {
      setDraft(profileDraft(profile))
      setRole('product')
    }
  }, [open, profile])
  const totals = useMemo(() => Object.fromEntries(ROLE_ORDER.map((roleKey) => [
    roleKey,
    draft.filter((item) => item.role_key === roleKey && item.enabled).reduce((total, item) => (
      total + Math.round((Number(item.weightText) || 0) * 100)
    ), 0),
  ])) as Record<TeamMetricRow['role_key'], number>, [draft])
  const coreTotals = useMemo(() => Object.fromEntries(ROLE_ORDER.map((roleKey) => [
    roleKey,
    draft.filter((item) => (
      item.role_key === roleKey
      && item.enabled
      && (item.metric_key === 'workload_share' || item.metric_key === 'completion_rate')
    )).reduce((total, item) => total + Math.round((Number(item.weightText) || 0) * 100), 0),
  ])) as Record<TeamMetricRow['role_key'], number>, [draft])
  const totalsValid = ROLE_ORDER.every((roleKey) => totals[roleKey] === 10_000 && draft.some((item) => (
    item.role_key === roleKey && item.enabled
  )))
  const coreTotalsValid = ROLE_ORDER.every((roleKey) => coreTotals[roleKey] >= 5_000)
  const valid = totalsValid && coreTotalsValid
  const visibleMetrics = draft.filter((item) => item.role_key === role)
  const body = (resetToDefault = false): TeamScoreProfileUpdate => ({
    expected_version: profile.version,
    reset_to_default: resetToDefault || undefined,
    items: draft.map((item) => ({
      metric_key: item.metric_key,
      role_key: item.role_key,
      enabled: item.enabled,
      weight_bps: Math.round((Number(item.weightText) || 0) * 100),
      missing_policy: item.missing_policy,
      minimum_samples: Math.max(1, Math.round(Number(item.sampleText) || 1)),
    })),
  })
  const update = (metric: DraftMetric, updater: (item: DraftMetric) => DraftMetric) => {
    setDraft((current) => current.map((item) => (
      item.metric_key === metric.metric_key && item.role_key === metric.role_key ? updater(item) : item
    )))
  }
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      title={copy.settingsTitle}
      description={copy.settingsDescription}
      pending={pending}
      className="max-w-4xl"
      footer={canManage ? (
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={() => onReset(body(true))}>{copy.resetWeights}</Button>
          <Button type="button" disabled={!valid || pending} onClick={() => onSave(body())}>{pending ? copy.savingWeights : copy.saveWeights}</Button>
        </>
      ) : undefined}
    >
      <div className="grid gap-4">
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm">
          <span>{copy.profileVersion(profile.version)}</span>
          <span className="flex flex-wrap items-center gap-x-4 gap-y-1">
            <strong className="tabular-nums">{copy.weightTotalValue((totals[role] / 100).toLocaleString('zh-CN'))}</strong>
            <strong className="tabular-nums">{copy.coreWeightValue((coreTotals[role] / 100).toLocaleString('zh-CN'))}</strong>
          </span>
        </div>
        <ViewTabs
          value={role}
          label={copy.roleFilter}
          tabs={ROLE_ORDER.map((roleKey) => ({ id: roleKey, label: copy.roleLabels[roleKey] }))}
          onValueChange={(value) => setRole(value as TeamMetricRow['role_key'])}
        />
        {!canManage ? <p className="m-0 text-sm text-muted-foreground">{copy.settingsReadOnly}</p> : null}
        <div className="grid gap-2">
          {visibleMetrics.map((metric) => (
            <div key={`${metric.role_key}:${metric.metric_key}`} className="grid gap-3 rounded-lg border border-border p-3 lg:grid-cols-[auto_minmax(0,1fr)_8rem_8rem] lg:items-center">
              <div className="inline-flex items-center gap-2 text-sm">
                <Checkbox
                  id={`${metricInputPrefix}-${metric.role_key}-${metric.metric_key}`}
                  checked={metric.enabled}
                  disabled={!canManage || pending || (metric.status !== 'active' && !metric.enabled)}
                  onCheckedChange={(enabled) => update(metric, (item) => ({ ...item, enabled }))}
                />
                <label htmlFor={`${metricInputPrefix}-${metric.role_key}-${metric.metric_key}`}>
                  {copy.metricEnabled}
                </label>
              </div>
              <div className="min-w-0">
                <p className="m-0 text-sm font-medium">{metric.label}</p>
                <p className="mt-1 mb-0 text-xs text-muted-foreground">{metric.description}</p>
                <dl className="mt-2 mb-0 grid gap-1 text-xs">
				  <div><dt className="inline font-medium">{copy.metricFormulaLabel}</dt><dd className="inline">{metric.formula}</dd></div>
				  <div><dt className="inline font-medium">{copy.metricDirectionLabel}</dt><dd className="inline">{metric.direction === 'lower_better' ? copy.lowerBetter : copy.higherBetter}</dd></div>
                </dl>
              </div>
              <label className="grid gap-1 text-xs text-muted-foreground">
                {copy.metricWeight}
                <span className="flex items-center gap-2">
                  <Input type="number" min="0" max="100" step="0.01" value={metric.weightText} disabled={!canManage || pending || !metric.enabled} onChange={(event) => update(metric, (item) => ({ ...item, weightText: event.target.value }))} aria-label={copy.metricWeightLabel(metric.label)} />
                  <span>{copy.percentSuffix}</span>
                </span>
              </label>
              <label className="grid gap-1 text-xs text-muted-foreground">
                {copy.minimumSamples}
                <Input type="number" min="1" max="10000" step="1" value={metric.sampleText} disabled={!canManage || pending || !metric.enabled} onChange={(event) => update(metric, (item) => ({ ...item, sampleText: event.target.value }))} aria-label={copy.minimumSamplesLabel(metric.label)} />
              </label>
            </div>
          ))}
        </div>
        <p role={valid ? 'status' : 'alert'} className={valid ? 'm-0 text-sm text-emerald-700 dark:text-emerald-400' : 'm-0 text-sm text-destructive'}>
          {valid ? copy.weightValid : (totalsValid ? copy.coreWeightInvalid : copy.weightInvalid)}
        </p>
        {saveState === 'success' ? <p role="status" className="m-0 text-sm text-emerald-700 dark:text-emerald-400">{copy.weightsSaved}</p> : null}
        {saveState === 'error' ? <p role="alert" className="m-0 text-sm text-destructive">{copy.weightsSaveFailed}</p> : null}
      </div>
    </ResponsivePanel>
  )
}
