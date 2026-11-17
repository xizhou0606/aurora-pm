import { describe, expect, it } from 'bun:test';

import {
  buildCommitQualityReviewPrompt,
  commitQualityJobTitleFocusLine,
  normalizeCommitQualityReviewOutput,
} from './code-quality-review.js';

const ROLE_FOCUS_MARK = '岗位视角：';

function subjectInput(member: Record<string, unknown>): Record<string, unknown> {
  return { subject: { member } };
}

describe('commitQualityJobTitleFocusLine', () => {
  it('emits an auxiliary line when job_title is a non-empty string', () => {
    const line = commitQualityJobTitleFocusLine(subjectInput({ name: '张三', job_title: '高级后端工程师' }));
    expect(line).toContain('生效角色（roles）为主');
    expect(line).toContain('飞书职务（job_title）为辅');
    expect(line).toContain('系统级权限角色');
    expect(line).toContain('高级后端工程师');
  });

  it('omits the line when job_title is missing, blank, null or not a string', () => {
    const cases: Array<Record<string, unknown>> = [
      { name: '张三' },
      { name: '张三', job_title: '' },
      { name: '张三', job_title: '   ' },
      { name: '张三', job_title: null },
      { name: '张三', job_title: 42 },
    ];
    for (const member of cases) {
      expect(commitQualityJobTitleFocusLine(subjectInput(member))).toBeNull();
    }
    expect(commitQualityJobTitleFocusLine({})).toBeNull();
    expect(commitQualityJobTitleFocusLine({ subject: 'not-an-object' })).toBeNull();
  });
});

describe('buildCommitQualityReviewPrompt', () => {
  it('keeps the prompt unchanged when job_title is absent (backward compatible)', () => {
    const prompt = buildCommitQualityReviewPrompt(subjectInput({ name: '张三', roles: ['前端'] }));
    expect(prompt).toContain(ROLE_FOCUS_MARK);
    expect(prompt).not.toContain('辅助岗位判断');
  });

  it('appends the job_title auxiliary line right after the role focus line', () => {
    const prompt = buildCommitQualityReviewPrompt(
      subjectInput({ name: '张三', roles: ['系统管理员'], job_title: '高级后端工程师' }),
    );
    const lines = prompt.split('\n');
    const roleIndex = lines.findIndex((line) => line.startsWith(ROLE_FOCUS_MARK));
    const jobTitleIndex = lines.findIndex((line) => line.startsWith('辅助岗位判断：'));
    expect(roleIndex).toBeGreaterThan(0);
    expect(jobTitleIndex).toBe(roleIndex + 1);
    expect(lines[jobTitleIndex]).toContain('高级后端工程师');
  });

  it('still emits the job_title line when roles are empty (member carries only name + job_title)', () => {
    const prompt = buildCommitQualityReviewPrompt(subjectInput({ name: '张三', roles: [], job_title: '测试工程师' }));
    expect(prompt).not.toContain(ROLE_FOCUS_MARK);
    expect(prompt).toContain('辅助岗位判断：');
    expect(prompt).toContain('测试工程师');
  });
});

describe('buildCommitQualityReviewPrompt 岗位职能判定', () => {
  it('非研发角色：提示词包含「非研发/辅助性质」措辞，禁止仅因代码量少给差评或压分', () => {
    const prompt = buildCommitQualityReviewPrompt(
      subjectInput({ name: '张三', roles: ['产品经理'], job_title: '产品经理' }),
    );
    expect(prompt).toContain('非研发类岗位');
    expect(prompt).toContain('辅助性质');
    expect(prompt).toContain('不得仅因代码量少或无 diff 样本给「差」');
    expect(prompt).toContain('不得因其代码产出低而压低');
  });

  it('研发角色或无角色：不弱化，按研发类严格审查', () => {
    // 研发角色：判定为研发类，不含任何弱化措辞。
    const rdPrompt = buildCommitQualityReviewPrompt(subjectInput({ name: '张三', roles: ['前端'] }));
    expect(rdPrompt).toContain('研发类岗位');
    expect(rdPrompt).toContain('按双轴方法论严格审查');
    expect(rdPrompt).not.toContain('非研发类岗位');
    expect(rdPrompt).not.toContain('辅助性质');

    // 系统级权限角色 + 研发职务：按职务判定为研发类，不弱化。
    const systemicPrompt = buildCommitQualityReviewPrompt(
      subjectInput({ name: '张三', roles: ['系统管理员'], job_title: '高级后端工程师' }),
    );
    expect(systemicPrompt).toContain('研发类岗位');
    expect(systemicPrompt).not.toContain('辅助性质');

    // roles/job_title 均缺失：按研发类处理，不弱化。
    const noRolePrompt = buildCommitQualityReviewPrompt(subjectInput({ name: '张三' }));
    expect(noRolePrompt).toContain('按研发类岗位处理');
    expect(noRolePrompt).not.toContain('辅助性质');
  });
});

describe('normalizeCommitQualityReviewOutput contract', () => {
  it('keeps the skill output contract fields {summary, changes, commitReviews, qualityAssessment}', () => {
    const output = normalizeCommitQualityReviewOutput({
      summary: '整体质量良好',
      commitReviews: [{ sha: 'abc123', verdict: '良', summary: '干净', issues: ['样本缺失'] }],
      qualityAssessment: { score: 82, conclusion: '结论', commonIssues: ['共性'], risks: ['风险'], suggestions: ['建议'] },
    });
    expect(output).toMatchObject({
      summary: '整体质量良好',
      changes: [],
      commitReviews: [{ sha: 'abc123', verdict: '良', summary: '干净', issues: ['样本缺失'] }],
      qualityAssessment: { score: 82, conclusion: '结论', commonIssues: ['共性'], risks: ['风险'], suggestions: ['建议'] },
    });
  });
});
