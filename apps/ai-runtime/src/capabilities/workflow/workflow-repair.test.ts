import { describe, expect, it } from 'bun:test';
import {
  inspectWorkflowOutput,
  parseWorkflowOutput,
  validateAndRepairWorkflowOutput,
  workflowGenerationOptions,
  workflowProposalIssues,
  WORKFLOW_MAX_OUTPUT_TOKENS,
} from './index.js';

describe('workflowGenerationOptions', () => {
  it('uses the fast non-thinking path and caps verbose workflow output', () => {
    expect(workflowGenerationOptions(16000)).toEqual({
      maxOutputTokens: WORKFLOW_MAX_OUTPUT_TOKENS,
      providerOptions: { dashscope: { enable_thinking: false } },
    });
  });
});

describe('inspectWorkflowOutput', () => {
  it('turns malformed JSON into a repairable issue instead of throwing', () => {
    const inspection = inspectWorkflowOutput('{"workflowProposal":');

    expect(inspection.output).toBeNull();
    expect(inspection.issues[0]).toContain('工作流输出无法解析');
  });

  it('turns schema-invalid output into a repairable issue', () => {
    const inspection = inspectWorkflowOutput(JSON.stringify({
      workflowProposal: { definition: { nodes: [{ id: 'start', type: 'start', name: '开始' }], edges: [] } },
    }));

    expect(inspection.output).toBeNull();
    expect(inspection.issues[0]).toContain('工作流输出无法解析');
  });

  it('turns a missing proposal into a repairable issue', () => {
    const inspection = inspectWorkflowOutput(JSON.stringify({ summary: '未生成流程' }));

    expect(inspection.output?.workflowProposal).toBeNull();
    expect(inspection.issues).toEqual(['未生成工作流提案']);
  });

  it.each(['form', 'timer', 'outbox_action', 'create_drafts'] as const)(
    'rejects newly generated %s nodes that the authoring UI does not support',
    (type) => {
      const inspection = inspectWorkflowOutput(JSON.stringify({
        summary: '包含未支持节点', workflowProposal: { definition: {
          nodes: [
            { id: 'start', type: 'start', name: '开始' },
            { id: 'unsupported', type, name: '未支持节点' },
            { id: 'end', type: 'end', name: '结束' },
          ],
          edges: [
            { id: 'e1', source: 'start', target: 'unsupported' },
            { id: 'e2', source: 'unsupported', target: 'end' },
          ],
        } },
      }));

      expect(inspection.output?.workflowProposal?.definition.nodes[1].type).toBe(type);
      expect(inspection.issues).toContain(`节点“未支持节点”使用当前编排器不支持的类型：${type}`);
    },
  );

  const historicalDefinition = {
    nodes: [
      { id: 'start', type: 'start', name: '开始' },
      { id: 'wait', type: 'timer', name: '等待窗口', config: { delay_seconds: 3600, calendar: 'business-day' } },
      { id: 'end', type: 'end', name: '结束' },
    ],
    edges: [
      { id: 'e1', source: 'start', target: 'wait' },
      { id: 'e2', source: 'wait', target: 'end' },
    ],
  };

  it('allows an existing hidden node only when its identity and semantic fields are preserved', () => {
    const inspection = inspectWorkflowOutput(JSON.stringify({
      summary: '保留历史节点', workflowProposal: { definition: historicalDefinition },
    }), 'requirement', historicalDefinition);

    expect(inspection.issues).toEqual([]);
    expect(inspection.output?.workflowProposal?.definition.nodes.find((node) => node.id === 'wait')?.config)
      .toEqual({ delay_seconds: 3600, calendar: 'business-day' });
  });

  it.each([
    ['deletion', {
      nodes: historicalDefinition.nodes.filter((node) => node.id !== 'wait'),
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }, '不得删除'],
    ['type change', {
      ...historicalDefinition,
      nodes: historicalDefinition.nodes.map((node) => node.id === 'wait' ? { ...node, type: 'form' } : node),
    }, '类型必须原样保留'],
    ['type alias change', {
      ...historicalDefinition,
      nodes: historicalDefinition.nodes.map((node) => node.id === 'wait' ? { ...node, type: 'wait_timer' } : node),
    }, '工作流输出无法解析'],
    ['name change', {
      ...historicalDefinition,
      nodes: historicalDefinition.nodes.map((node) => node.id === 'wait' ? { ...node, name: '等待两天' } : node),
    }, '名称必须原样保留'],
    ['config change', {
      ...historicalDefinition,
      nodes: historicalDefinition.nodes.map((node) => node.id === 'wait' ? { ...node, config: { delay_seconds: 7200, calendar: 'business-day' } } : node),
    }, 'config 必须原样保留'],
  ] as const)('rejects historical hidden-node %s', (_case, proposal, expectedIssue) => {
    const inspection = inspectWorkflowOutput(JSON.stringify({
      summary: '错误修改历史节点', workflowProposal: { definition: proposal },
    }), 'requirement', historicalDefinition);

    expect(inspection.issues.some((issue) => issue.includes(expectedIssue))).toBe(true);
  });

  it('rejects a newly introduced hidden node even when another historical hidden node is preserved', () => {
    const proposal = {
      nodes: [
        historicalDefinition.nodes[0], historicalDefinition.nodes[1],
        { id: 'new-form', type: 'form', name: '新增表单' }, historicalDefinition.nodes[2],
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'wait' },
        { id: 'e2', source: 'wait', target: 'new-form' },
        { id: 'e3', source: 'new-form', target: 'end' },
      ],
    };
    const inspection = inspectWorkflowOutput(JSON.stringify({
      summary: '新增隐藏节点', workflowProposal: { definition: proposal },
    }), 'requirement', historicalDefinition);

    expect(inspection.issues).toContain('节点“新增表单”使用当前编排器不支持的类型：form');
  });

  it('still canonicalizes a legacy AI subtype from the current definition', () => {
    const current = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'plan', type: 'ai_plan', name: '任务拆解', capability: 'requirement.task_plan' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'plan' },
        { id: 'e2', source: 'plan', target: 'end' },
      ],
    };
    const inspection = inspectWorkflowOutput(JSON.stringify({
      summary: '归一历史 AI', workflowProposal: { definition: current },
    }), 'requirement', current);

    expect(inspection.issues).toEqual([]);
    expect(inspection.output?.workflowProposal?.definition.nodes.find((node) => node.id === 'plan')?.type).toBe('ai_generate');
  });
});

