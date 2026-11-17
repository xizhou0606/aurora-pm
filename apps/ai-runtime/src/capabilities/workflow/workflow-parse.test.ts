import { describe, expect, it } from 'bun:test';
import {
  inspectWorkflowOutput,
  parseWorkflowOutput,
  validateAndRepairWorkflowOutput,
  workflowGenerationOptions,
  workflowProposalIssues,
  WORKFLOW_MAX_OUTPUT_TOKENS,
} from './index.js';

describe('parseWorkflowOutput', () => {
  it('normalizes a JSON workflow proposal before schema validation', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '已生成草案', changes: [], items: [], risks: [], humanReviewRequired: true,
      workflowProposal: {
        mode: 'replace', explanation: '增加人工审批', warnings: [],
        definition: {
          nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
          edges: [{ id: 'e1', source: 'start', target: 'end' }],
        },
      },
    }));

    expect(output.workflowProposal?.definition.nodes[0].capability).toBeUndefined();
    expect(output.workflowProposal?.definition.edges[0].loop).toBeUndefined();
  });

  it('accepts common workflow proposal field aliases from compatible models', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '已生成', workflow_proposal: {
        explanation: '生成线性流程',
        nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
        edges: [{ id: 'e1', source: 'start', target: 'end' }],
      },
    }));

    expect(output.workflowProposal?.definition.nodes).toHaveLength(2);
  });

  it('maps descriptive model node aliases onto the workflow allowlist', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '已生成', workflowProposal: { definition: {
        nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'check', type: 'ai_completeness_check', name: '完整性', capability: 'requirement.completeness', config: { critical_gate: false } }, { id: 'end', type: 'end', name: '结束' }],
        edges: [{ id: 'e1', source: 'start', target: 'check' }, { id: 'e2', source: 'check', target: 'end', kind: 'failure' }],
      } },
    }));

    expect(output.workflowProposal?.definition.nodes[1].type).toBe('ai_check');
    expect(output.workflowProposal?.definition.nodes[1].capability).toBe('requirement.completeness');
  });

  it('removes an invented dotted task capability instead of silently mapping it', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '并行拆解任务', workflowProposal: { definition: {
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'split', type: 'parallel_split', name: '并行开始' },
          { id: 'plan', type: 'ai_plan', name: 'AI 任务拆解', capability: 'task.breakdown' },
          { id: 'join', type: 'parallel_join', name: '并行汇聚' },
          { id: 'end', type: 'end', name: '结束' },
        ],
        edges: [
          { id: 'e1', source: 'start', target: 'split' },
          { id: 'e2', source: 'split', target: 'plan' },
          { id: 'e3', source: 'plan', target: 'join', kind: 'success' },
          { id: 'e4', source: 'join', target: 'end' },
        ],
      } },
    }));

    const node = output.workflowProposal?.definition.nodes.find((candidate) => candidate.id === 'plan');
    expect(node?.type).toBe('ai_generate');
    expect(node?.capability).toBeUndefined();
    expect(workflowProposalIssues(output.workflowProposal!.definition))
      .toContain('AI 节点“AI 任务拆解”使用未开放能力：未配置');
  });

  it.each([
    ['ai_extract', 'requirement.task_plan', 'ai_generate'],
    ['ai_classify', 'requirement.completeness', 'ai_check'],
    ['ai_plan', 'requirement.task_plan', 'ai_generate'],
    ['ai_summarize', 'requirement.task_plan', 'ai_generate'],
    ['ai_compare', 'requirement.completeness', 'ai_check'],
  ] as const)('normalizes legacy %s nodes while retaining an approved capability', (type, capability, expectedType) => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: 'AI 节点归一化', workflowProposal: { definition: {
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'ai', type, name: 'AI 处理', capability },
          { id: 'end', type: 'end', name: '结束' },
        ],
        edges: [
          { id: 'e1', source: 'start', target: 'ai' },
          { id: 'e2', source: 'ai', target: 'end' },
        ],
      } },
    }));

    expect(output.workflowProposal?.definition.nodes.find((node) => node.id === 'ai')?.type).toBe(expectedType);
    expect(workflowProposalIssues(output.workflowProposal!.definition)).toEqual([]);
  });

  it.each([
    ['requirement', 'ai_generate', 'requirement.review_pack'],
    ['requirement', 'ai_generate', 'version.scope_proposal'],
    ['requirement', 'ai_check', 'unknown.analysis'],
    ['requirement', 'ai_generate', 'requirement.completeness'],
    ['requirement', 'ai_check', 'requirement.task_plan'],
    ['plan_unit', 'ai_generate', 'version.scope_proposal'],
    ['plan_unit', 'ai_check', 'release.readiness'],
    ['plan_unit', 'ai_check', 'unknown.analysis'],
  ] as const)(
    'removes unopened or mode-mismatched %s workflow-node capability %s instead of replacing its semantics',
    (subjectType, nodeType, capability) => {
      const output = parseWorkflowOutput(JSON.stringify({
        summary: '能力校验', workflowProposal: { definition: {
          nodes: [
            { id: 'start', type: 'start', name: '开始' },
            { id: 'ai', type: nodeType, name: 'AI 处理', capability },
            { id: 'end', type: 'end', name: '结束' },
          ],
          edges: [
            { id: 'e1', source: 'start', target: 'ai' },
            { id: 'e2', source: 'ai', target: 'end' },
          ],
        } },
      }), subjectType);

      const node = output.workflowProposal?.definition.nodes.find((candidate) => candidate.id === 'ai');
      expect(node?.type).toBe(nodeType);
      expect(node?.capability).toBeUndefined();
      const issues = workflowProposalIssues(output.workflowProposal!.definition, undefined, subjectType);
      expect(issues.some((issue) => issue.includes(subjectType === 'plan_unit' && nodeType === 'ai_generate'
        ? '项目/版本流程暂不开放 AI 生成节点'
        : '使用未开放能力：未配置'))).toBe(true);
    },
  );

  it.each([
    ['requirement', 'ai_check', 'requirement.completeness'],
    ['requirement', 'ai_generate', 'requirement.task_plan'],
    ['plan_unit', 'ai_check', 'delivery.risk_monitor'],
  ] as const)('retains the approved %s %s capability', (subjectType, type, capability) => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '已开放能力', workflowProposal: { definition: {
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'ai', type, name: 'AI 处理', capability },
          { id: 'end', type: 'end', name: '结束' },
        ],
        edges: [
          { id: 'e1', source: 'start', target: 'ai' },
          { id: 'e2', source: 'ai', target: 'end' },
        ],
      } },
    }), subjectType);

    expect(output.workflowProposal?.definition.nodes.find((node) => node.id === 'ai')).toMatchObject({ type, capability });
    expect(workflowProposalIssues(output.workflowProposal!.definition, undefined, subjectType)).toEqual([]);
  });

  it('rejects unopened capabilities and type mismatches even when validation is called directly', () => {
    const base = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'ai', type: 'ai_generate', name: 'AI 处理', capability: 'version.scope_proposal' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'ai' },
        { id: 'e2', source: 'ai', target: 'end' },
      ],
    } as Parameters<typeof workflowProposalIssues>[0];

    expect(workflowProposalIssues(base, undefined, 'plan_unit')).toContain('项目/版本流程暂不开放 AI 生成节点：AI 处理');
    expect(workflowProposalIssues({
      ...base,
      nodes: base.nodes.map((node) => node.id === 'ai'
        ? { ...node, type: 'ai_generate', capability: 'requirement.completeness' }
        : node),
    }, undefined, 'requirement')).toContain('AI 节点“AI 处理”的类型与能力不匹配：requirement.completeness');
  });

  it('ignores malformed generic extras when the workflow proposal is valid', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '已生成复杂流程',
      changes: [{ node: 'not-a-field-change' }],
      items: [{ id: 'node-without-item-title' }],
      risks: ['需要人工确认', { invalid: true }],
      workflowProposal: { definition: {
        nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
        edges: [{ id: 'e1', source: 'start', target: 'end' }],
      } },
    }));

    expect(output.changes).toEqual([]);
    expect(output.items).toEqual([]);
    expect(output.risks).toEqual(['需要人工确认']);
    expect(output.workflowProposal?.definition.nodes).toHaveLength(2);
  });

  it('normalizes approval aliases and explicitly marks every edge in a cycle', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: '完整生命周期',
      workflowProposal: { definition: {
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'review', type: 'approval', name: '评审', config: { critical_gate: true } },
          { id: 'fix', type: 'human_task', name: '补充' },
          { id: 'end', type: 'end', name: '结束' },
        ],
        edges: [
          { id: 'e1', source: 'start', target: 'review' },
          { id: 'e2', source: 'review', target: 'end', condition: 'approved' },
          { id: 'e3', source: 'review', target: 'fix', condition: 'rejected' },
          { id: 'e4', source: 'fix', target: 'review' },
        ],
      } },
    }));

    const edges = output.workflowProposal?.definition.edges ?? [];
    expect(edges.find((edge) => edge.id === 'e2')).toMatchObject({ kind: 'approve' });
    expect(edges.find((edge) => edge.id === 'e3')).toMatchObject({ kind: 'reject', loop: true });
    expect(edges.find((edge) => edge.id === 'e4')).toMatchObject({ loop: true });
    expect(edges.find((edge) => edge.id === 'e2')?.condition).toBeUndefined();
  });

  it('adds a safe human fallback when an AI node omits its failure branch', () => {
    const output = parseWorkflowOutput(JSON.stringify({
      summary: 'AI 分类流程',
      workflowProposal: { definition: {
        nodes: [
          { id: 'start', type: 'start', name: '开始' },
          { id: 'classify', type: 'ai_classify', name: 'AI 分类', capability: 'requirement.design_classify' },
          { id: 'end', type: 'end', name: '结束' },
        ],
        edges: [
          { id: 'e1', source: 'start', target: 'classify' },
          { id: 'e2', source: 'classify', target: 'end' },
        ],
      } },
    }));

    const definition = output.workflowProposal?.definition;
    const fallback = definition?.nodes.find((node) => node.id === 'ai-fallback-classify');
    expect(fallback).toMatchObject({ type: 'human_task' });
    expect(definition?.edges.find((edge) => edge.source === 'classify' && edge.kind === 'failure')).toMatchObject({ target: fallback?.id, loop: true });
    expect(definition?.edges.find((edge) => edge.source === fallback?.id && edge.target === 'classify')).toMatchObject({ loop: true });
  });
});
