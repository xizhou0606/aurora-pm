/**
 * 版本公告草稿生成器(changelog:draft):扫描上一条发布锚点提交以来的
 * conventional commits,调用本仓 AI 技能 changelog.draft 生成面向用户的
 * CHANGELOG.md 区块草稿,供人工润色后并入根目录 CHANGELOG.md。
 *
 * 用法:
 *   pnpm changelog:draft [--from <ref>] [--to <ref>] [--version x.y.z]
 *                         [--date YYYY-MM-DD] [--out <file>]
 * 默认从最近一条 `docs(changelog): 发布 …` 提交扫到 HEAD;AI 配置读根目录 .env。
 */
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { CHANGELOG_DRAFT_CAPABILITY } from '../src/capabilities/changelog-draft/index.js';
import {
  findReleaseAnchor,
  lastChangelogVersion,
  mergeCollectedCommits,
  parseDotEnv,
  parseGitLog,
  parseGitNumstat,
  renderChangelogBlock,
  suggestNextVersion,
  type CollectedCommit,
} from '../src/capabilities/changelog-draft/index.js';
import { loadConfig } from '../src/config.js';
import { createGenerator } from '../src/capabilities/registry.js';

const MAX_COMMITS = 300;
const MAX_BODY_CHARS = 400;

function parseArgs(argv: string[]): Record<string, string> {
  const args: Record<string, string> = {};
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    if (!key?.startsWith('--') || index + 1 >= argv.length) {
      throw new Error(`无法识别的参数:${key ?? '<空>'}(形如 --version 1.5.0)`);
    }
    args[key.slice(2)] = argv[index + 1] ?? '';
  }
  return args;
}

function git(root: string, args: string[]): string {
  return execFileSync('git', ['-C', root, '-c', 'core.quotepath=false', ...args], {
    encoding: 'utf8',
    maxBuffer: 64 * 1024 * 1024,
  });
}

function repoRoot(): string {
  return execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
}

// 根 .env 仅在进程环境缺 AI_API_KEY 时补齐,已导出的变量优先。
function loadDotEnv(root: string): void {
  if (process.env.AI_API_KEY) return;
  const envPath = join(root, '.env');
  if (!existsSync(envPath)) return;
  const values = parseDotEnv(readFileSync(envPath, 'utf8'));
  for (const [key, value] of Object.entries(values)) {
    if (!(key in process.env)) process.env[key] = value;
  }
}

function truncate(text: string, max: number): string {
  const trimmed = text.trim();
  return trimmed.length <= max ? trimmed : `${trimmed.slice(0, max)}…(截断)`;
}

function todayInShanghai(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai' }).format(new Date());
}

function collectCommits(root: string, from: string, to: string): CollectedCommit[] {
  const range = `${from}..${to}`;
  const log = parseGitLog(git(root, ['log', '--no-merges', '--format=%x1e%H%x1f%s%x1f%b%x1e', range]));
  const numstat = parseGitNumstat(git(root, ['log', '--no-merges', '--format=%x1e%H%x1f', '--numstat', range]));
  return mergeCollectedCommits(log, numstat)
    .slice(0, MAX_COMMITS)
    .map((commit) => ({ ...commit, body: truncate(commit.body, MAX_BODY_CHARS) }));
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));
  const root = repoRoot();
  loadDotEnv(root);

  let from = args.from ?? '';
  if (!from) {
    const anchor = findReleaseAnchor(parseGitLog(git(root, ['log', '--format=%x1e%H%x1f%s%x1e', '-n', '1000'])));
    if (!anchor) {
      throw new Error('未找到发布锚点提交(docs(changelog): 发布 …),请用 --from 指定起点');
    }
    from = anchor.sha;
  }
  const to = args.to ?? 'HEAD';
  const commits = collectCommits(root, from, to);
  if (!commits.length) {
    throw new Error(`区间 ${from}..${to} 内没有可用的常规提交`);
  }

  const changelogPath = join(root, 'CHANGELOG.md');
  const lastVersion = lastChangelogVersion(readFileSync(changelogPath, 'utf8'));
  const version = args.version ?? suggestNextVersion(lastVersion, commits);
  if (!version) {
    throw new Error(`无法从 CHANGELOG.md(最新版本:${lastVersion ?? '无'})推断版本号,请用 --version 指定`);
  }
  const date = args.date ?? todayInShanghai();

  const config = loadConfig(process.env);
  const generate = createGenerator(config);
  const result = await generate({
    runId: Math.floor(Date.now() / 1000),
    workspaceId: 1,
    capability: CHANGELOG_DRAFT_CAPABILITY,
    input: {
      subject: {
        last_version: lastVersion ?? '',
        suggested_version: version,
        from,
        to,
        commits,
      },
    },
  });
  const draft = result.output.changelogDraft;
  if (!draft || !draft.summary.length) {
    throw new Error('模型未返回有效的公告草稿');
  }

  const block = renderChangelogBlock({ version, date, summary: draft.summary, sections: draft.sections });
  const outPath = args.out ?? join(root, 'CHANGELOG.draft.md');
  writeFileSync(outPath, block);

  const usage = (result.usage ?? {}) as { inputTokens?: number; outputTokens?: number };
  process.stdout.write(
    [
      `扫描区间:${from.slice(0, 12)}..${to}(${commits.length} 条提交)`,
      `建议版本:${version}${args.version ? '(指定)' : `(基于上一版本 ${lastVersion ?? '?'})`}`,
      `Token 用量:输入 ${usage.inputTokens ?? '?'} / 输出 ${usage.outputTokens ?? '?'}`,
      '',
      block,
      `草稿已写入:${outPath}`,
      '人工润色后,把区块粘贴进 CHANGELOG.md 的「## 更新日志」维护约定之后并删除本文件。',
    ].join('\n'),
  );
}

main().catch((error) => {
  process.stderr.write(`changelog:draft 失败:${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
