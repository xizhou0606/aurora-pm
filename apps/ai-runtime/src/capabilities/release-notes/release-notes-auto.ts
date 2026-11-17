// 版本公告全自动构建链路的纯逻辑:CHANGELOG 区块解析(含 pm:auto 标记)、
// 版本决策、自动区块合并、AI 不可用时的确定性降级草稿。无副作用,I/O 见
// scripts/release-notes-auto.ts。
//
// 无人值守契约:
// - 同一版本区块只写一次(已发布措辞稳定,重复构建不重写);
// - 人工手写的版本区块(无 pm:auto 标记)永不被自动改写;
// - 区块标题行内嵌 `<!-- pm:auto release=<sha> -->`:既是发布锚点链,
//   也不影响 Web 端 parseReleaseNotes 解析(其标题正则允许行尾随内容)。

export type ChangelogBlockMeta = {
  version: string;
  date: string;
  autoReleaseSha: string | null;
  /** 标题行起始偏移。 */
  headingStart: number;
  /** 区块正文结束偏移(下一个区块标题前)。 */
  bodyEnd: number;
};

const BLOCK_HEADING =
  /^##\s+\[([^\]]+)\](?:\s*-\s*(\d{4}-\d{2}-\d{2}))?[^\n]*$/gm;
const RELEASE_MARKER = /<!--\s*pm:auto\s+release=([0-9a-f]{40})\s*-->/;

export function parseChangelogBlocks(changelog: string): ChangelogBlockMeta[] {
  return [...changelog.matchAll(BLOCK_HEADING)].map((match) => {
    const headingStart = match.index ?? 0;
    return {
      version: match[1]!.trim(),
      date: match[2] ?? '',
      autoReleaseSha: RELEASE_MARKER.exec(match[0])?.[1] ?? null,
      headingStart,
      bodyEnd: nextHeadingStart(changelog, headingStart + match[0].length),
    };
  });
}

function nextHeadingStart(text: string, from: number): number {
  const match = /\n##\s/.exec(text.slice(from));
  return match ? from + match.index + 1 : text.length;
}

export function blockForVersion(
  blocks: ChangelogBlockMeta[],
  version: string,
): ChangelogBlockMeta | null {
  return blocks.find((block) => block.version === version) ?? null;
}

export type VersionDecision =
  | { action: 'create'; version: string }
  | { action: 'skip'; reason: string };

// 版本决策:显式指定(PM_RELEASE_VERSION/--version)优先,否则用推导值
// (最新区块版本 + 窗口提交类型,见 suggestNextVersion)。同版本区块默认
// 跳过(写一次保护);仅 --force 且该区块确为自动生成时才允许替换。
export function decideVersion(options: {
  blocks: ChangelogBlockMeta[];
  derived: string | null;
  explicitVersion: string | null;
  force: boolean;
}): VersionDecision {
  const version = options.explicitVersion ?? options.derived;
  if (!version) {
    return { action: 'skip', reason: '无法确定版本号(CHANGELOG 无既有区块且未指定 PM_RELEASE_VERSION)' };
  }
  const existing = blockForVersion(options.blocks, version);
  if (!existing) return { action: 'create', version };
  if (existing.autoReleaseSha && options.force) return { action: 'create', version };
  if (existing.autoReleaseSha) {
    return { action: 'skip', reason: `版本 ${version} 已自动发布(写一次保护);需要重写请加 --force` };
  }
  return { action: 'skip', reason: `版本 ${version} 已有人工区块,自动生成不覆盖` };
}

// 把自动区块插入「维护约定」之后、第一个既有区块之前;仅 --force 时替换
// 同版本自动区块。返回新文件全文。
export function mergeAutoBlock(
  changelog: string,
  rendered: string,
  version: string,
  options: { force?: boolean } = {},
): string {
  const blocks = parseChangelogBlocks(changelog);
  const existing = blockForVersion(blocks, version);
  if (existing) {
    if (!options.force || !existing.autoReleaseSha) {
      throw new Error(`refusing to rewrite existing block ${version}`);
    }
    return `${changelog.slice(0, existing.headingStart)}${rendered.trimEnd()}\n\n${changelog
      .slice(existing.bodyEnd)
      .replace(/^\n+/, '')}`;
  }
  const firstBlock = blocks[0];
  const insertAt = firstBlock ? firstBlock.headingStart : changelog.length;
  const head = changelog.slice(0, insertAt).replace(/\n+$/, '');
  const tail = changelog.slice(insertAt).replace(/^\n+/, '');
  return `${head}\n\n${rendered.trimEnd()}\n\n${tail}`;
}

