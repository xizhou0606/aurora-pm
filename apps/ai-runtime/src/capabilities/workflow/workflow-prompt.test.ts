import { describe, expect, it } from 'bun:test';
import { buildWorkflowPrompt } from './prompts/prompt.js';

describe('buildWorkflowPrompt', () => {
  it('asks workflow generation for a canvas proposal that preserves manual edits', () => {
    const prompt = buildWorkflowPrompt({ user_prompt: '增加产品审批', workflow_definition: { nodes: [], edges: [] } });

    expect(prompt).toContain('workflowProposal');
    expect(prompt).toContain('优先保留当前画布');
    expect(prompt).toContain('不要输出无意义的 null 字段');
    expect(prompt).toContain('禁止原样返回当前定义');
    expect(prompt).toContain('需求录入、完整性检查、产品评审');
    expect(prompt).toContain('parallel_split');
    expect(prompt).toContain('节点仅可使用 start、end、human_task、approval、condition、parallel_split、parallel_join、ai_check、ai_generate');
    expect(prompt).toContain('分析、检查、分类、判断、比较类能力使用 ai_check');
    expect(prompt).toContain('内容生成、提取、汇总、规划、任务拆解类能力使用 ai_generate');
    expect(prompt).toContain('需求录入等人工填写步骤使用 human_task');
    expect(prompt).toContain('禁止生成 form、timer、outbox_action、create_drafts');
    expect(prompt).toContain('需求 ai_check 只能使用 requirement.completeness');
    expect(prompt).toContain('需求 ai_generate 只能使用 requirement.task_plan');
    expect(prompt).toContain('项目/版本 ai_check 只能使用 delivery.risk_monitor');
    expect(prompt).toContain('项目/版本流程禁止生成 ai_generate');
    expect(prompt).toContain('version.scope_proposal、requirement.review_pack、workflow.*、qa.test_cases 及其他未列出的 capability 均禁止用于流程节点');
    for (const legacyAIType of ['ai_extract', 'ai_classify', 'ai_plan', 'ai_summarize', 'ai_compare']) {
      expect(prompt).not.toContain(legacyAIType);
    }
    expect(prompt).toContain('禁止用普通节点同时连接多条默认/成功连线来伪装并行');
    expect(prompt.length).toBeLessThan(2500);
  });
});
