import { describe, expect, it } from 'bun:test'

import type { WorkflowInstance } from '../model/types'
import {
  workflowActionNeedsNextAssignees,
  workflowAssigneeRequiredNodes,
  workflowEdgeMatchesAction,
  workflowResolvedNextAssignee,
} from '../utils/workflow-next-assignees'

function instance(partial: Partial<WorkflowInstance>): WorkflowInstance {
  return {
    id: 1, template_id: 1, template_name: '流程', template_version: 1, status: 'running',
    subject_type: 'requirement', subject_id: 1, current_node_ids: ['input'], current_nodes: [],
    timeline_nodes: [], definition_edges: [], node_runs: [], approvals: [],
    completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-11T00:00:00Z',
    ...partial,
  }
}

describe('workflow-next-assignees', () => {
  it('skips reject and terminal nodes', () => {
    expect(workflowActionNeedsNextAssignees('reject')).toBe(false)
    expect(workflowEdgeMatchesAction('approve', 'approve')).toBe(true)
    expect(workflowEdgeMatchesAction('reject', 'complete')).toBe(false)
    const next = workflowAssigneeRequiredNodes(instance({
      timeline_nodes: [
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'e1', source: 'input', target: 'end' }],
    }), 'input', 'complete')
    expect(next).toEqual([])
  })

  it('requires each parallel business node including planned assignees', () => {
    const fixture = instance({
      timeline_nodes: [
        { id: 'split', type: 'parallel_split', name: '并行', status: 'pending' },
        { id: 'dev', type: 'human_task', name: '研发', status: 'pending' },
        { id: 'qa', type: 'human_task', name: '测试', status: 'pending' },
      ],
      definition_edges: [
        { id: 'e1', source: 'input', target: 'split' },
        { id: 'e2', source: 'split', target: 'dev' },
        { id: 'e3', source: 'split', target: 'qa' },
      ],
      node_plans: { qa: { assignee_user_id: 8 } },
    })
    expect(workflowAssigneeRequiredNodes(fixture, 'input', 'complete')).toEqual([
      { id: 'dev', name: '研发' },
      { id: 'qa', name: '测试' },
    ])
    expect(workflowResolvedNextAssignee({}, 'qa')).toBe(null)
    expect(workflowResolvedNextAssignee({ qa: 21 }, 'qa')).toBe(21)
  })

  it('picks the condition branch that matches the decision', () => {
    const fixture = instance({
      timeline_nodes: [
        { id: 'ui', type: 'condition', name: '是否需要UI设计', status: 'active' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending' },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
      ],
      definition_edges: [
        { id: 'yes', source: 'ui', target: 'design', condition: 'context.decision == true' },
        { id: 'no', source: 'ui', target: 'develop', condition: 'context.decision == false' },
      ],
    })
    expect(workflowAssigneeRequiredNodes(fixture, 'ui', 'condition_yes')).toEqual([{ id: 'design', name: 'UI/UX设计' }])
    expect(workflowAssigneeRequiredNodes(fixture, 'ui', 'condition_no')).toEqual([{ id: 'develop', name: '研发实现' }])
    const planned = instance({
      ...fixture,
      node_plans: { design: { assignee_user_id: 1 }, develop: { assignee_user_id: 1 } },
    })
    expect(workflowAssigneeRequiredNodes(planned, 'ui', 'condition_yes')).toEqual([{ id: 'design', name: 'UI/UX设计' }])
    expect(workflowResolvedNextAssignee({}, 'design')).toBe(null)
    expect(workflowResolvedNextAssignee({ design: 31 }, 'design')).toBe(31)
  })

  it('requires the next assignee on form complete and approval even when a plan already exists', () => {
    const fixture = instance({
      timeline_nodes: [
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'pending' },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'e1', source: 'input', target: 'review' },
        { id: 'e2', source: 'review', target: 'develop', kind: 'approve' },
        { id: 'e3', source: 'develop', target: 'end' },
      ],
      node_plans: { review: { assignee_user_id: 1 }, develop: { assignee_user_id: 1 } },
    })
    expect(workflowAssigneeRequiredNodes(fixture, 'input', 'complete')).toEqual([{ id: 'review', name: '产品评审' }])
    expect(workflowAssigneeRequiredNodes(fixture, 'review', 'approve')).toEqual([{ id: 'develop', name: '研发实现' }])
    expect(workflowResolvedNextAssignee({}, 'review')).toBe(null)
    expect(workflowResolvedNextAssignee({ review: 21 }, 'review')).toBe(21)
    expect(workflowResolvedNextAssignee({ develop: 41 }, 'develop')).toBe(41)
  })

  it('skips the AI node when leaving requirement entry', () => {
    const fixture = instance({
      timeline_nodes: [
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'ai', type: 'ai_check', name: '完整性检查', status: 'pending' },
      ],
      definition_edges: [{ id: 'e1', source: 'input', target: 'ai' }],
      node_plans: { ai: { assignee_user_id: 1 } },
    })
    expect(workflowAssigneeRequiredNodes(fixture, 'input', 'complete')).toEqual([])
  })
})
