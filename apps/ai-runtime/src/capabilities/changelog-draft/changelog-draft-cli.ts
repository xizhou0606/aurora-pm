// 版本公告草稿 CLI 的纯逻辑:git 输出解析、conventional commit 拆解、版本
// 建议、CHANGELOG 区块渲染、.env 解析。无副作用,便于单测;I/O 见
// scripts/changelog-draft.ts。

export type RawLogCommit = { sha: string; subject: string; body: string };

export type NumstatSummary = {
  files: number;
  insertions: number;
  deletions: number;
  areas: string[];
};

export type CollectedCommit = RawLogCommit & {
  type: string;
  scope: string;
  files: number;
  insertions: number;
  deletions: number;
  areas: string[];
};

const CONVENTIONAL_SUBJECT = /^(\w+)(?:\(([^)]*)\))?:\s*(.+)$/;

// git log --format=%x1e%H%x1f%s%x1f%b%x1e 的记录分隔符。
const RECORD_SEP = '\u001e';
const UNIT_SEP = '\u001f';

export function parseConventionalSubject(subject: string): { type: string; scope: string; subject: string } {
  const match = CONVENTIONAL_SUBJECT.exec(subject.trim());
  if (!match) return { type: 'other', scope: '', subject: subject.trim() };
  return { type: match[1].toLowerCase(), scope: match[2] ?? '', subject: match[3].trim() };
}

export function parseGitLog(raw: string): RawLogCommit[] {
  return raw
    .split(RECORD_SEP)
    .map((record) => record.replace(/^\n/, ''))
    .filter((record) => record.trim())
    .map((record) => {
      const [sha = '', subject = '', body = ''] = record.split(UNIT_SEP);
      return { sha: sha.trim(), subject: subject.trim(), body: body.trim() };
    })
    .filter((commit) => commit.sha && commit.subject);
}

// git log --format=%x1e%H%x1f --numstat 的记录:头部行 \x1e<sha>\x1f,其后
// 每个文件一行 `additions\tdeletions\tpath`(二进制文件计数为 `-`)。
export function parseGitNumstat(raw: string): Map<string, NumstatSummary> {
  const summaries = new Map<string, NumstatSummary>();
  for (const record of raw.split(RECORD_SEP)) {
    const lines = record.split('\n').filter((line) => line.trim());
    const header = lines.find((line) => line.includes(UNIT_SEP));
    if (!header) continue;
    const sha = header.split(UNIT_SEP)[0]?.trim();
    if (!sha) continue;
    const summary: NumstatSummary = { files: 0, insertions: 0, deletions: 0, areas: [] };
    const areaSet = new Set<string>();
    for (const line of lines) {
      if (line === header) continue;
      const [additions = '', deletions = '', path = ''] = line.split('\t');
      summary.files += 1;
      summary.insertions += additions === '-' ? 0 : Number.parseInt(additions, 10) || 0;
      summary.deletions += deletions === '-' ? 0 : Number.parseInt(deletions, 10) || 0;
      const segments = path.trim().split('/').filter(Boolean);
      // area 取目录前两段;顶层单文件(如 README.md)以文件名兜底。
      const directory = segments.slice(0, -1);
      if (directory.length >= 2) areaSet.add(directory.slice(0, 2).join('/'));
      else if (directory.length === 1) areaSet.add(directory[0]);
      else if (segments.length === 1) areaSet.add(segments[0]);
    }
    summary.areas = [...areaSet].slice(0, 6);
    summaries.set(sha, summary);
  }
  return summaries;
}

export function mergeCollectedCommits(
  log: RawLogCommit[],
  numstat: Map<string, NumstatSummary>,
): CollectedCommit[] {
  return log.map((commit) => {
    const conventional = parseConventionalSubject(commit.subject);
    const summary = numstat.get(commit.sha) ?? { files: 0, insertions: 0, deletions: 0, areas: [] };
    return {
      ...commit,
      ...conventional,
      files: summary.files,
      insertions: summary.insertions,
      deletions: summary.deletions,
      areas: summary.areas,
    };
  });
}

// 发布锚点:最近一条 `docs(changelog): 发布 x.y.z 版本公告` 提交。
export function findReleaseAnchor(commits: RawLogCommit[]): RawLogCommit | null {
  const pattern = /^docs\(changelog\):\s*发布/;
  for (const commit of commits) if (pattern.test(commit.subject)) return commit;
  return null;
}

export function lastChangelogVersion(changelog: string): string | null {
  const match = /^##\s+\[([^\]]+)\]/m.exec(changelog);
  return match ? match[1].trim() : null;
}

// 建议:窗口内存在 feat 则 minor,否则 patch;无法解析版本号时返回 null。
export function suggestNextVersion(
  lastVersion: string | null,
  commits: ReadonlyArray<{ type: string }>,
): string | null {
  if (!lastVersion) return null;
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(lastVersion);
  if (!match) return null;
  const [, major, minor, patch] = match;
  if (commits.some((commit) => commit.type === 'feat')) {
    return `${major}.${Number(minor) + 1}.0`;
  }
  return `${major}.${minor}.${Number(patch) + 1}`;
}

export type ChangelogBlockInput = {
  version: string;
  date: string;
  summary: readonly string[];
  sections: ReadonlyArray<{ title: string; items: readonly string[] }>;
};

// 渲染结果必须严格命中 CHANGELOG 解析器约定:`## [版本] - 日期` 区块、
// `### 更新摘要` 摘要、`### 小节` + `- 条目`。
export function renderChangelogBlock(input: ChangelogBlockInput): string {
  const lines: string[] = [`## [${input.version}] - ${input.date}`, '', '### 更新摘要', ''];
  for (const item of input.summary) lines.push(`- ${item}`);
  for (const section of input.sections) {
    if (!section.items.length) continue;
    lines.push('', `### ${section.title}`, '');
    for (const item of section.items) lines.push(`- ${item}`);
  }
  return `${lines.join('\n')}\n`;
}

// 仅填充 process.env 缺失的键;不支持展开/导出等 shell 语法(本仓 .env 不用)。
export function parseDotEnv(text: string): Record<string, string> {
  const values: Record<string, string> = {};
  for (const rawLine of text.split('\n')) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) continue;
    let value = line.slice(eq + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    values[key] = value;
  }
  return values;
}