export type DraftInput = {
  type: string;
  scope: string;
  subject: string;
};

// 会进入公告并触发新版本的内容类型。docs/chore/test/ci/build/style 等内部
// 提交不触发发布——尤其本链路自己的 `docs(changelog): 自动发布 …` 提交,
// 否则每次自动提交都会在下一轮构建里造出一个空版本区块(自我递归)。
export const CONTENT_TYPES: ReadonlySet<string> = new Set(['feat', 'fix', 'perf', 'refactor', 'revert']);

export function contentCommits<T extends { type: string }>(commits: readonly T[]): T[] {
  return commits.filter((commit) => CONTENT_TYPES.has(commit.type));
}

const SECTION_BY_TYPE: Record<string, string> = {
  feat: '新增',
  fix: '修复',
  perf: '调整',
  refactor: '调整',
  revert: '调整',
};

// AI 不可用时的确定性降级:按提交类型分组,提交主题即条目(本仓 conventional
// commits 本身已是面向用户的中文措辞)。chore/docs/test/ci 等内部变更不收录。
export function buildDeterministicDraft(
  commits: readonly DraftInput[],
): { summary: string[]; sections: Array<{ title: string; items: string[] }> } {
  const sections = new Map<string, string[]>();
  const summary: string[] = [];
  for (const commit of commits) {
    const title = SECTION_BY_TYPE[commit.type];
    if (!title) continue;
    const item = commit.scope ? `${commit.subject}(${commit.scope})` : commit.subject;
    const bucket = sections.get(title) ?? [];
    if (!bucket.includes(item)) bucket.push(item);
    sections.set(title, bucket);
    if (summary.length < 6 && !summary.includes(item)) summary.push(item);
  }
  return {
    summary,
    sections: [...sections].map(([title, items]) => ({ title, items })),
  };
}

// 构建期只为 feat/fix 提交采集截断 diff 样本,供 AI「阅读」实际改动。
export const DIFF_SAMPLE_MAX_COMMITS = 40;
export const DIFF_SAMPLE_MAX_CHARS = 600;

export function truncateDiff(text: string): string {
  const trimmed = text.trim();
  return trimmed.length <= DIFF_SAMPLE_MAX_CHARS
    ? trimmed
    : `${trimmed.slice(0, DIFF_SAMPLE_MAX_CHARS)}…(截断)`;
}

// --emit-json 生成-only 模式的上传载荷:字段与 pm-api
// POST {PM_API_URL}/internal/release-notes 契约一一对应,由部署脚本捕获
// stdout 后携带 HMAC 签名转发,公告不再写回仓库。
export type ReleaseNotesEmitPayload = {
  version: string;
  releaseSha: string;
  contentMd: string;
  source: 'autogen';
};

const RELEASE_SHA_PATTERN = /^[0-9a-f]{40}$/;

// releaseSha 取自 git rev-parse HEAD,必须是 40 位十六进制提交号;格式异常
// 直接抛错(fail closed),由调用方按「公告生成失败」降级告警,不阻断部署。
export function buildEmitPayload(input: {
  version: string;
  releaseSha: string;
  contentMd: string;
}): ReleaseNotesEmitPayload {
  const releaseSha = input.releaseSha.trim().toLowerCase();
  if (!RELEASE_SHA_PATTERN.test(releaseSha)) {
    throw new Error(`releaseSha 不是合法的 40 位提交号:${input.releaseSha.slice(0, 12)}…`);
  }
  return {
    version: input.version,
    releaseSha,
    contentMd: input.contentMd,
    source: 'autogen',
  };
}
