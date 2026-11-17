import { describe, expect, it } from 'bun:test';

import {
  blockForVersion,
  buildDeterministicDraft,
  buildEmitPayload,
  contentCommits,
  decideVersion,
  mergeAutoBlock,
  parseChangelogBlocks,
  truncateDiff,
} from './release-notes-auto.js';

const SHA_A = 'a'.repeat(40);
const SHA_B = 'b'.repeat(40);

function changelogFixture(): string {
  return [
    '# 更新日志',
    '',
    '维护约定:',
    '',
    '- 每次发布新增一个区块。',
    '',
    '## [1.5.0] - 2026-09-09 <!-- pm:auto release=' + SHA_A + ' -->',
    '',
    '### 更新摘要',
    '',
    '- 已自动发布的内容',
    '',
    '## [1.4.0] - 2026-09-08',
    '',
    '### 更新摘要',
    '',
    '- 人工写的旧版本',
    '',
  ].join('\n');
}

describe('parseChangelogBlocks', () => {
  it('解析版本、日期与 pm:auto 锚点标记', () => {
    const blocks = parseChangelogBlocks(changelogFixture());
    expect(blocks).toHaveLength(2);
    expect(blocks[0]).toMatchObject({ version: '1.5.0', date: '2026-09-09', autoReleaseSha: SHA_A });
    expect(blocks[1]).toMatchObject({ version: '1.4.0', date: '2026-09-08', autoReleaseSha: null });
    expect(blocks[1].bodyEnd).toBe(changelogFixture().length);
  });
});

describe('blockForVersion / decideVersion', () => {
  const blocks = parseChangelogBlocks(changelogFixture());

  it('按版本定位区块', () => {
    expect(blockForVersion(blocks, '1.5.0')?.autoReleaseSha).toBe(SHA_A);
    expect(blockForVersion(blocks, '9.9.9')).toBeNull();
  });

  it('新版本 → create;同版本自动区块 → 写一次保护;force 才允许替换', () => {
    expect(decideVersion({ blocks, derived: '1.6.0', explicitVersion: null, force: false }))
      .toEqual({ action: 'create', version: '1.6.0' });
    expect(decideVersion({ blocks, derived: '1.5.0', explicitVersion: null, force: false }).action)
      .toBe('skip');
    expect(decideVersion({ blocks, derived: '1.5.0', explicitVersion: null, force: true }))
      .toEqual({ action: 'create', version: '1.5.0' });
  });

  it('人工区块即使 force 也不覆盖;无法推导版本 → skip', () => {
    expect(decideVersion({ blocks, derived: '1.4.0', explicitVersion: null, force: true }).action)
      .toBe('skip');
    expect(decideVersion({ blocks: [], derived: null, explicitVersion: null, force: false }).action)
      .toBe('skip');
  });

  it('显式版本优先于推导值', () => {
    expect(decideVersion({ blocks, derived: '1.6.0', explicitVersion: '2.0.0', force: false }))
      .toEqual({ action: 'create', version: '2.0.0' });
  });
});

