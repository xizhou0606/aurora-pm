import { describe, expect, it } from 'bun:test';
import { completenessAnalysisSchema, normalizeVerifiedCompletenessOutput, normalizeCompletenessOutput } from './completeness.js';
import { validateCapabilityOutput } from '../executor.js';
import { buildCompletenessPrompt } from './prompt.js';
import type { AIOutput } from '../../shared/ai/schema.js';

describe('validateCapabilityOutput', () => {
  it('rejects a completeness report generated for a stale requirement version', () => {
    const output = {
      summary: '报告', changes: [], items: [], risks: ['验收标准缺失'], humanReviewRequired: true, workflowProposal: null,
      completenessReport: {
        verdict: 'blocked', score: 0, threshold: 80, canProceed: false, checkedSubjectVersion: 5,
        dimensions: [], missingRequiredFields: [], blockingIssues: ['验收标准缺失'], recommendations: [],
      },
    } as unknown as AIOutput;

    expect(() => validateCapabilityOutput('requirement.completeness', { subject: { version: 6 } }, output)).toThrow('stale subject version');
  });
});

describe('normalizeCompletenessOutput', () => {
  it('calculates the gate deterministically and never reuses model-provided scores', () => {
    const output = normalizeCompletenessOutput({ subject: { version: 6, description: '需求背景明确', acceptance_criteria: '', product_doc_url: '' } }, {
      summary: '模型分析',
      completenessReport: {
        dimensions: [
          { key: 'background', status: 'pass', findings: ['Background is present'], evidence: ['subject.description'], remediation: ['No remediation needed'] },
          { key: 'goal', status: 'missing', findings: ['缺失'], evidence: [], remediation: ['补充目标'] },
          { key: 'scope', status: 'missing', findings: ['缺失'], evidence: [], remediation: ['补充范围'] },
          { key: 'boundaries', status: 'missing', findings: ['缺失'], evidence: [], remediation: ['补充边界'] },
          { key: 'exceptions', status: 'missing', findings: ['缺失'], evidence: [], remediation: ['补充异常'] },
          { key: 'dependencies', status: 'missing', findings: ['缺失'], evidence: [], remediation: ['补充依赖'] },
          { key: 'acceptance_criteria', status: 'pass', findings: ['模型误判'], evidence: ['subject.acceptance_criteria'], remediation: [] },
        ],
        missingRequiredFields: [], blockingIssues: ['Critical information is missing'], recommendations: ['Rewrite the requirement'],
      },
    });

    expect(output.completenessReport).toMatchObject({ score: 15, threshold: 80, canProceed: false, verdict: 'blocked', checkedSubjectVersion: 6 });
    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'acceptance_criteria')).toMatchObject({ status: 'missing', score: 0 });
    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'background')).toMatchObject({ findings: ['已找到可验证的背景信息'], remediation: [] });
    expect(output.completenessReport?.recommendations).toContain('根据七维检查结果补充缺失的需求信息和可验证证据');
    expect(output.risks).toContain('AI 识别到阻断问题，请补充需求信息后重新检查');
    expect(output.risks).toContain('验收标准缺失，不能进入产品评审');
  });

  it('creates a complete strict fallback report when the provider returns malformed JSON', () => {
    const output = normalizeCompletenessOutput({ subject: { version: 7 } });
    expect(output.completenessReport?.dimensions).toHaveLength(7);
    expect(output.completenessReport).toMatchObject({ score: 0, canProceed: false, verdict: 'blocked', checkedSubjectVersion: 7 });
  });

  it('rejects fabricated evidence paths instead of allowing a false 100-point pass', () => {
    const keys = ['background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria'] as const;
    const output = normalizeCompletenessOutput({
      subject: { version: 9, title: '只有标题', acceptance_criteria: '点击后显示成功' },
    }, {
      summary: '模型误判为完整',
      completenessReport: {
        dimensions: keys.map((key) => ({
          key,
          status: 'pass' as const,
          findings: ['已提供'],
          evidence: [key === 'acceptance_criteria' ? 'subject.acceptance_criteria' : 'subject.not_real'],
          remediation: [],
        })),
        missingRequiredFields: [], blockingIssues: [], recommendations: [],
      },
    });

    expect(output.completenessReport).toMatchObject({ verdict: 'blocked', canProceed: false, score: 20 });
    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'goal')).toMatchObject({
      status: 'missing', evidence: [], findings: ['未提供可验证的目标信息'], remediation: ['补充目标及可验证证据'],
    });
    expect(output.completenessReport?.blockingIssues).toContain('目标缺失，不能进入产品评审');
  });

  it('does not let real but irrelevant subject fields satisfy unrelated dimensions', () => {
    const keys = ['background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria'] as const;
    const output = normalizeCompletenessOutput({
      subject: { version: 9, title: '只有标题', acceptance_criteria: '点击后显示成功' },
    }, {
      summary: '模型误判为完整',
      completenessReport: {
        dimensions: keys.map((key) => ({
          key,
          status: 'pass' as const,
          findings: ['已提供'],
          evidence: [key === 'acceptance_criteria' ? 'subject.acceptance_criteria' : 'subject.title'],
          remediation: [],
        })),
        missingRequiredFields: [], blockingIssues: [], recommendations: [],
      },
    });

    expect(output.completenessReport).toMatchObject({ verdict: 'blocked', canProceed: false, score: 20 });
    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'goal')).toMatchObject({ status: 'missing', evidence: [] });
  });

  it('accepts acceptance criteria evidenced by the primary Feishu document when the form field is empty', () => {
    const output = normalizeCompletenessOutput({
      subject: { version: 8, acceptance_criteria: '', product_doc_url: 'https://acme.feishu.cn/docx/token' },
      sources: [{ type: 'feishu_document', content: '验收标准：给定用户已登录，当提交有效信息时，系统保存并提示成功。' }],
    }, {
      summary: '飞书正文分析',
      completenessReport: {
        dimensions: [
          { key: 'acceptance_criteria', status: 'pass', findings: ['飞书正文包含可验证验收标准'], evidence: ['sources[0].content：验收标准'], remediation: [] },
        ],
        missingRequiredFields: ['acceptance_criteria'], blockingIssues: [], recommendations: [],
      },
    });

    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'acceptance_criteria')).toMatchObject({ status: 'pass', score: 20 });
    expect(output.completenessReport?.missingRequiredFields).not.toContain('acceptance_criteria');
  });

  it('does not treat an unavailable Feishu source as completeness evidence', () => {
    const output = normalizeCompletenessOutput({
      subject: { version: 8, acceptance_criteria: '', product_doc_url: 'https://acme.feishu.cn/docx/token' },
      sources: [{ type: 'feishu_document', available: false, content: '缓存的验收标准' }],
    }, {
      summary: '不可用来源',
      completenessReport: {
        dimensions: [
          { key: 'acceptance_criteria', status: 'pass', findings: ['存在验收标准'], evidence: ['sources[0].content'], remediation: [] },
        ],
        missingRequiredFields: [], blockingIssues: [], recommendations: [],
      },
    });

    expect(output.completenessReport?.dimensions.find((dimension) => dimension.key === 'acceptance_criteria')).toMatchObject({ status: 'missing', score: 0, evidence: [] });
    expect(output.completenessReport).toMatchObject({ verdict: 'blocked', canProceed: false });
  });
});

