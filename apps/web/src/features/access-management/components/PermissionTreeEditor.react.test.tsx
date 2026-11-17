import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import type { PermissionNode } from '../model'
import { PermissionTreeEditor } from './PermissionTreeEditor'

const nodes: PermissionNode[] = [{
  code: 'testing',
  name: '测试管理',
  type: 'page',
  grantable: false,
  tools: [],
  objectPolicy: null,
  children: [{
    code: 'testing.report',
    name: '测试报告',
    type: 'view',
    grantable: false,
    tools: [],
    objectPolicy: null,
    children: [{
      code: 'testing.report.version',
      name: '版本测试报告',
      type: 'module',
      grantable: false,
      tools: [],
      objectPolicy: null,
      children: [
        {
          code: 'testing.report.version.view',
          name: '查看',
          type: 'action',
          grantable: true,
          tools: ['task.get'],
          objectPolicy: { code: 'task.creator', kind: 'creator_override' },
          children: [],
        },
        {
          code: 'testing.report.version.download',
          name: '下载',
          type: 'action',
          grantable: true,
          tools: ['task.attachment.list', 'task.attachment.download'],
          objectPolicy: null,
          children: [],
        },
        {
          code: 'testing.report.version.internal',
          name: '系统内部动作',
          type: 'action',
          grantable: false,
          tools: [],
          objectPolicy: null,
          children: [],
        },
      ],
    }],
  }],
}]

const implicationNodes: PermissionNode[] = [{
  code: 'quality', name: '质量管理', type: 'page', grantable: false,
  tools: [], objectPolicy: null, children: [{
    code: 'quality.bugs', name: '缺陷', type: 'module', grantable: false,
    tools: [], objectPolicy: null, children: [
      {
        code: PM_ACTION.QualityBugsUpdate, name: '编辑', type: 'action', grantable: true,
        tools: ['quality.bug.update'], objectPolicy: null, children: [],
      },
      {
        code: PM_ACTION.QualityBugsTransitionStatus, name: '流转状态', type: 'action', grantable: true,
        tools: ['quality.bug.status.update'], objectPolicy: null, children: [],
      },
    ],
  }],
}]

function renderEditor(selectedCodes: string[], onChange = vi.fn()) {
  return render(
    <PermissionTreeEditor
      subjectName="测试角色"
      nodes={nodes}
      selectedCodes={selectedCodes}
      loading={false}
      loadError={false}
      pending={false}
      saveError={false}
      disabled={false}
      onRetry={vi.fn()}
      onChange={onChange}
      onSave={vi.fn()}
    />,
  )
}

