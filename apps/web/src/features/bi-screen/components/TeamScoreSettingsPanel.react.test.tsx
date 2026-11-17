import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { BI_SCREEN_COPY } from '@/shared/copy'
import type { TeamScoreProfile } from '../types'
import { TeamScoreSettingsPanel } from './TeamScoreSettingsPanel'

const copy = BI_SCREEN_COPY.team
const profile: TeamScoreProfile = {
  id: 1, version: 2, source: 'workspace', can_manage: true,
  total_weight_bps: 10_000, minimum_coverage_bps: 5_000, minimum_workload: 1,
  items: ['product', 'testing', 'development'].map((role) => ({
    metric_key: 'completion_rate', role_key: role as 'product' | 'testing' | 'development',
    label: `${role} completion`, description: 'Completion metric', formula: 'completed / total',
    unit: 'percent', direction: 'higher_better', normalization: 'linear',
    applicable_roles: ['product', 'testing', 'development'], definition_version: 1,
    status: 'active', enabled: true, weight_bps: 10_000, weight_pct: 100,
    missing_policy: 'renormalize', minimum_samples: 1, sort_order: 1,
  })),
}

function renderPanel(overrides: { canUpdate?: boolean; saveState?: 'idle' | 'pending'; profile?: TeamScoreProfile } = {}) {
  const onSave = vi.fn()
  const result = render(<TeamScoreSettingsPanel open onOpenChange={vi.fn()} profile={profile}
    onSave={onSave} onReset={vi.fn()} saveState="idle" {...overrides} />)
  return { ...result, onSave }
}

describe('TeamScoreSettingsPanel metric selection', () => {
  it('updates the enabled value, dependent fields and weight validation, then saves the draft', async () => {
    const { onSave } = renderPanel()
    const toggle = await screen.findByRole('checkbox', { name: copy.metricEnabled })
    const weight = screen.getByRole('spinbutton', { name: copy.metricWeightLabel('product completion') })
    const save = screen.getByRole('button', { name: copy.saveWeights })
    fireEvent.click(toggle)
    expect(toggle.getAttribute('aria-checked')).toBe('false')
    expect(weight.hasAttribute('disabled')).toBe(true)
    expect(save.hasAttribute('disabled')).toBe(true)
    fireEvent.click(screen.getByText(copy.metricEnabled))
    expect(toggle.getAttribute('aria-checked')).toBe('true')
    expect(weight.hasAttribute('disabled')).toBe(false)
    fireEvent.click(save)
    expect(onSave).toHaveBeenCalledWith({ expected_version: 2, reset_to_default: undefined,
      items: profile.items.map(({ metric_key, role_key, enabled, weight_bps, missing_policy, minimum_samples }) =>
        ({ metric_key, role_key, enabled, weight_bps, missing_policy, minimum_samples })),
    })
  })

  it.each([
    { canUpdate: false },
    { saveState: 'pending' as const },
    { profile: { ...profile, can_manage: false } },
    { profile: { ...profile, items: profile.items.map((item) => ({ ...item, enabled: false, status: 'deprecated' as const })) } },
  ])('preserves disabled selection for restricted metric settings %j', async (overrides) => {
    renderPanel(overrides)
    const toggle = await screen.findByRole('checkbox', { name: copy.metricEnabled })
    const checked = toggle.getAttribute('aria-checked')
    expect(toggle.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(toggle)
    expect(toggle.getAttribute('aria-checked')).toBe(checked)
  })
})
