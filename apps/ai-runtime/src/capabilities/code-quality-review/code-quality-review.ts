import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { aiOutputSchema, type AIOutput } from '../../shared/ai/schema.js';

// 代码质量审查技能（code.commit_quality_review）：输入一名成员统计周期内的
// 逐次提交证据（含截断 diff 样本与调试/TODO 命中），输出逐提交结论与综合
// 质量评估。专供汇报链路在叙述生成前调用，结论并入报告输入供 AI 引用。

export const COMMIT_QUALITY_REVIEW_CAPABILITY = 'code.commit_quality_review';

// 技能文件与 lock 清单与 QA 技能（skills/qa-test-case-design）同构：SKILL.md
// 承载审查方法论，lock 记录上游来源（仓库/路径/commit）与内容哈希，启动时
// 校验，保证线上跑的方法论可回溯到具体上游版本。
const CODE_QUALITY_SKILL_NAME = 'code-quality-review';
const codeQualitySkillRoot = fileURLToPath(new URL('../../../skills/code-quality-review/', import.meta.url));
const codeQualitySkillLockPath = fileURLToPath(new URL('../../../skills/code-quality-review.lock.json', import.meta.url));

export type CodeQualityReviewSkillMetadata = {
  name: typeof CODE_QUALITY_SKILL_NAME;
  capability: typeof COMMIT_QUALITY_REVIEW_CAPABILITY;
  version: string;
  packageSha256: string;
  sources: ReadonlyArray<{ name: string; path: string; commit: string; note: string }>;
};

export function codeQualityReviewSkillMetadata(): CodeQualityReviewSkillMetadata {
  const lock = JSON.parse(readFileSync(codeQualitySkillLockPath, 'utf8')) as {
    source: { repositories: CodeQualityReviewSkillMetadata['sources'] };
    files: Record<string, { sha256: string; mode: string }>;
    packageSha256: string;
  };
  const skillMarkdown = readFileSync(codeQualitySkillRoot + 'SKILL.md', 'utf8');
  const sha256 = createHash('sha256').update(skillMarkdown).digest('hex');
  const locked = lock.files?.['SKILL.md'];
  if (!locked || locked.sha256 !== sha256) {
    throw new Error('code-quality-review bundle: SKILL.md content hash differs from lock manifest');
  }
  return {
    name: CODE_QUALITY_SKILL_NAME,
    capability: COMMIT_QUALITY_REVIEW_CAPABILITY,
    version: '2.0.0',
    packageSha256: lock.packageSha256,
    sources: lock.source.repositories,
  };
}

// 方法论正文（SKILL.md 去掉 frontmatter）注入提示词；契约行仍在代码里维护。
function codeQualitySkillMethodology(): string {
  const markdown = readFileSync(codeQualitySkillRoot + 'SKILL.md', 'utf8');
  return markdown.replace(/^---\n[\s\S]*?\n---\n/, '').trim();
}

export const commitQualityVerdicts = ['优', '良', '中', '差'] as const;

// 岗位视角指令：member.roles 来自飞书组织的角色名（如“前端”/“后端”/“测试”）。
// 命中已知岗位给出对应关注点，多岗位命中则叠加为综合视角；全部未命中时退到
// 通用岗位提示；roles 缺失或为空时不输出岗位视角行，保持提示词向后兼容。
const commitQualityRoleFocusRules: ReadonlyArray<{ match: RegExp; focus: string }> = [
  { match: /前端|客户端|web|界面|ui/i, focus: '交互、样式、兼容性、状态管理' },
  { match: /后端|服务端|接口|server/i, focus: '数据一致性、事务、边界条件、权限' },
  { match: /测试|qa|质量/i, focus: '覆盖度与断言质量' },
];

export function commitQualityRoleFocusLine(input: Record<string, unknown>): string | null {
  const subject = input.subject;
  if (!subject || typeof subject !== 'object' || Array.isArray(subject)) return null;
  const member = (subject as Record<string, unknown>).member;
  if (!member || typeof member !== 'object' || Array.isArray(member)) return null;
  const rawRoles = (member as Record<string, unknown>).roles;
  const roles = Array.isArray(rawRoles)
    ? rawRoles.filter((role): role is string => typeof role === 'string' && role.trim().length > 0)
    : [];
  if (roles.length === 0) return null;
  const focuses = commitQualityRoleFocusRules
    .filter((rule) => roles.some((role) => rule.match.test(role)))
    .map((rule) => rule.focus);
  const focusText = focuses.length > 0 ? focuses.join('；') : '结合岗位角色定位评估变更意图与影响面';
  const multi = focuses.length > 1 ? '（多岗位综合判断）' : '';
  return `岗位视角：该成员岗位角色为「${roles.join('、')}」，除通用双轴结论外重点关注：${focusText}${multi}。`;
}