describe('PermissionTreeEditor', () => {
  it('renders page, view and module ancestors as truly indeterminate for a partial grant', () => {
    const viewCode = 'testing.report.version.view'
    const { rerender } = renderEditor([viewCode])
    const ancestorNames = ['测试管理', '测试报告', '版本测试报告']

    ancestorNames.forEach((name) => {
      const checkbox = screen.getByRole('checkbox', { name })
      expect(checkbox.getAttribute('aria-checked')).toBe('mixed')
      expect(checkbox.hasAttribute('data-indeterminate')).toBe(true)
    })
    expect(screen.getByRole('checkbox', { name: '查看' }).getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('checkbox', { name: '下载' }).getAttribute('aria-checked')).toBe('false')

    rerender(
      <PermissionTreeEditor
        subjectName="测试角色"
        nodes={nodes}
        selectedCodes={[viewCode, 'testing.report.version.download']}
        loading={false}
        loadError={false}
        pending={false}
        saveError={false}
        disabled={false}
        onRetry={vi.fn()}
        onChange={vi.fn()}
        onSave={vi.fn()}
      />,
    )

    ancestorNames.forEach((name) => {
      const checkbox = screen.getByRole('checkbox', { name })
      expect(checkbox.getAttribute('aria-checked')).toBe('true')
      expect(checkbox.hasAttribute('data-indeterminate')).toBe(false)
    })
    const internalAction = screen.getByRole('checkbox', { name: '系统内部动作' })
    expect(internalAction.getAttribute('aria-checked')).toBe('false')
    expect(internalAction.getAttribute('aria-disabled')).toBe('true')
  })

  it('keeps empty ancestors unchecked and selects only grantable leaves from a mixed branch', () => {
    const onChange = vi.fn()
    renderEditor([], onChange)

    for (const name of ['测试管理', '测试报告', '版本测试报告']) {
      const checkbox = screen.getByRole('checkbox', { name })
      expect(checkbox.getAttribute('aria-checked')).toBe('false')
      expect(checkbox.hasAttribute('data-indeterminate')).toBe(false)
    }

    fireEvent.click(screen.getByRole('checkbox', { name: '版本测试报告' }))
    expect(onChange).toHaveBeenCalledWith([
      'testing.report.version.download',
      'testing.report.version.view',
    ])
  })

  it('selects and clears every grantable permission without including protected actions', () => {
    const onChange = vi.fn()
    const { rerender } = renderEditor([], onChange)

    fireEvent.click(screen.getByRole('button', { name: '全选' }))
    expect(onChange).toHaveBeenCalledWith([
      'testing.report.version.download',
      'testing.report.version.view',
    ])

    rerender(
      <PermissionTreeEditor
        subjectName="测试角色"
        nodes={nodes}
        selectedCodes={[
          'testing.report.version.download',
          'testing.report.version.view',
        ]}
        loading={false}
        loadError={false}
        pending={false}
        saveError={false}
        disabled={false}
        onRetry={vi.fn()}
        onChange={onChange}
        onSave={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '取消全选' }))
    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it('opens an accessible Tool list on hover without changing permission selection', () => {
    const onChange = vi.fn()
    renderEditor([], onChange)

    const trigger = screen.getByRole('button', { name: '查看“查看”动作关联的 1 个 Tool' })
    expect(trigger.tagName).toBe('BUTTON')
    expect(trigger.textContent).toBe('1 个 Tool')
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    fireEvent.pointerEnter(trigger, { pointerType: 'mouse' })

    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    fireEvent.click(screen.getByText('task.get'))
    expect(screen.getByRole('list', { name: '查看关联的 Tools' })).toBeTruthy()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('shows Tool lists while the permission editor is read-only', () => {
    render(
      <PermissionTreeEditor
        subjectName="系统角色"
        nodes={nodes}
        selectedCodes={[]}
        loading={false}
        loadError={false}
        pending={false}
        saveError={false}
        disabled
        onRetry={vi.fn()}
        onChange={vi.fn()}
        onSave={vi.fn()}
      />,
    )

    const trigger = screen.getByRole('button', { name: '查看“下载”动作关联的 2 个 Tool' })
    expect(trigger.hasAttribute('disabled')).toBe(false)
    expect(trigger.textContent).toBe('2 个 Tools')
    fireEvent.click(trigger)
    expect(screen.getByText('task.attachment.list')).toBeTruthy()
    expect(screen.getByText('task.attachment.download')).toBeTruthy()
  })

  it('shows the creator override policy without changing the action selection', () => {
    const onChange = vi.fn()
    renderEditor([], onChange)

    const policy = screen.getByText('创建者可操作')
    expect(policy.getAttribute('data-slot')).toBe('status-pill')
    fireEvent.click(policy)
    expect(onChange).not.toHaveBeenCalled()
  })

  it('shows implied actions as effective without pretending they can be removed independently', () => {
    render(
      <PermissionTreeEditor
        subjectName="质量管理员"
        nodes={implicationNodes}
        selectedCodes={[PM_ACTION.QualityBugsUpdate]}
        loading={false}
        loadError={false}
        pending={false}
        saveError={false}
        disabled={false}
        onRetry={vi.fn()}
        onChange={vi.fn()}
        onSave={vi.fn()}
      />,
    )

    const transition = screen.getByRole('checkbox', { name: '流转状态' })
    expect(transition.getAttribute('aria-checked')).toBe('true')
    expect(transition.getAttribute('aria-disabled')).toBe('true')
    expect(screen.getByText('由上级权限包含')).toBeTruthy()
  })
})
