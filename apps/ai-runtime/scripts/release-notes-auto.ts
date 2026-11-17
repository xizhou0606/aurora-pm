/**
 * 版本公告全自动生成(release-notes:auto):部署构建前在宿主机运行——
 * 扫描上一发布锚点以来的提交,AI(changelog.draft 技能)阅读提交与 diff
 * 样本后生成 CHANGELOG.md 区块,同步 bump apps/web/package.json 版本并
 * 自动提交(build-and-push 拒绝脏工作区)。AI 不可用或失败时降级为按
 * 提交类型分组的确定性草稿,构建永不因公告生成而中断。
 *
 * 无人值守契约(见 src/release-notes-auto.ts):同版本区块只写一次;
 * 人工区块不覆盖;区块标题内嵌 pm:auto release=<sha> 作为下一轮锚点。
 *
 * --emit-json 生成-only 模式(部署脚本的新默认链路):生成区块后**不写
 * CHANGELOG、不 bump package.json、不 git 提交**,改为向 stdout 输出单行
 * JSON {"version","releaseSha","contentMd","source":"autogen"},由部署脚本
 * POST 到 pm-api 内部接口存储;该模式下所有进度与告警一律走 stderr,
 * stdout 只允许出现最终 JSON。AI 不可用时降级草稿同样以 JSON 输出。
 * --dry-run 与 --emit-json 同为非破坏模式,同时给出时 --emit-json 优先。
 *
 * 用法:pnpm --filter @pm/ai-runtime release-notes:auto
 *   [--emit-json] [--version x.y.z] [--force] [--no-ai] [--no-commit] [--dry-run]
 * 环境开关:PM_RELEASE_NOTES_AUTO=false 整体停用(退出 0)。
 */
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { CHANGELOG_DRAFT_CAPABILITY } from '../src/capabilities/changelog-draft/index.js';
import {
  findReleaseAnchor,
  mergeCollectedCommits,
  parseGitLog,
  parseGitNumstat,
  renderChangelogBlock,
  suggestNextVersion,
  type CollectedCommit,
} from '../src/capabilities/changelog-draft/index.js';
import { loadConfig } from '../src/config.js';
import { createGenerator } from '../src/capabilities/registry.js';
import {
  buildDeterministicDraft,
  buildEmitPayload,
  contentCommits,
  decideVersion,
  DIFF_SAMPLE_MAX_CHARS,
  DIFF_SAMPLE_MAX_COMMITS,
  mergeAutoBlock,
  parseChangelogBlocks,
  truncateDiff,
} from '../src/capabilities/release-notes/index.js';
import { parseDotEnv } from '../src/capabilities/changelog-draft/index.js';

const MAX_COMMITS = 300;
const MAX_BODY_CHARS = 400;

type Args = {
  force: boolean;
  noAi: boolean;
  noCommit: boolean;
  dryRun: boolean;
  emitJson: boolean;
  version: string | null;
};

function parseArgs(argv: string[]): Args {
  const args: Args = {
    force: false,
    noAi: false,
    noCommit: false,
    dryRun: false,
    emitJson: false,
    version: null,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (value === '--force') args.force = true;
    else if (value === '--no-ai') args.noAi = true;
    else if (value === '--no-commit') args.noCommit = true;
    else if (value === '--dry-run') args.dryRun = true;
    else if (value === '--emit-json') args.emitJson = true;
    // pnpm run 会把 `--` 原样透传给脚本(`release-notes:auto -- --emit-json`),
    // 这里跳过裸 `--` 以兼容两种调用形式。
    else if (value === '--') continue;
    else if (value === '--version') args.version = argv[++index] ?? null;
    else throw new Error(`无法识别的参数:${value}`);
  }
  return args;
}

// --emit-json 模式下 stdout 只允许出现最终 JSON 载荷:进度与告警改走 stderr,
// 否则部署脚本捕获的 stdout 无法按 JSON 解析。非 emit-json 模式行为不变。
function note(message: string, emitJson: boolean): void {
  (emitJson ? process.stderr : process.stdout).write(message);
}