describe('mergeAutoBlock', () => {
  it('插入到维护约定之后、第一个既有区块之前', () => {
    const merged = mergeAutoBlock(changelogFixture(), '## [1.6.0] - 2026-09-10\n\n### 更新摘要\n\n- 新内容', '1.6.0');
    expect(merged).toContain('- 每次发布新增一个区块。\n\n## [1.6.0]');
    expect(merged.indexOf('## [1.6.0]')).toBeLessThan(merged.indexOf('## [1.5.0]'));
    expect(merged).toContain('## [1.5.0] - 2026-09-09 <!-- pm:auto release=');
    expect(merged.trimEnd().endsWith('- 人工写的旧版本')).toBe(true);
  });

  it('force 时替换同版本自动区块且不留重复', () => {
    const merged = mergeAutoBlock(
      changelogFixture(),
      '## [1.5.0] - 2026-09-09 <!-- pm:auto release=' + SHA_B + ' -->\n\n### 更新摘要\n\n- 重写内容',
      '1.5.0',
      { force: true },
    );
    expect(merged.match(/## \[1\.5\.0\]/g)).toHaveLength(1);
    expect(merged).toContain('pm:auto release=' + SHA_B);
    expect(merged).toContain('- 重写内容');
    expect(merged).not.toContain('- 已自动发布的内容');
  });

  it('拒绝改写人工区块', () => {
    expect(() => mergeAutoBlock(changelogFixture(), '## [1.4.0] x', '1.4.0', { force: true }))
      .toThrow(/refusing to rewrite/);
  });
});

describe('contentCommits', () => {
  it('只保留内容型提交;内部提交(含自动链路自己的 docs(changelog))不触发发布', () => {
    const commits = [
      { type: 'feat', sha: '1' },
      { type: 'docs', sha: '2' },
      { type: 'fix', sha: '3' },
      { type: 'chore', sha: '4' },
      { type: 'ci', sha: '5' },
      { type: 'revert', sha: '6' },
    ];
    expect(contentCommits(commits).map((commit) => commit.sha)).toEqual(['1', '3', '6']);
  });
});

describe('buildDeterministicDraft', () => {
  it('按类型分节、去重、内部变更不收录', () => {
    const draft = buildDeterministicDraft([
      { type: 'feat', scope: 'identity', subject: '目录角色取值调整' },
      { type: 'feat', scope: 'identity', subject: '目录角色取值调整' },
      { type: 'fix', scope: '', subject: '修复导出失败' },
      { type: 'chore', scope: 'deps', subject: '升级依赖' },
      { type: 'docs', scope: '', subject: '补充文档' },
      { type: 'refactor', scope: 'web', subject: '任务规划抽钩子' },
    ]);
    expect(draft.summary).toEqual(['目录角色取值调整(identity)', '修复导出失败', '任务规划抽钩子(web)']);
    expect(draft.sections).toEqual([
      { title: '新增', items: ['目录角色取值调整(identity)'] },
      { title: '修复', items: ['修复导出失败'] },
      { title: '调整', items: ['任务规划抽钩子(web)'] },
    ]);
  });

  it('全部为内部变更时产出空草稿', () => {
    expect(buildDeterministicDraft([{ type: 'chore', scope: '', subject: 'x' }]))
      .toEqual({ summary: [], sections: [] });
  });
});

describe('truncateDiff', () => {
  it('截断超长 diff 并标注', () => {
    expect(truncateDiff('  short  ')).toBe('short');
    const long = 'x'.repeat(700);
    expect(truncateDiff(long)).toHaveLength(605);
    expect(truncateDiff(long)).toContain('…(截断)');
  });
});

describe('buildEmitPayload', () => {
  it('构建 --emit-json 上传载荷:source 固定 autogen,sha 归一化为 40 位小写十六进制', () => {
    expect(
      buildEmitPayload({
        version: '1.6.0',
        releaseSha: ` ${SHA_A.toUpperCase()} `,
        contentMd: '## [1.6.0] - 2026-09-09\n\n### 更新摘要\n\n- 新内容',
      }),
    ).toEqual({
      version: '1.6.0',
      releaseSha: SHA_A,
      contentMd: '## [1.6.0] - 2026-09-09\n\n### 更新摘要\n\n- 新内容',
      source: 'autogen',
    });
  });

  it('releaseSha 非 40 位提交号时抛错(fail closed,由调用方降级告警)', () => {
    expect(() => buildEmitPayload({ version: '1.6.0', releaseSha: 'abc123', contentMd: 'x' }))
      .toThrow(/releaseSha/);
    expect(() => buildEmitPayload({ version: '1.6.0', releaseSha: '', contentMd: 'x' }))
      .toThrow(/releaseSha/);
  });

  it('载荷 JSON 序列化后为单行(contentMd 换行被转义),可无损往返', () => {
    const payload = buildEmitPayload({
      version: '1.6.0',
      releaseSha: SHA_A,
      contentMd: '## [1.6.0] - 2026-09-09\n\n- 条目一\n- 条目二',
    });
    const serialized = JSON.stringify(payload);
    expect(serialized).not.toContain('\n');
    expect(JSON.parse(serialized)).toEqual(payload);
  });
});
