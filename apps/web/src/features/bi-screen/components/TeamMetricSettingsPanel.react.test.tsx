import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { TeamMetricSettings } from '../types'

import { TeamMetricSettingsPanel } from './TeamMetricSettingsPanel'

const settings: TeamMetricSettings = {
  version: 0,
  can_manage: true,
  configured: false,
  valid: false,
  options: [{
    template_id: 12,
    template_name: '需求评审流程',
    published_version: 3,
    approval_nodes: [{ node_id: 'review', node_name: '产品评审' }],
  }],
}

describe('TeamMetricSettingsPanel', () => {
  it('requires an explicit approval-node selection before saving', async () => {
    const onSave = vi.fn()
    render(
      <TeamMetricSettingsPanel
        open
        onOpenChange={vi.fn()}
        settings={settings}
        loadState="ready"
        onRetry={vi.fn()}
        onSave={onSave}
        saveState="idle"
      />,
    )

    expect(await screen.findByText(/尚未配置产品评审节点/)).toBeTruthy()
    expect(screen.getByText('发布版本 V3')).toBeTruthy()
    const save = screen.getByRole('button', { name: '保存配置' })
    expect(save.hasAttribute('disabled')).toBe(true)

    fireEvent.click(screen.getByRole('combobox', { name: '产品评审审批节点' }))
    const option = await screen.findByRole('option', { name: '产品评审' })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    await waitFor(() => expect(save.hasAttribute('disabled')).toBe(false))
    fireEvent.click(save)

    expect(onSave).toHaveBeenCalledWith({
      expected_version: 0,
      requirement_workflow_template_id: 12,
      approval_node_id: 'review',
    })
  })

  it('shows an invalid saved node and keeps read-only users from editing it', async () => {
    render(
      <TeamMetricSettingsPanel
        open
        onOpenChange={vi.fn()}
        settings={{
          ...settings,
          version: 4,
          can_manage: false,
          configured: true,
          requirement_workflow_template_id: 12,
          approval_node_id: 'removed-review',
        }}
        loadState="ready"
        onRetry={vi.fn()}
        onSave={vi.fn()}
        saveState="idle"
      />,
    )

    expect(await screen.findByText(/原配置节点已不在当前发布模板中/)).toBeTruthy()
    expect(screen.getByText(/没有修改工作区指标口径的权限/)).toBeTruthy()
    expect(screen.queryByRole('button', { name: '保存配置' })).toBeNull()
    expect(screen.getByRole('combobox', { name: '需求工作流模板' }).hasAttribute('disabled')).toBe(true)
  })

  it('exposes loading and retry states without rendering stale settings', async () => {
    const { rerender } = render(
      <TeamMetricSettingsPanel
        open
        onOpenChange={vi.fn()}
        loadState="loading"
        onRetry={vi.fn()}
        onSave={vi.fn()}
        saveState="idle"
      />,
    )
    expect(await screen.findByRole('status', { name: /正在加载产品评审指标配置/ })).toBeTruthy()

    const onRetry = vi.fn()
    rerender(
      <TeamMetricSettingsPanel
        open
        onOpenChange={vi.fn()}
        loadState="error"
        onRetry={onRetry}
        onSave={vi.fn()}
        saveState="idle"
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('offers a settings reload after a version conflict or save failure', async () => {
    const onRetry = vi.fn()
    render(
      <TeamMetricSettingsPanel
        open
        onOpenChange={vi.fn()}
        settings={settings}
        loadState="ready"
        onRetry={onRetry}
        onSave={vi.fn()}
        saveState="error"
      />,
    )

    expect((await screen.findByRole('alert')).textContent).toContain('配置保存失败')
    fireEvent.click(screen.getByRole('button', { name: '重新加载' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })
})