describe('validateAndRepairWorkflowOutput', () => {
  const validOutput = JSON.stringify({
    summary: '已修复',
    workflowProposal: { definition: {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    } },
  });
  const invalidOutputs = [
    ['parse', '{"workflowProposal":'],
    ['schema', JSON.stringify({ workflowProposal: { definition: { nodes: [{ id: 'start', type: 'start', name: '开始' }], edges: [] } } })],
    ['missing', JSON.stringify({ summary: '未生成流程' })],
    ['topology', JSON.stringify({ workflowProposal: { definition: {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'ghost' }, { id: 'e2', source: 'ghost', target: 'end' }],
    } } })],
    ['unsupported node', JSON.stringify({ workflowProposal: { definition: {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'timer', type: 'timer', name: '等待一天' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'timer' },
        { id: 'e2', source: 'timer', target: 'end' },
      ],
    } } })],
    ['unopened capability', JSON.stringify({ workflowProposal: { definition: {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'ai', type: 'ai_generate', name: '评审材料', capability: 'requirement.review_pack' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'ai' },
        { id: 'e2', source: 'ai', target: 'end' },
      ],
    } } })],
  ] as const;

  it.each(invalidOutputs)('repairs one %s failure and returns only the repaired proposal', async (_kind, firstOutput) => {
    let repairs = 0;
    const resolved = await validateAndRepairWorkflowOutput(
      { text: firstOutput, usage: { outputTokens: 1 } },
      'requirement',
      async (issues, previousOutput) => {
        repairs += 1;
        expect(issues.length).toBeGreaterThan(0);
        expect(previousOutput).toBe(firstOutput);
        return { text: validOutput, usage: { outputTokens: 2 } };
      },
    );

    expect(repairs).toBe(1);
    expect(resolved.output.workflowProposal?.definition.nodes).toHaveLength(2);
    expect(resolved.usage).toEqual({ outputTokens: 2 });
  });

  it('fails closed after exactly one unsuccessful repair', async () => {
    let repairs = 0;
    await expect(validateAndRepairWorkflowOutput(
      { text: '{"workflowProposal":', usage: null },
      'requirement',
      async () => {
        repairs += 1;
        return { text: JSON.stringify({ summary: '仍未生成' }), usage: null };
      },
    )).rejects.toThrow('workflow proposal failed structural validation');
    expect(repairs).toBe(1);
  });

  it('repairs a hidden-node mutation and returns the preserved historical node unchanged', async () => {
    const current = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'wait', type: 'timer', name: '等待窗口', config: { delay_seconds: 3600, calendar: 'business-day' } },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'wait' },
        { id: 'e2', source: 'wait', target: 'end' },
      ],
    };
    const changed = JSON.stringify({
      summary: '错误修改', workflowProposal: { definition: {
        ...current,
        nodes: current.nodes.map((node) => node.id === 'wait' ? { ...node, config: { delay_seconds: 7200 } } : node),
      } },
    });
    const repaired = JSON.stringify({ summary: '已修复', workflowProposal: { definition: current } });

    const resolved = await validateAndRepairWorkflowOutput(
      { text: changed, usage: null },
      'requirement',
      async (issues) => {
        expect(issues.some((issue) => issue.includes('config 必须原样保留'))).toBe(true);
        return { text: repaired, usage: null };
      },
      current,
    );

    expect(resolved.output.workflowProposal?.definition.nodes.find((node) => node.id === 'wait')?.config)
      .toEqual({ delay_seconds: 3600, calendar: 'business-day' });
  });
});
