import { describe, expect, it } from 'bun:test';
import { aiOutputSchema, workflowAuthoringNodeTypeSchema, workflowAuthoringNodeTypes } from './schema.js';

describe('workflow node schemas', () => {
  it('separates the new authoring allowlist from historical definition parsing', () => {
    expect(workflowAuthoringNodeTypeSchema.options).toEqual([...workflowAuthoringNodeTypes]);
    for (const type of ['form', 'timer', 'outbox_action', 'create_drafts', 'ai_plan']) {
      expect(workflowAuthoringNodeTypeSchema.safeParse(type).success).toBe(false);
    }

    const parsed = aiOutputSchema.parse({
      summary: '读取历史流程',
      workflowProposal: {
        definition: {
          nodes: [
            { id: 'start', type: 'start', name: '开始' },
            { id: 'legacy', type: 'form', name: '历史表单', config: { requirement_input: true, legacy_policy: 'preserve' } },
            { id: 'legacy-ai', type: 'ai_plan', name: '历史 AI 规划', capability: 'requirement.task_plan' },
            { id: 'end', type: 'end', name: '结束' },
          ],
          edges: [
            { id: 'e1', source: 'start', target: 'legacy' },
            { id: 'e2', source: 'legacy', target: 'legacy-ai' },
            { id: 'e3', source: 'legacy-ai', target: 'end' },
          ],
        },
      },
    });

    expect(parsed.workflowProposal?.definition.nodes.map((node) => node.type)).toContain('form');
    expect(parsed.workflowProposal?.definition.nodes.map((node) => node.type)).toContain('ai_plan');
    expect(parsed.workflowProposal?.definition.nodes.find((node) => node.id === 'legacy')?.config)
      .toEqual({ requirement_input: true, legacy_policy: 'preserve' });
  });
});

describe('aiOutputSchema completeness report', () => {
  it('requires all seven scored dimensions for a strict completeness report', () => {
    const dimensions = [
      ['background', '背景', 15], ['goal', '目标', 15], ['scope', '范围', 15],
      ['boundaries', '边界', 10], ['exceptions', '异常场景', 15],
      ['dependencies', '依赖', 10], ['acceptance_criteria', '验收标准', 20],
    ].map(([key, label, maxScore]) => ({
      key, label, status: 'pass', score: maxScore, maxScore,
      findings: ['信息明确'], evidence: ['subject.description'], remediation: [],
    }));
    const result = aiOutputSchema.parse({
      summary: '100 分，允许进入评审', changes: [], items: [], risks: [], humanReviewRequired: true,
      completenessReport: {
        verdict: 'pass', score: 100, threshold: 80, canProceed: true,
        checkedSubjectVersion: 6, dimensions, missingRequiredFields: [], blockingIssues: [], recommendations: [],
      },
      workflowProposal: null,
    });

    expect(result.completenessReport?.dimensions).toHaveLength(7);
    expect(result.completenessReport?.canProceed).toBe(true);
  });

  it('rejects an incomplete dimension list', () => {
    expect(() => aiOutputSchema.parse({
      summary: '报告不完整', changes: [], items: [], risks: [], humanReviewRequired: true,
      completenessReport: {
        verdict: 'blocked', score: 0, threshold: 80, canProceed: false,
        checkedSubjectVersion: 6, dimensions: [], missingRequiredFields: [], blockingIssues: [], recommendations: [],
      },
      workflowProposal: null,
    })).toThrow();
  });
});
