import { describe, expect, it } from 'bun:test';

import {
  findReleaseAnchor,
  lastChangelogVersion,
  mergeCollectedCommits,
  parseConventionalSubject,
  parseDotEnv,
  parseGitLog,
  parseGitNumstat,
  renderChangelogBlock,
  suggestNextVersion,
} from './changelog-draft-cli.js';

const RS = '\u001e';
const US = '\u001f';

describe('parseConventionalSubject', () => {
  it('解析 type/scope/subject,小写化 type', () => {
    expect(parseConventionalSubject('feat(identity): 目录角色取值调整')).toEqual({
      type: 'feat', scope: 'identity', subject: '目录角色取值调整',
    });
  });

  it('无 scope 与无法解析时降级', () => {
    expect(parseConventionalSubject('docs: 发布说明')).toEqual({ type: 'docs', scope: '', subject: '发布说明' });
    expect(parseConventionalSubject('随手改一下')).toEqual({ type: 'other', scope: '', subject: '随手改一下' });
  });
});

describe('parseGitLog / parseGitNumstat / mergeCollectedCommits', () => {
  const logRaw = [
    `${RS}cb3a7182${US}feat(identity): 目录角色取值调整${US}补充说明${RS}`,
    `929ae281${US}fix(report): 修复导出问题${US}${RS}`,
    '',
  ].join('\n');
  const numstatRaw = [
    `${RS}cb3a7182${US}`,
    '10\t2\tbackend/internal/identity/role.go',
    '3\t1\tapps/web/src/features/report/a.tsx',
    `${RS}929ae281${US}`,
    '-\t-\tassets/logo.bin',
    '2\t0\tREADME.md',
    '',
  ].join('\n');

  it('按分隔符解析提交记录', () => {
    const commits = parseGitLog(logRaw);
    expect(commits).toHaveLength(2);
    expect(commits[0]).toEqual({ sha: 'cb3a7182', subject: 'feat(identity): 目录角色取值调整', body: '补充说明' });
  });

  it('解析 numstat:二进制计零、areas 取前两段目录', () => {
    const summaries = parseGitNumstat(numstatRaw);
    expect(summaries.get('cb3a7182')).toEqual({
      files: 2, insertions: 13, deletions: 3, areas: ['backend/internal', 'apps/web'],
    });
    expect(summaries.get('929ae281')).toEqual({
      files: 2, insertions: 2, deletions: 0, areas: ['assets', 'README.md'],
    });
  });

  it('合并为采集提交,缺 numstat 时用零值', () => {
    const merged = mergeCollectedCommits(parseGitLog(logRaw), parseGitNumstat(numstatRaw));
    expect(merged[0]).toMatchObject({ type: 'feat', scope: 'identity', files: 2, insertions: 13 });
    expect(merged[1]).toMatchObject({ type: 'fix', scope: 'report', files: 2 });
    expect(mergeCollectedCommits(parseGitLog(logRaw), new Map())[2]).toBeUndefined();
    const missing = mergeCollectedCommits([{ sha: 'ffff', subject: 'x', body: '' }], new Map());
    expect(missing[0]).toMatchObject({ type: 'other', files: 0, areas: [] });
  });
});

describe('findReleaseAnchor / lastChangelogVersion / suggestNextVersion', () => {
  it('定位最近一条发布锚点提交', () => {
    const commits = [
      { sha: 'a', subject: 'feat(x): something', body: '' },
      { sha: 'b', subject: 'docs(changelog): 发布 1.4.0 版本公告', body: '' },
      { sha: 'c', subject: 'docs(changelog): 发布 1.3.0 版本公告', body: '' },
    ];
    expect(findReleaseAnchor(commits)?.sha).toBe('b');
    expect(findReleaseAnchor([{ sha: 'a', subject: 'feat(x): y', body: '' }])).toBeNull();
  });

  it('取 CHANGELOG 最新版本号', () => {
    expect(lastChangelogVersion('# 更新日志\n\n## [1.4.0] - 2026-09-08\n\n## [1.3.0]\n')).toBe('1.4.0');
    expect(lastChangelogVersion('没有版本区块')).toBeNull();
  });

  it('feat 存在则 minor,否则 patch;无法解析返回 null', () => {
    const feat = [{ type: 'feat' }, { type: 'fix' }];
    const fixOnly = [{ type: 'fix' }, { type: 'chore' }];
    expect(suggestNextVersion('1.4.0', feat)).toBe('1.5.0');
    expect(suggestNextVersion('1.4.0', fixOnly)).toBe('1.4.1');
    expect(suggestNextVersion('v1.4', fixOnly)).toBeNull();
    expect(suggestNextVersion(null, feat)).toBeNull();
  });
});

describe('renderChangelogBlock', () => {
  it('渲染结果严格命中 CHANGELOG 解析器约定', () => {
    const block = renderChangelogBlock({
      version: '1.5.0',
      date: '2026-09-09',
      summary: ['摘要一', '摘要二'],
      sections: [{ title: '新增', items: ['条目一'] }, { title: '修复', items: [] }],
    });
    expect(block).toBe(
      '## [1.5.0] - 2026-09-09\n'
      + '\n### 更新摘要\n\n- 摘要一\n- 摘要二\n'
      + '\n### 新增\n\n- 条目一\n',
    );
    // 空小节不渲染;版本头与摘要标题可被既有解析规则命中。
    expect(block).toContain('## [1.5.0] - 2026-09-09');
    expect(block).not.toContain('### 修复');
  });
});

describe('parseDotEnv', () => {
  it('解析 KEY=VALUE,跳过注释与非法行,去引号', () => {
    const values = parseDotEnv([
      '# 注释',
      'AI_API_KEY="quoted-value"',
      "AI_MODEL='single'",
      'AI_ENABLED=true',
      'export IGNORED=1',
      '=novalue',
      '  SPACED = padded  ',
    ].join('\n'));
    expect(values).toEqual({
      AI_API_KEY: 'quoted-value',
      AI_MODEL: 'single',
      AI_ENABLED: 'true',
      SPACED: 'padded',
    });
  });
});