// job_title 辅助视角：member.job_title 是可选的飞书原始职务（由报告主体并行
// 链路写入，字段可能整体缺失、为 null 或非字符串）。非空字符串才输出辅助
// 说明行，缺失或空白时返回 null，保持提示词向后兼容。定位：生效角色（roles）
// 是岗位判断的主依据，飞书职务为辅——当生效角色是系统级权限角色（如
// 「系统管理员」）而非具体岗位时，改以职务判断岗位视角。
export function commitQualityJobTitleFocusLine(input: Record<string, unknown>): string | null {
  const subject = input.subject;
  if (!subject || typeof subject !== 'object' || Array.isArray(subject)) return null;
  const member = (subject as Record<string, unknown>).member;
  if (!member || typeof member !== 'object' || Array.isArray(member)) return null;
  const rawJobTitle = (member as Record<string, unknown>).job_title;
  if (typeof rawJobTitle !== 'string') return null;
  const jobTitle = rawJobTitle.trim();
  if (!jobTitle) return null;
  return `辅助岗位判断：生效角色（roles）为主、飞书职务（job_title）为辅——当生效角色是系统级权限角色（如「系统管理员」）而非具体岗位时，以飞书职务「${jobTitle}」判断岗位视角。`;
}

// 岗位职能判定：用户规则——只有研发类岗位才对代码进行考核；产品/设计/运营/
// 管理等非研发岗位不写代码是正常的，不得因代码产出少或缺失而给负面评价。
// 判定依据：member.roles（生效角色）为主、member.job_title（飞书职务）兜底；
// 生效角色是「系统管理员」等系统级权限角色而非具体岗位时改看职务；两处都
// 无法判断时按研发类处理，保持既有严格审查口径。研发类命中优先：混合命中
// （如「研发经理」「架构设计」）按研发类，避免误弱化真正的开发角色。
const RD_JOB_FUNCTION_PATTERN = /前端|后端|全栈|架构|客户端|开发|研发|算法|测试|qa|工程|技术|程序|dev|engineer|server/i;
const NON_RD_JOB_FUNCTION_PATTERN = /产品|设计|运营|管理|经理|项目|行政|人事|人力|资源|财务|市场|销售|客服|商务|实施|售前|支持/;
const SYSTEM_ROLE_PATTERN = /系统管理|管理员/;

type commitQualityJobFunction = 'rd' | 'non-rd' | 'unknown';

function classifyJobFunctionText(text: string): commitQualityJobFunction {
  if (RD_JOB_FUNCTION_PATTERN.test(text)) return 'rd';
  if (NON_RD_JOB_FUNCTION_PATTERN.test(text)) return 'non-rd';
  return 'unknown';
}

export function commitQualityJobFunctionLine(input: Record<string, unknown>): string | null {
  const subject = input.subject;
  if (!subject || typeof subject !== 'object' || Array.isArray(subject)) return null;
  const member = (subject as Record<string, unknown>).member;
  if (!member || typeof member !== 'object' || Array.isArray(member)) return null;
  const record = member as Record<string, unknown>;
  const rawRoles = record.roles;
  const roles = Array.isArray(rawRoles)
    ? rawRoles.filter((role): role is string => typeof role === 'string' && role.trim().length > 0)
    : [];
  const jobTitle = typeof record.job_title === 'string' ? record.job_title.trim() : '';
  const classify = (text: string) => classifyJobFunctionText(text);
  let jobFunction: commitQualityJobFunction;
  if (roles.length > 0 && roles.every((role) => SYSTEM_ROLE_PATTERN.test(role))) {
    // 系统级权限角色不代表具体岗位，交给职务兜底；职务也缺失则按研发类处理。
    jobFunction = jobTitle ? classify(jobTitle) : 'unknown';
  } else if (roles.length > 0) {
    jobFunction = classify(roles.join(' '));
    // 角色无信号时用职务兜底。
    if (jobFunction === 'unknown' && jobTitle) jobFunction = classify(jobTitle);
  } else {
    jobFunction = jobTitle ? classify(jobTitle) : 'unknown';
  }
  const basisParts: string[] = [];
  if (roles.length > 0) basisParts.push(`生效角色「${roles.join('、')}」`);
  if (jobTitle) basisParts.push(`飞书职务「${jobTitle}」`);
  const basis = basisParts.length > 0 ? `依据${basisParts.join('、')}，` : 'member.roles 与 member.job_title 均缺失，';
  if (jobFunction === 'non-rd') {
    return (
      `岗位职能判定：${basis}判定为非研发类岗位（产品/设计/运营/管理等），代码活动属辅助性质：` +
      '逐提交结论采用「少量/协助性代码变更」等措辞描述，verdict 不得仅因代码量少或无 diff 样本给「差」，' +
      'qualityAssessment.score 不得因其代码产出低而压低；其余维度仍按事实正常评估。'
    );
  }
  if (jobFunction === 'rd') {
    return (
      `岗位职能判定：${basis}判定为研发类岗位（前端/后端/全栈/架构/测试开发等），代码产出属核心工作内容，` +
      '按双轴方法论严格审查，正常给出 verdict 与综合评分。'
    );
  }
  return `岗位职能判定：${basis}无法判断岗位职能，按研发类岗位处理，代码按双轴方法论严格审查，正常给出 verdict 与综合评分。`;
}

