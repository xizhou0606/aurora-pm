import { describe, expect, it } from 'bun:test';
import { buildGenericPrompt } from './generic-prompt.js';

describe('buildGenericPrompt', () => {
  it('states the exact JSON output contract required by DashScope', () => {
    const prompt = buildGenericPrompt('inbox.daily_brief', { tasks: [] });

    expect(prompt).toContain('JSON 顶层必须且只能包含 summary、changes、items、risks、testCases、humanReviewRequired、completenessReport、workflowProposal');
    expect(prompt).toContain('changes 必须是数组');
    expect(prompt).toContain('暂无可汇总事实');
  });
});
