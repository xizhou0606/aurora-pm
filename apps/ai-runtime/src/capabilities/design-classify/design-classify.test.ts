import { describe, expect, it } from 'bun:test';
import { normalizeDesignClassificationOutput, parseDesignClassificationText } from './design-classify.js';
import { buildDesignClassifyPrompt } from './prompt.js';

describe('design classification generation', () => {
  it('normalizes compatible decision aliases into the generic AI result contract', () => {
    const output = normalizeDesignClassificationOutput({
      data: {
        summary: '涉及新的用户操作界面，需要设计介入',
        ui_needed: 'yes',
        basis: ['subject.description 包含页面操作'],
        missing_materials: ['空状态说明'],
        risk: '未提供响应式规则',
      },
    });

    expect(output.changes).toEqual([]);
    expect(output.items.map((item) => item.title)).toContain('设计介入结论：需要');
    expect(output.items.map((item) => item.title)).toContain('空状态说明');
    expect(output.risks).toEqual(['未提供响应式规则']);
    expect(output.humanReviewRequired).toBe(true);
  });

  it('recovers fenced JSON emitted after model reasoning', () => {
    const output = parseDesignClassificationText(`<think>正在判断</think>
\`\`\`json
{"summary":"纯后端数据修复","requires_design":false,"evidence":["subject.description"],"missing_materials":[],"risks":[]}
\`\`\``);

    expect(output.changes).toEqual([]);
    expect(output.items[0].title).toBe('设计介入结论：暂不需要');
    expect(output.completenessReport).toBeNull();
    expect(output.workflowProposal).toBeNull();
  });

  it('fails safe to explicit human review when no decision can be parsed', () => {
    const output = normalizeDesignClassificationOutput(null);

    expect(output.changes).toEqual([]);
    expect(output.risks[0]).toContain('人工评审');
    expect(output.humanReviewRequired).toBe(true);
  });
});

describe('buildDesignClassifyPrompt', () => {
  it('uses a compact boolean contract for design classification', () => {
    const prompt = buildDesignClassifyPrompt({ subject: { title: '结算优化' } });

    expect(prompt).toContain('summary、requiresDesign、evidence、missingMaterials、risks');
    expect(prompt).toContain('requiresDesign 必须是布尔值');
    expect(prompt).toContain('所有结论都必须由人类确认');
    expect(prompt).not.toContain('workflowProposal');
  });
});
