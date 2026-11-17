import { describe, expect, it } from 'bun:test';

import {
  buildChangelogDraftPrompt,
  changelogDraftSkillMetadata,
  normalizeChangelogDraftOutput,
  recoverChangelogDraftOutput,
} from './changelog-draft.js';

describe('changelogDraftSkillMetadata', () => {
  it('加载 lock 清单并通过 SKILL.md 哈希校验', () => {
    const metadata = changelogDraftSkillMetadata();
    expect(metadata.name).toBe('changelog-draft');
    expect(metadata.capability).toBe('changelog.draft');
    expect(metadata.packageSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(metadata.sources[0]?.name).toBe('xizhou0606/pm');
  });
});

describe('buildChangelogDraftPrompt', () => {
  it('包含方法论正文、契约行与提交证据', () => {
    const prompt = buildChangelogDraftPrompt({
      subject: { last_version: '1.4.0', commits: [{ sha: 'cb3a7182', type: 'feat', subject: '目录角色取值调整' }] },
    });
    expect(prompt).toContain('——SKILL 方法论开始——');
    expect(prompt).toContain('——SKILL 方法论结束——');
    expect(prompt).toContain('面向使用者措辞');
    expect(prompt).toContain('禁止照抄提交标题');
    expect(prompt).toContain('cb3a7182');
    expect(prompt).toContain('目录角色取值调整');
  });

  it('subject 缺失时不抛错(证据为空对象)', () => {
    const prompt = buildChangelogDraftPrompt({});
    expect(prompt).toContain('提交证据(发布窗口全覆盖):{}');
  });
});

describe('normalizeChangelogDraftOutput', () => {
  it('保留 summary 与 sections,并写入 changelogDraft 结构', () => {
    const output = normalizeChangelogDraftOutput({
      summary: ['新增汇报模块', '修复导出问题'],
      sections: [
        { title: '新增', items: ['新增代码贡献接入'] },
        { title: '修复', items: ['修复组织同步失败的问题'] },
      ],
    });
    expect(output.changelogDraft).toEqual({
      summary: ['新增汇报模块', '修复导出问题'],
      sections: [
        { title: '新增', items: ['新增代码贡献接入'] },
        { title: '修复', items: ['修复组织同步失败的问题'] },
      ],
    });
    expect(output.humanReviewRequired).toBe(true);
  });

  it('把「更新摘要」小节并入 summary 并去重', () => {
    const output = normalizeChangelogDraftOutput({
      summary: ['新增代码贡献接入'],
      sections: [{ title: '更新摘要', items: ['新增代码贡献接入', '修复同步超时'] }],
    });
    expect(output.changelogDraft?.summary).toEqual(['新增代码贡献接入', '修复同步超时']);
    expect(output.changelogDraft?.sections).toEqual([]);
  });

  it('过滤空标题、空条目与非字符串条目', () => {
    const output = normalizeChangelogDraftOutput({
      summary: ['', '  ', '有效摘要'],
      sections: [
        { title: '', items: ['无效小节'] },
        { title: '调整', items: ['', null, '有效条目'] },
        { title: '修复', items: [] },
        'not-an-object',
      ],
    });
    expect(output.changelogDraft?.summary).toEqual(['有效摘要']);
    expect(output.changelogDraft?.sections).toEqual([{ title: '调整', items: ['有效条目'] }]);
  });

  it('输入为空时退回安全默认值', () => {
    const output = normalizeChangelogDraftOutput(null);
    expect(output.summary).toBe('版本公告草稿生成完成');
    expect(output.changelogDraft).toEqual({ summary: [], sections: [] });
  });
});

describe('recoverChangelogDraftOutput', () => {
  it('从错误携带的模型原始文本中解析 JSON', () => {
    const text = '前言 {"summary":["新增公告草稿技能"],"sections":[]} 后记';
    const output = recoverChangelogDraftOutput({ text });
    expect(output?.changelogDraft?.summary).toEqual(['新增公告草稿技能']);
  });

  it('无文本或无 JSON 时返回 null', () => {
    expect(recoverChangelogDraftOutput(new Error('boom'))).toBeNull();
    expect(recoverChangelogDraftOutput({ text: '完全没有 JSON' })).toBeNull();
    expect(recoverChangelogDraftOutput(null)).toBeNull();
  });
});
