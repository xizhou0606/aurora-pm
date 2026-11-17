import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'
import { WorkflowAIResult, workflowCompletenessIsStale } from './WorkflowAIResult'

describe('WorkflowAIResult task plan', () => {
  it('previews the complete task plan without claiming that task records already exist', () => {
    render(<WorkflowAIResult nodeName="AI任务拆解" execution={{
      run_id: 78,
      status: 'completed',
      capability: 'requirement.task_plan',
      output: {
        summary: '已拆分前后端研发任务。', changes: [], items: [], risks: [], humanReviewRequired: true,
        taskPlan: { chains: [
          { lane: 'frontend', summary: '前端交付', tasks: [{ title: '实现页面', description: '完成交互和接口接入', acceptanceCriteria: ['页面可操作'] }] },
          { lane: 'backend', summary: '后端交付', tasks: [{ title: '实现接口', description: '完成租户隔离和持久化', acceptanceCriteria: ['接口测试通过'] }] },
        ] },
      },
    }} />)

    expect(screen.getByText('任务方案待确认')).toBeTruthy()
    expect(screen.getByText('确认采用后将创建并关联 4 个任务')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('确认采用后将创建并关联 4 个任务')
    expect(within(screen.getByRole('region', { name: '前端链路' })).getByText('1. 实现页面')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: '后端链路' })).getByText('1. 实现接口')).toBeTruthy()
    expect(screen.queryByRole('link', { name: /打开任务/ })).toBeNull()
  })

  it('renders applied frontend and backend chains with links to created tasks', () => {
    render(<WorkflowAIResult nodeName="AI任务拆解" execution={{
      run_id: 79,
      status: 'applied',
      capability: 'requirement.task_plan',
      output: {
        summary: '已拆分前后端研发任务。', changes: [], items: [], risks: [], humanReviewRequired: true,
        taskPlan: { chains: [
          { lane: 'frontend', summary: '前端交付', tasks: [{ title: '实现页面', description: '完成交互和接口接入', acceptanceCriteria: ['页面可操作'] }] },
          { lane: 'backend', summary: '后端交付', tasks: [{ title: '实现接口', description: '完成租户隔离和持久化', acceptanceCriteria: ['接口测试通过'] }] },
        ] },
        createdTasks: [
          { id: 101, taskKey: 'DEMO-1', title: '前端研发 · 结算优化', lane: 'frontend', kind: 'task' },
          { id: 102, taskKey: 'DEMO-2', title: '实现页面', lane: 'frontend', kind: 'subtask', parentId: 101 },
          { id: 103, taskKey: 'DEMO-3', title: '后端研发 · 结算优化', lane: 'backend', kind: 'task' },
          { id: 104, taskKey: 'DEMO-4', title: '实现接口', lane: 'backend', kind: 'subtask', parentId: 103 },
        ],
      },
    }} />)

    expect(screen.getByText('已创建并关联 4 个任务')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: '前端链路' })).getByText('1. 实现页面')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: '后端链路' })).getByText('1. 实现接口')).toBeTruthy()
    expect(screen.getByRole('link', { name: '打开任务 DEMO-2' }).getAttribute('href')).toBe('/workspace/work-items?task_id=102')
    expect(screen.getByRole('link', { name: '打开任务 DEMO-4' }).getAttribute('href')).toBe('/workspace/work-items?task_id=104')
  })

  it('shows task-specific queued copy before the worker claims the run', () => {
    render(<WorkflowAIResult nodeName="AI任务拆解" execution={{ run_id: 80, status: 'queued', capability: 'requirement.task_plan' }} />)

    expect(screen.getByRole('status').textContent).toContain('按前端、后端两条链路生成任务')
  })
})

describe('workflowCompletenessIsStale', () => {
  it('treats a newer requirement version as stale against the checked AI run', () => {
    expect(workflowCompletenessIsStale(5, { subject_version: 4 })).toBe(true)
    expect(workflowCompletenessIsStale(5, {
      output: { summary: '', changes: [], items: [], risks: [], humanReviewRequired: false, completenessReport: {
        verdict: 'blocked', score: 22, threshold: 80, canProceed: false, checkedSubjectVersion: 4,
        dimensions: [], missingRequiredFields: [], blockingIssues: [], recommendations: [],
      } },
    })).toBe(true)
    expect(workflowCompletenessIsStale(4, { subject_version: 4 })).toBe(false)
    expect(workflowCompletenessIsStale(undefined, { subject_version: 4 })).toBe(false)
  })
})