export function buildCommitQualityReviewPrompt(input: Record<string, unknown>): string {
  const commits = Array.isArray(input.subject)
    ? input.subject
    : (input.subject as Record<string, unknown> | undefined);
  const jobFunction = commitQualityJobFunctionLine(input);
  const roleFocus = commitQualityRoleFocusLine(input);
  const jobTitleFocus = commitQualityJobTitleFocusLine(input);
  return [
    '你是资深代码审查技能（双轴审查法）。严格遵循下方 SKILL 方法论，对每一条提交独立出结论：一条不许漏、禁止合并点评、sha 原样回填。',
    ...(jobFunction ? [jobFunction] : []),
    ...(roleFocus ? [roleFocus] : []),
    ...(jobTitleFocus ? [jobTitleFocus] : []),
    '每条 issue 必须带证据：引用 diff_sample 中的具体代码片段或文件路径；没有证据的问题禁止编造。diff_sample 缺失的提交按 subject 与行数给有限判断，并在 issues 标注「无代码样本，按提交元信息判断」。',
    'debug_hits/todo_hits > 0 时必须在 issues 点名（调试残留、TODO 欠账）；单次新增 >800 行的巨型变更必须在 issues 提示拆分。',
    '输出体量硬约束：每条提交的 issues 最多 3 条、每条 ≤40 字；verdict/summary 精炼不展开；保证全部提交的结论都装得下。',
    '只输出一个 JSON 对象，不要 Markdown 代码块和解释文字。顶层字段：summary（一句话总评）、changes（固定空数组）、commitReviews（逐条结论）、qualityAssessment（综合评估：score 0-100 整数且与逐条 verdict 分布一致、conclusion 一段话综合结论、commonIssues 共性问题、risks 风险、suggestions 可执行的改进建议）。全部使用简体中文。',
    '——SKILL 方法论开始——',
    codeQualitySkillMethodology(),
    '——SKILL 方法论结束——',
    `提交证据（逐条全覆盖）：${JSON.stringify(commits ?? input.subject ?? {})}`,
  ].join('\n');
}

// 模型可能把 issues/changes 省略或把 verdict 写成别的说法；这里做宽容归一，
// 归一失败的字段退回安全默认值，不让格式抖动打断汇报生成。扩展字段
// （commitReviews/qualityAssessment）随 AIOutput passthrough 透传给调用方。
export function normalizeCommitQualityReviewOutput(value: unknown): AIOutput {
  const entry = (value ?? {}) as Record<string, unknown>;
  const reviews = Array.isArray(entry.commitReviews) ? entry.commitReviews : [];
  const assessment = (entry.qualityAssessment ?? {}) as Record<string, unknown>;
  const commitReviews = reviews.map((raw) => {
    const review = (raw ?? {}) as Record<string, unknown>;
    const verdict = typeof review.verdict === 'string' ? review.verdict.trim() : '';
    return {
      sha: typeof review.sha === 'string' ? review.sha : '',
      verdict: (commitQualityVerdicts as readonly string[]).includes(verdict)
        ? (verdict as (typeof commitQualityVerdicts)[number])
        : ('中' as (typeof commitQualityVerdicts)[number]),
      summary: typeof review.summary === 'string' ? review.summary : '',
      issues: stringArray(review.issues),
    };
  });
  const qualityAssessment = {
    score: typeof assessment.score === 'number' ? Math.round(assessment.score) : 0,
    conclusion: typeof assessment.conclusion === 'string' ? assessment.conclusion : '',
    commonIssues: stringArray(assessment.commonIssues),
    risks: stringArray(assessment.risks),
    suggestions: stringArray(assessment.suggestions),
  };
  // commitReviews/qualityAssessment 已注册进 aiOutputSchema,parse 不再剥离。
  return aiOutputSchema.parse({
    summary: typeof entry.summary === 'string' && entry.summary.trim() ? entry.summary : '代码质量审查完成',
    changes: [],
    items: [],
    risks: qualityAssessment.risks,
    testCases: [],
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
    commitReviews,
    qualityAssessment,
  });
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

// NoObjectGeneratedError 会携带模型原始文本（text）；从中抠出 JSON 兜底。
export function recoverCommitQualityReviewOutput(error: unknown): AIOutput | null {
  const text = (error as { text?: unknown } | null)?.text;
  if (typeof text !== 'string' || !text.trim()) return null;
  const start = text.indexOf('{');
  const end = text.lastIndexOf('}');
  if (start < 0 || end <= start) return null;
  try {
    return normalizeCommitQualityReviewOutput(JSON.parse(text.slice(start, end + 1)));
  } catch {
    return null;
  }
}
