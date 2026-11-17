import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { z } from 'zod';

import { aiOutputSchema, type AIOutput } from '../../shared/ai/schema.js';

// 版本公告草稿技能(changelog.draft):输入一个发布窗口内的 conventional
// commits 证据,输出面向用户的 CHANGELOG.md 区块草稿(更新摘要 + 分节条目)。
// 供维护者发布前经 CLI(changelog:draft)调用,人工润色后并入 CHANGELOG.md;
// 不进入 PM 业务 API,不做任何业务数据分析。

export const CHANGELOG_DRAFT_CAPABILITY = 'changelog.draft';

// 技能文件与 lock 清单与代码质量审查技能(skills/code-quality-review)同构:
// SKILL.md 承载转写方法论,lock 记录来源(本仓自研)与内容哈希,加载时校验,
// 保证线上跑的方法论与仓库版本一致。
const CHANGELOG_DRAFT_SKILL_NAME = 'changelog-draft';
// 模块已从 src/ 迁入 src/capabilities/changelog-draft/,相对深度 +2,
// skills 目录的运行时解析位置与搬迁前保持一致(仍为 <package>/skills/)。
const changelogSkillRoot = fileURLToPath(new URL('../../../skills/changelog-draft/', import.meta.url));
const changelogSkillLockPath = fileURLToPath(new URL('../../../skills/changelog-draft.lock.json', import.meta.url));

export type ChangelogDraftSkillMetadata = {
  name: typeof CHANGELOG_DRAFT_SKILL_NAME;
  capability: typeof CHANGELOG_DRAFT_CAPABILITY;
  version: string;
  packageSha256: string;
  sources: ReadonlyArray<{ name: string; path: string; commit: string; note: string }>;
};

export function changelogDraftSkillMetadata(): ChangelogDraftSkillMetadata {
  const lock = JSON.parse(readFileSync(changelogSkillLockPath, 'utf8')) as {
    source: { repositories: ChangelogDraftSkillMetadata['sources'] };
    files: Record<string, { sha256: string; mode: string }>;
    packageSha256: string;
  };
  const skillMarkdown = readFileSync(changelogSkillRoot + 'SKILL.md', 'utf8');
  const sha256 = createHash('sha256').update(skillMarkdown).digest('hex');
  const locked = lock.files?.['SKILL.md'];
  if (!locked || locked.sha256 !== sha256) {
    throw new Error('changelog-draft bundle: SKILL.md content hash differs from lock manifest');
  }
  return {
    name: CHANGELOG_DRAFT_SKILL_NAME,
    capability: CHANGELOG_DRAFT_CAPABILITY,
    version: '1.0.0',
    packageSha256: lock.packageSha256,
    sources: lock.source.repositories,
  };
}

// 方法论正文(SKILL.md 去掉 frontmatter)注入提示词;契约行仍在代码里维护。
function changelogSkillMethodology(): string {
  const markdown = readFileSync(changelogSkillRoot + 'SKILL.md', 'utf8');
  return markdown.replace(/^---\n[\s\S]*?\n---\n/, '').trim();
}

// 模型结构化输出:更新摘要条目 + 分节条目。宽松上限只防失控,正常区块远小于此。
export const changelogDraftOutputSchema = z.object({
  summary: z.array(z.string().min(1)).min(1).max(10),
  sections: z.array(z.object({
    title: z.string().min(1).max(30),
    items: z.array(z.string().min(1)).min(1).max(30),
  })).max(10),
});

export const CHANGELOG_SUMMARY_TITLE = '更新摘要';

export function buildChangelogDraftPrompt(input: Record<string, unknown>): string {
  const subject = input.subject ?? {};
  return [
    '你是版本公告编辑技能。严格遵循下方 SKILL 方法论,把提交证据转写为面向用户的版本公告草稿:',
    '面向使用者措辞、同主题多提交必须归并、禁止照抄提交标题、禁止出现 sha、内部变更默认不写、不编造。',
    '「修复」小节只允许归纳 type=fix 的提交;窗口内没有 fix 提交时不得输出「修复」小节,不得把功能迭代改写成缺陷修复。',
    '版本号与日期由调用方回填,不要在输出中出现版本号、日期或「本版本」以外的元信息。',
    '只输出一个 JSON 对象,不要 Markdown 代码块和解释文字;顶层字段:summary(更新摘要条目数组)、sections(分节条目数组,标题限「新增/修复/调整」)。全部使用简体中文。',
    '——SKILL 方法论开始——',
    changelogSkillMethodology(),
    '——SKILL 方法论结束——',
    `提交证据(发布窗口全覆盖):${JSON.stringify(subject)}`,
  ].join('\n');
}

// 模型可能把「更新摘要」也当成 section 标题返回,或产出空条目;这里做宽容
// 归一:摘要小节并入 summary、空条目过滤,归一失败退回安全默认,不让格式
// 抖动打断草稿生成。产出走共享 AIOutput 信封,changelogDraft 随信封透传。
export function normalizeChangelogDraftOutput(value: unknown): AIOutput {
  const entry = (value ?? {}) as Record<string, unknown>;
  const summary: string[] = [];
  const sections: Array<{ title: string; items: string[] }> = [];
  const pushItems = (items: unknown) => {
    for (const item of stringArray(items)) if (!summary.includes(item)) summary.push(item);
  };
  pushItems(entry.summary);
  for (const raw of Array.isArray(entry.sections) ? entry.sections : []) {
    const section = (raw ?? {}) as Record<string, unknown>;
    const title = typeof section.title === 'string' ? section.title.trim() : '';
    const items = stringArray(section.items);
    if (!title || !items.length) continue;
    if (title === CHANGELOG_SUMMARY_TITLE) pushItems(items);
    else sections.push({ title, items });
  }
  return aiOutputSchema.parse({
    summary: summary.length ? summary.join(' ') : '版本公告草稿生成完成',
    changes: [],
    items: [],
    risks: [],
    testCases: [],
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
    changelogDraft: {
      summary,
      sections,
    },
  });
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return typeof value === 'string' && value.trim() ? [value.trim()] : [];
  }
  return value
    .flatMap((item) => (typeof item === 'string' ? [item.trim()] : []))
    .filter(Boolean);
}

// NoObjectGeneratedError 会携带模型原始文本(text);从中抠出 JSON 兜底。
export function recoverChangelogDraftOutput(error: unknown): AIOutput | null {
  const text = (error as { text?: unknown } | null)?.text;
  if (typeof text !== 'string' || !text.trim()) return null;
  const start = text.indexOf('{');
  const end = text.lastIndexOf('}');
  if (start < 0 || end <= start) return null;
  try {
    return normalizeChangelogDraftOutput(JSON.parse(text.slice(start, end + 1)));
  } catch {
    return null;
  }
}
