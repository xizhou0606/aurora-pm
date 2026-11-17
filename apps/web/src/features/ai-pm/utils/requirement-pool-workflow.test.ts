import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import {
  REQUIREMENT_POOL_NODE_COMPLETED,
  REQUIREMENT_POOL_NODE_IDLE,
} from '../utils/requirement-pool-filter-fields'
import {
  requirementCurrentNode,
  requirementCurrentNodes,
  requirementMatchesCurrentNodeFilter,
  requirementPoolWorkflowNodeOptions,
  requirementWorkflowNodeSearch,
  requirementWorkflowNodeTargets,
  resolveRequirementWorkflowNode,
} from '../utils/requirement-pool-workflow'

const poolCopy = copy.aiPM.requirementPool

describe('requirement pool workflow nodes', () => {
  it('exposes clickable current nodes when the overview includes ids', () => {
    const requirement = {
      workflow_overview: {
        status: 'running',
        current_node_names: ['产品评审', '研发实现'],
        current_nodes: [
          { id: 'review', name: '产品评审', overdue: true },
          { id: ' develop ', name: '研发实现' },
        ],
      },
    }

    expect(requirementCurrentNodes(requirement)).toEqual([
      { id: 'review', name: '产品评审', overdue: true },
      { id: 'develop', name: '研发实现', overdue: false },
    ])
    expect(requirementWorkflowNodeTargets(requirement)).toEqual([
      { id: 'review', name: '产品评审' },
      { id: 'develop', name: '研发实现' },
    ])
    expect(requirementCurrentNode(requirement)).toEqual({ label: '产品评审', tone: 'overdue' })
  })

  it('does not invent a node id when the overview only has names', () => {
    const requirement = {
      workflow_overview: {
        status: 'running',
        current_node_names: ['UI/UX设计'],
      },
    }

    expect(requirementCurrentNodes(requirement)).toEqual([{ name: 'UI/UX设计' }])
    expect(requirementWorkflowNodeTargets(requirement)).toEqual([{ name: 'UI/UX设计' }])
    expect(requirementCurrentNode(requirement)).toEqual({ label: 'UI/UX设计', tone: 'running' })
  })

  it('keeps completed and unbound rows from looking like current todos', () => {
    expect(requirementCurrentNodes({
      workflow_overview: { status: 'completed', current_node_names: [poolCopy.workflowCompleted] },
    })).toEqual([])
    expect(requirementCurrentNode({ workflow_overview: { status: 'completed' } })).toEqual({
      label: poolCopy.workflowCompleted,
      tone: 'completed',
    })
    expect(requirementCurrentNode({})).toEqual({
      label: poolCopy.workflowNotStarted,
      tone: 'idle',
    })
  })

  it('builds a bounded workflow-node search payload', () => {
    expect(requirementWorkflowNodeSearch(' review ')).toEqual({ workflow_node: 'review' })
    expect(requirementWorkflowNodeSearch('   ')).toEqual({})
  })

  it('resolves a list click by node name when the overview has no id', () => {
    const instance = {
      current_nodes: [
        { id: 'form-requirement-input', type: 'form', name: '需求录入', status: 'active' },
      ],
      timeline_nodes: [
        { id: 'join', type: 'parallel_join', name: '交付汇聚', status: 'waiting' },
        { id: 'form-requirement-input', type: 'form', name: '需求录入', status: 'active' },
      ],
    }

    expect(resolveRequirementWorkflowNode(instance, { name: '需求录入' })).toEqual(
      instance.current_nodes[0],
    )
    expect(resolveRequirementWorkflowNode(instance, { id: 'form-requirement-input' })?.id).toBe(
      'form-requirement-input',
    )
  })

  it('collects published requirement workflow nodes and skips start/end', () => {
    expect(requirementPoolWorkflowNodeOptions([
      {
        subject_type: 'requirement',
        published_version: 2,
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'input', type: 'form', name: '需求录入' },
          { id: 'review', type: 'approval', name: '产品评审' },
          { id: 'end', type: 'end', name: '结束' },
        ],
      },
      {
        subject_type: 'plan_unit',
        published_version: 1,
        nodes: [{ id: 'delivery', type: 'human_task', name: '版本交付' }],
      },
    ])).toEqual([
      { id: 'input', name: '需求录入' },
      { id: 'review', name: '产品评审' },
    ])
  })

  it('reads published definition nodes when the list payload omits nodes', () => {
    expect(requirementPoolWorkflowNodeOptions([
      {
        subject_type: 'requirement',
        published_version: 1,
        definition: {
          nodes: [
            { id: 'start', type: 'start', name: '开始' },
            { id: 'task_plan', type: 'ai_generate', name: 'AI任务拆解' },
            { id: 'end', type: 'end', name: '结束' },
          ],
        },
      },
    ])).toEqual([{ id: 'task_plan', name: 'AI任务拆解' }])
  })

  it('matches current-node filters against the displayed workflow state', () => {
    const running = {
      workflow_overview: {
        status: 'running',
        current_node_names: ['产品评审'],
        current_nodes: [{ id: 'product_review', name: '产品评审' }],
      },
    }
    const taskPlan = {
      workflow_overview: {
        status: 'running',
        current_node_names: ['AI任务拆解'],
        current_nodes: [{ id: 'task-plan', name: 'AI任务拆解' }],
      },
    }
    const options = [
      { id: 'product_review', name: '产品评审' },
      { id: 'task_plan', name: 'AI任务拆解' },
    ]
    expect(requirementMatchesCurrentNodeFilter(running, ['product_review'])).toBe(true)
    expect(requirementMatchesCurrentNodeFilter(running, [REQUIREMENT_POOL_NODE_IDLE])).toBe(false)
    expect(requirementMatchesCurrentNodeFilter({}, [REQUIREMENT_POOL_NODE_IDLE])).toBe(true)
    expect(requirementMatchesCurrentNodeFilter({
      workflow_overview: { status: 'completed', current_node_names: [poolCopy.workflowCompleted] },
    }, [REQUIREMENT_POOL_NODE_COMPLETED])).toBe(true)
    expect(requirementMatchesCurrentNodeFilter(running, [])).toBe(true)
    expect(requirementMatchesCurrentNodeFilter(taskPlan, ['task_plan'], options)).toBe(true)
    expect(requirementMatchesCurrentNodeFilter(running, ['task_plan'], options)).toBe(false)
  })
})
