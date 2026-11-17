import { describe, expect, it } from 'bun:test';
import {
  inspectWorkflowOutput,
  parseWorkflowOutput,
  validateAndRepairWorkflowOutput,
  workflowGenerationOptions,
  workflowProposalIssues,
  WORKFLOW_MAX_OUTPUT_TOKENS,
} from './index.js';

describe('workflowProposalIssues', () => {
  it('rejects fake parallel fan-out and dead nodes', () => {
    const issues = workflowProposalIssues({
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'review', type: 'approval', name: '产品评审' },
        { id: 'plan', type: 'human_task', name: '任务拆解' },
        { id: 'ui', type: 'condition', name: '是否需要 UI' },
        { id: 'orphan', type: 'human_task', name: '悬空任务' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'review' },
        { id: 'e2', source: 'review', target: 'plan' },
        { id: 'e3', source: 'review', target: 'ui' },
        { id: 'e4', source: 'plan', target: 'end' },
        { id: 'e5', source: 'ui', target: 'end' },
      ],
    });

    expect(issues).toContain('节点“产品评审”使用多条向前连线伪装并行');
    expect(issues).toContain('节点“悬空任务”没有前驱');
    expect(issues).toContain('节点“悬空任务”无法到达结束节点');
  });

  it('accepts an explicit split and join with two converging lanes', () => {
    expect(workflowProposalIssues({
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'split', type: 'parallel_split', name: '并行启动' },
        { id: 'plan', type: 'human_task', name: '任务拆解' },
        { id: 'ui', type: 'condition', name: '是否需要 UI' },
        { id: 'join', type: 'parallel_join', name: '并行汇聚' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'split' },
        { id: 'e2', source: 'split', target: 'plan' },
        { id: 'e3', source: 'split', target: 'ui' },
        { id: 'e4', source: 'plan', target: 'join' },
        { id: 'e5', source: 'ui', target: 'join' },
        { id: 'e6', source: 'join', target: 'end' },
      ],
    })).toEqual([]);
  });

  it.each([
    ['missing start', {
      nodes: [{ id: 'condition', type: 'condition', name: '条件' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'condition', target: 'end' }, { id: 'e2', source: 'end', target: 'condition', loop: true }],
    }],
    ['multiple starts', {
      nodes: [{ id: 's1', type: 'start', name: '开始1' }, { id: 's2', type: 'start', name: '开始2' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 's1', target: 'end' }, { id: 'e2', source: 's2', target: 'end' }],
    }],
    ['dangling edge', {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'ghost' }, { id: 'e2', source: 'ghost', target: 'end' }],
    }],
    ['unreachable node', {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'orphan', type: 'human_task', name: '孤岛' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }, { id: 'e2', source: 'ghost', target: 'orphan' }, { id: 'e3', source: 'orphan', target: 'end' }],
    }],
  ] as const)('rejects %s topology before returning an AI proposal', (_name, definition) => {
    expect(workflowProposalIssues(definition as unknown as Parameters<typeof workflowProposalIssues>[0])).not.toEqual([]);
  });
});
