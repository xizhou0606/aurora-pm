import { describe, expect, it } from 'bun:test';

import { buildReportPrompt } from './prompt.js';
import { parseReportOutput, parseReportSummary } from './report.js';

describe('report output parsing', () => {
  it('extracts summary, overall comment, overall score and dimension comments', () => {
    const parsed = parseReportOutput(JSON.stringify({
      summary: '# 周报\n一切顺利',
      overall_comment: '整体表现稳健。',
      overall_score: 86,
      dimension_comments: { tasks: '任务短评', quality: '', code: '' },
    }));
    expect(parsed.summary).toBe('# 周报\n一切顺利');
    expect(parsed.overallComment).toBe('整体表现稳健。');
    expect(parsed.overallScore).toBe(86);
    expect(parsed.dimensionComments.tasks).toBe('任务短评');
  });

  it('clamps out-of-range scores and drops non-numeric ones', () => {
    const clamped = parseReportOutput(JSON.stringify({ summary: '正文', overall_score: 250 }));
    expect(clamped.overallScore).toBe(100);
    const negative = parseReportOutput(JSON.stringify({ summary: '正文', overall_score: -3 }));
    expect(negative.overallScore).toBe(0);
    const missing = parseReportOutput(JSON.stringify({ summary: '正文', overall_score: 'high' }));
    expect(missing.overallScore).toBeUndefined();
    const legacy = parseReportOutput(JSON.stringify({ summary: '旧格式' }));
    expect(legacy.overallScore).toBeUndefined();
  });

  it('repairs unescaped inner quotes instead of dumping raw JSON as summary', () => {
    const broken = '{"summary":"优化分工机制，避免成员出现"有需求无产出"的闲置状态。","overall_comment":"评语","overall_score":83,'
      + '"dimension_comments":{"tasks":"","quality":"","code":""}}';
    const parsed = parseReportOutput(broken);
    expect(parsed.summary).toContain('有需求无产出');
    expect(parsed.overallComment).toBe('评语');
    expect(parsed.overallScore).toBe(83);
  });

  it('falls back to the raw text as summary without score', () => {
    const parsed = parseReportOutput('这不是 JSON 的正文');
    expect(parsed.summary).toBe('这不是 JSON 的正文');
    expect(parsed.overallScore).toBeUndefined();
    expect(parseReportSummary('只有正文')).toBe('只有正文');
  });
});

describe('report prompt contract', () => {
  it('requires an overall_score integer in the output contract', () => {
    const prompt = buildReportPrompt({ instructions: '结构要求' });
    expect(prompt).toContain('overall_score');
    expect(prompt).toContain('bug_reports');
    expect(prompt).toContain('requirements');
  });
});
