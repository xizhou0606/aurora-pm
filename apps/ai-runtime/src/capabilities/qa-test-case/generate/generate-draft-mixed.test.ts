import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseDraftBatch } from './draft/generate-draft.js';
import { incompleteHighRiskCoverageDraft, omittedHighRiskCoverageDraft } from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('removes an unavailable context reference retained by the first full Draft repair', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const allowedRefs = ['subject:title', 'requirement:0:title', 'task:0:title'];
    const initial = incompleteHighRiskCoverageDraft(['subject:title', 'requirement:0:title']);
    initial.risks[0].contextRefs.push('source:1:content');
    initial.cases[0].riskLinks[0].contextRef = 'source:1:content';
    const stillInvalid = incompleteHighRiskCoverageDraft([...allowedRefs]);
    stillInvalid.risks[0].contextRefs.push('source:1:content');
    stillInvalid.cases[0].riskLinks[0].contextRef = 'source:1:content';
    const final = omittedHighRiskCoverageDraft(
      allowedRefs, '当前输入未提供正向与边界输入的可判定结果',
    );
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];

    const resolved = await generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs,
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify([initial, stillInvalid, final][calls.length - 1]),
          finishReason: 'stop',
          usage: null,
        };
      },
    });

    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair', 'draft_full_repair',
    ]);
    expect(calls[2].messages?.[1]).toEqual({
      role: 'assistant', content: JSON.stringify(stillInvalid),
    });
    expect(calls[2].messages?.[2].content)
      .toContain('risk RISK-SETTLEMENT has unavailable context ref: source:1:content');
    expect(calls[2].messages?.[2].content).toContain(JSON.stringify(allowedRefs));
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      coveragePatched: false,
    });
    expect(JSON.stringify(resolved.output)).not.toContain('source:1:content');
    expect(resolved.output.testCases[0].evidence).toEqual([
      'subject.title',
      'test_case_context.requirements.0.title',
      'test_case_context.tasks.0.title',
    ]);
  });
});
