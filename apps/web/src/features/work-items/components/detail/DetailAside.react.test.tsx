import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItem } from '../../model/types'
import { DetailAside } from './DetailAside'

const item: WorkItem = {
  id: 5474,
  rowId: 'task:5474',
  title: '推荐位二级页',
  status: 'draft',
  priority: 'normal',
  projectSetId: 5,
  projectId: 15,
  requirementId: 97,
  requirementTitle: '小说漫画需求',
  watched: false,
  capabilities: {
    canUpdate: false,
    canDelete: false,
    canComment: false,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: [],
  },
}

const draft = {
  title: item.title,
  description: '',
  status: item.status || '',
  priority: item.priority || '',
  assigneeId: undefined,
  startsAt: '',
  dueAt: '',
  estimatedMinutes: '',
}

describe('work-item detail metadata', () => {
  it('links the owning requirement through its canonical plan-unit route', () => {
    const onOpenRequirement = vi.fn()
    render(
      <DetailAside
        item={item}
        draft={draft}
        editable={false}
        saveState="idle"
        workflow={{ status: 'idle' }}
        onOpenRequirement={onOpenRequirement}
        onChange={vi.fn()}
      />,
    )

    expect(screen.getByRole('heading', { name: WORK_ITEMS_COPY.detail.metadataTitle })).toBeTruthy()
    expect(screen.getByText(WORK_ITEMS_COPY.detail.status).closest('dl')?.className).toContain(
      'grid-cols-[minmax(7rem,auto)_minmax(0,1fr)]',
    )
    expect(screen.getByRole('link', { name: '小说漫画需求' }).getAttribute('href')).toBe(
      '/workspace/projects/5/items/15?tab=requirements',
    )
    fireEvent.click(screen.getByRole('link', { name: '小说漫画需求' }))
    expect(onOpenRequirement).toHaveBeenCalledWith(item)
  })

  it('does not invent a link when the hierarchy is incomplete', () => {
    render(
      <DetailAside
        item={{ ...item, projectId: undefined }}
        draft={draft}
        editable={false}
        saveState="idle"
        workflow={{ status: 'idle' }}
        onChange={vi.fn()}
      />,
    )

    expect(screen.getByText(item.requirementTitle || WORK_ITEMS_COPY.list.noRequirement)).toBeTruthy()
    expect(screen.queryByRole('link', { name: '小说漫画需求' })).toBeNull()
  })

  it('shows the parent task name as a link and opens its detail', () => {
    const onOpenParent = vi.fn()
    render(
      <DetailAside
        item={{ ...item, kind: 'subtask', parentId: 5400 }}
        parent={{
          ...item,
          relation: 'parent', id: 5400, taskKey: 'MIG-S-0200',
          title: '积分兑换签到任务',
        }}
        draft={draft}
        editable={false}
        saveState="idle"
        workflow={{ status: 'idle' }}
        onOpenParent={onOpenParent}
        onChange={vi.fn()}
      />,
    )

    const parent = screen.getByRole('link', { name: '积分兑换签到任务' })
    expect(parent.getAttribute('href')).toBe('/workspace/work-items?task_id=5400')
    fireEvent.click(parent)
    expect(onOpenParent).toHaveBeenCalledWith(5400)
    expect(screen.queryByText('#5400')).toBeNull()
  })

  it('falls back to the parent id while the related resource is unavailable', () => {
    render(
      <DetailAside
        item={{ ...item, kind: 'subtask', parentId: 5400 }}
        draft={draft}
        editable={false}
        saveState="idle"
        workflow={{ status: 'idle' }}
        onChange={vi.fn()}
      />,
    )

    expect(screen.getByText('#5400')).toBeTruthy()
    expect(screen.queryByRole('link', { name: '#5400' })).toBeNull()
  })
})