describe('buildCompletenessPrompt', () => {
  it('asks the model for seven evidence-based dimensions while reserving scoring for code', () => {
    const prompt = buildCompletenessPrompt({ subject: { version: 6, title: '结算优化' } });

    expect(prompt).toContain('dimensions 固定检查且各返回一次');
    expect(prompt).toContain('acceptance_criteria');
    expect(prompt).toContain('找不到证据时 status 必须为 missing');
    expect(prompt).toContain('不要输出分数、门槛、结论或版本，这些由系统按固定规则计算');
    expect(prompt).toContain('所有结论、问题和整改建议必须使用简体中文');
    expect(prompt).toContain('feishu_document 或 internal_document 的 content 是需求事实主来源');
    expect(prompt).toContain('禁止降级为只检查表单字段');
  });
  it('accepts internal body evidence without requiring a Feishu URL', () => {
    const output = normalizeCompletenessOutput({
      subject: { version: 1, acceptance_criteria: '', product_doc_url: '' },
      sources: [{ type: 'internal_document', available: true, content: '验收标准：保存后显示成功。' }],
    }, {
      summary: '站内文档分析',
      completenessReport: {
        dimensions: [{ key: 'acceptance_criteria', status: 'pass', findings: ['正文包含验收标准'], evidence: ['sources[0].content'], remediation: [] }],
        missingRequiredFields: ['product_doc_url', 'acceptance_criteria'], blockingIssues: [], recommendations: [],
      },
    });
    expect(output.completenessReport?.dimensions.find((d) => d.key === 'acceptance_criteria')?.status).toBe('pass');
    expect(output.completenessReport?.missingRequiredFields).toEqual([]);
  });

});