function git(root: string, args: string[]): string {
  return execFileSync('git', ['-C', root, '-c', 'core.quotepath=false', ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
}

function todayInShanghai(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date());
}

function truncate(text: string, max: number): string {
  const trimmed = text.trim();
  return trimmed.length <= max ? trimmed : `${trimmed.slice(0, max)}…(截断)`;
}

function collectCommits(root: string, from: string): Array<CollectedCommit & { diff_sample?: string }> {
  const commits = mergeCollectedCommits(
    parseGitLog(git(root, ['log', '--no-merges', '--format=%x1e%H%x1f%s%x1f%b%x1e', `${from}..HEAD`])),
    parseGitNumstat(git(root, ['log', '--no-merges', '--format=%x1e%H%x1f', '--numstat', `${from}..HEAD`])),
  ).slice(0, MAX_COMMITS);
  // diff 样本只给 feat/fix 且有上限,控制 AI 阅读的体量。
  const sampleTargets = commits.filter((commit) => ['feat', 'fix'].includes(commit.type))
    .slice(0, DIFF_SAMPLE_MAX_COMMITS);
  const samples = new Map(sampleTargets.map((commit) => [commit.sha, truncate(
    git(root, ['show', '--format=', '--unified=1', '--no-color', commit.sha]),
    DIFF_SAMPLE_MAX_CHARS,
  )]));
  return commits.map((commit) => ({ ...commit, body: truncate(commit.body, MAX_BODY_CHARS) }))
    .map((commit) => ({ ...commit, diff_sample: samples.get(commit.sha) }));
}

function releaseAnchor(root: string, changelog: string): string | null {
  const blocks = parseChangelogBlocks(changelog);
  if (blocks[0]?.autoReleaseSha) return blocks[0].autoReleaseSha;
  const anchor = findReleaseAnchor(parseGitLog(git(root, ['log', '--format=%x1e%H%x1f%s%x1e', '-n', '1000'])));
  return anchor?.sha ?? null;
}

async function generateDraft(
  root: string,
  version: string,
  lastVersion: string | null,
  commits: CollectedCommit[],
  useAi: boolean,
  emitJson: boolean,
): Promise<{ summary: string[]; sections: Array<{ title: string; items: string[] }>; engine: 'ai' | 'deterministic' }> {
  const input = {
    subject: {
      last_version: lastVersion ?? '',
      suggested_version: version,
      from: 'release-anchor',
      to: 'HEAD',
      commits,
    },
  };
  if (useAi && process.env.AI_API_KEY) {
    try {
      const generate = createGenerator(loadConfig(process.env));
      const result = await generate({
        runId: Math.floor(Date.now() / 1000),
        workspaceId: 1,
        capability: CHANGELOG_DRAFT_CAPABILITY,
        input,
      });
      const draft = result.output.changelogDraft;
      if (draft?.summary.length) {
        const usage = (result.usage ?? {}) as { inputTokens?: number; outputTokens?: number };
        note(`AI 生成:输入 ${usage.inputTokens ?? '?'} / 输出 ${usage.outputTokens ?? '?'} tokens\n`, emitJson);
        return { summary: draft.summary, sections: draft.sections, engine: 'ai' };
      }
      note('AI 返回空草稿,降级为确定性生成\n', emitJson);
    } catch (error) {
      note(`AI 生成失败,降级为确定性生成:${error instanceof Error ? error.message : String(error)}\n`, emitJson);
    }
  }
  return { ...buildDeterministicDraft(commits), engine: 'deterministic' };
}

function withReleaseMarker(rendered: string, releaseSha: string): string {
  return rendered.replace(
    /^## \[([^\]]+)\] - (\d{4}-\d{2}-\d{2})/,
    `## [$1] - $2 <!-- pm:auto release=${releaseSha} -->`,
  );
}

function bumpPackageVersion(packageJsonPath: string, version: string): boolean {
  const source = readFileSync(packageJsonPath, 'utf8');
  if (JSON.parse(source).version === version) return false;
  const updated = source.replace(/"version":\s*"[^"]*"/, `"version": "${version}"`);
  JSON.parse(updated); // 替换后必须仍是合法 JSON
  writeFileSync(packageJsonPath, updated);
  return true;
}

// 根 .env 仅在进程环境缺 AI_API_KEY 时补齐(部署脚本已 set -a 导入 .env,
// 此处兜底保证独立运行也能走 AI 路径);已导出的变量优先。
function loadDotEnv(root: string): void {
  if (process.env.AI_API_KEY) return;
  const envPath = join(root, '.env');
  if (!existsSync(envPath)) return;
  const values = parseDotEnv(readFileSync(envPath, 'utf8'));
  for (const [key, value] of Object.entries(values)) {
    if (!(key in process.env)) process.env[key] = value;
  }
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));
  const emit = (message: string): void => note(message, args.emitJson);
  if ((process.env.PM_RELEASE_NOTES_AUTO ?? '').trim().toLowerCase() === 'false') {
    emit('PM_RELEASE_NOTES_AUTO=false,跳过自动版本公告\n');
    return;
  }
  const root = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
  loadDotEnv(root);
  const changelogPath = join(root, 'CHANGELOG.md');
  const changelog = readFileSync(changelogPath, 'utf8');
  const blocks = parseChangelogBlocks(changelog);
  const lastVersion = blocks[0]?.version ?? null;

  const anchor = releaseAnchor(root, changelog);
  if (!anchor) {
    emit('无发布锚点(区块无 pm:auto 标记且无 docs(changelog): 发布 提交),跳过自动版本公告\n');
    return;
  }
  const explicitVersion = args.version ?? process.env.PM_RELEASE_VERSION?.trim() ?? null;
  // 只看内容型提交(feat/fix/perf/refactor/revert):内部提交——包括本链路
  // 自己的 docs(changelog) 自动提交——不触发新版本,阻断自我递归。
  const commits = contentCommits(collectCommits(root, anchor));
  if (!commits.length) {
    emit('发布窗口内没有可归档的常规提交,跳过\n');
    return;
  }
  const derived = suggestNextVersion(lastVersion, commits);
  const decision = decideVersion({ blocks, derived, explicitVersion, force: args.force });
  if (decision.action === 'skip') {
    emit(`跳过自动版本公告:${decision.reason}\n`);
    return;
  }
  const version = decision.version;

  const draft = await generateDraft(root, version, lastVersion, commits, !args.noAi, args.emitJson);
  const headSha = git(root, ['rev-parse', 'HEAD']).trim();
  const rendered = withReleaseMarker(
    renderChangelogBlock({ version, date: todayInShanghai(), summary: draft.summary, sections: draft.sections }),
    headSha,
  );

  if (args.emitJson) {
    // 生成-only:仓库保持不动(不写 CHANGELOG、不 bump package.json、不提交),
    // 公告由部署脚本携带 HMAC 签名 POST 到 pm-api 存储;stdout 仅此一行 JSON。
    emit(`已生成版本公告 ${version}(${draft.engine}),输出 JSON 待上传\n`);
    process.stdout.write(`${JSON.stringify(buildEmitPayload({ version, releaseSha: headSha, contentMd: rendered }))}\n`);
    return;
  }

  const merged = mergeAutoBlock(changelog, rendered, version, { force: args.force });
  const packageJsonPath = join(root, 'apps', 'web', 'package.json');

  if (args.dryRun) {
    process.stdout.write(`[dry-run] 将写入 ${version}(${draft.engine}),不落盘不提交\n\n${rendered}`);
    return;
  }
  writeFileSync(changelogPath, merged);
  const packageBumped = bumpPackageVersion(packageJsonPath, version);
  process.stdout.write(`已写入版本区块 ${version}(${draft.engine};${packageBumped ? 'apps/web package.json 已同步 bump' : 'package.json 版本未变化'})\n`);

  if (args.noCommit) {
    process.stdout.write('--no-commit:留待人工检查,未自动提交\n');
    return;
  }
  const dirty = git(root, ['status', '--porcelain', '--', 'CHANGELOG.md', 'apps/web/package.json']);
  if (!dirty.trim()) {
    process.stdout.write('无待提交变更\n');
    return;
  }
  try {
    git(root, ['add', 'CHANGELOG.md', 'apps/web/package.json']);
    git(root, ['commit', '-m', `docs(changelog): 自动发布 ${version} 版本公告`]);
    process.stdout.write(`已提交:docs(changelog): 自动发布 ${version} 版本公告\n`);
  } catch (error) {
    // 无人值守兜底:agent 缺 git identity 等原因提交失败时,还原本次写入,
    // 保持工作区干净——build-and-push 拒绝脏树,不能让公告生成卡死部署。
    process.stderr.write(`警告:自动提交失败,已还原本次公告写入(本次镜像沿用既有公告):${error instanceof Error ? error.message : String(error)}\n`);
    try {
      git(root, ['restore', '--source=HEAD', '--staged', '--worktree', '--', 'CHANGELOG.md', 'apps/web/package.json']);
    } catch {
      // 连还原都失败时只能交给上层告警;build-and-push 的脏树检查会兜住。
    }
  }
}

main().catch((error) => {
  process.stderr.write(`release-notes:auto 失败:${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