describe('verified production completeness', () => {
  const keys = ['background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria'] as const;
  const analysis = (reference: string) => ({ summary: '检查', completenessReport: {
    dimensions: keys.map(key => ({ key, status: 'partial' as const, findings: ['部分覆盖'], evidence: [reference], remediation: ['继续补充'] })),
    missingRequiredFields: [], blockingIssues: [], recommendations: [],
  } });
  it.each(['internal_document', 'feishu_document'])('keeps valid body evidence and rejects unresolvable citations for %s', type => {
    const input = { subject: { version: 3 }, sources: [{ type, available: true, content: 'AI 接口超时重试 2 次' }] };
    const output = normalizeVerifiedCompletenessOutput(input, analysis('sources[0].content：AI 接口超时重试 2 次'));
    expect(output.completenessReport?.score).toBe(48);
    for (const reference of ['文档异常处理章节', 'sources[0].blocks[1]', 'sources[9].content', 'subject.title']) {
      const fallback = normalizeVerifiedCompletenessOutput(input, analysis(reference));
      expect(fallback.summary).toContain('证据引用校验异常');
      expect(fallback.summary).toContain('本次 0 分为兜底结果');
      expect(fallback.completenessReport).toMatchObject({ score: 0, canProceed: false, checkedSubjectVersion: 3, missingRequiredFields: [] });
      expect(fallback.completenessReport?.dimensions.every(d => d.score === 0 && d.evidence.length === 0)).toBe(true);
      expect(fallback.items.map(item => item.title)).toEqual(['重新运行完整性检查']);
      validateCapabilityOutput('requirement.completeness', input, fallback);
    }
    expect(normalizeVerifiedCompletenessOutput({ ...input, sources: [{ type, available: false, content: '缓存' }] }, analysis('sources[0].content')).summary).toContain('证据引用校验异常');
  });
  it('rejects omitted or duplicated dimensions instead of manufacturing missing dimensions', () => {
    const incomplete = analysis('sources[0].content');
    incomplete.completenessReport.dimensions.pop();
    expect(completenessAnalysisSchema.safeParse(incomplete).success).toBe(false);
    expect(normalizeVerifiedCompletenessOutput({ subject: { version: 3 } }, incomplete).summary).toContain('输出格式校验异常');
    const duplicate = analysis('sources[0].content');
    duplicate.completenessReport.dimensions[6] = duplicate.completenessReport.dimensions[0];
    expect(completenessAnalysisSchema.safeParse(duplicate).success).toBe(false);
    expect(normalizeVerifiedCompletenessOutput({ subject: { version: 3 } }, duplicate).summary).toContain('输出格式校验异常');
  });
  it('specifies citations and recognizes rules outside named sections', () => {
    expect(buildCompletenessPrompt({})).toContain('sources[0].content：AI 接口超时重试 2 次');
    expect(buildCompletenessPrompt({})).toContain('不要因文档没有同名章节');
  });
});

it('returns an explicitly warned zero-score report for malformed model output', async () => {
  const { resolveCapability } = await import('../registry.js');
  const descriptor = resolveCapability('requirement.completeness');
  const input = { subject: { version: 9 }, sources: [{ type: 'internal_document', content: '完整的正文' }] };
  const output = descriptor.noObjectFallback!.recover(new Error('raw provider response must not leak'), { capability: descriptor.capability, input });
  expect(output.summary).toContain('AI 输出格式校验异常');
  expect(output.summary).toContain('本次 0 分为兜底结果');
  expect(output.completenessReport).toMatchObject({ score: 0, verdict: 'blocked', canProceed: false, checkedSubjectVersion: 9, missingRequiredFields: [] });
  expect(output.risks).toEqual([output.summary]);
  expect(output.completenessReport?.recommendations.join()).toContain('重新运行');
  expect(JSON.stringify(output)).not.toContain('raw provider response');
  validateCapabilityOutput('requirement.completeness', input, output);
});

it('preserves a legitimate zero-score analysis without labeling it a generation fallback', () => {
  const dimensions = ['background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria'] as const;
  const input = { subject: { version: 4 } };
  const output = normalizeVerifiedCompletenessOutput(input, {
    summary: '正文内容缺失', completenessReport: {
      dimensions: dimensions.map(key => ({ key, status: 'missing', findings: ['未提供相关信息'], evidence: [], remediation: ['补充需求信息'] })),
      missingRequiredFields: [], blockingIssues: [], recommendations: [],
    },
  });
  expect(output.completenessReport?.score).toBe(0);
  expect(output.summary).not.toContain('兜底');
  expect(output.completenessReport?.blockingIssues).toContain('目标缺失，不能进入产品评审');
  validateCapabilityOutput('requirement.completeness', input, output);
});
