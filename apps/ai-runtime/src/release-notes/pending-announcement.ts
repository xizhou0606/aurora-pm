// 部署即公告:pm-ai-runtime 容器启动时对挂载的发布仓库(只读)生成版本公告,
// HMAC 签名后 POST 到 pm-api 内部接口入库。提交历史只存在于服务器的 git
// 工作区(挂载点),镜像内没有 .git,因此仓库数据一律来自挂载点而非镜像。
// 与 scripts/release-notes-auto.ts 的 --emit-json 路径保持同一契约:
// 同 version 同 releaseSha 幂等(200),同 version 异 sha 冲突(409)。
// 版本与窗口锚点以 pm-api 已入库的最新公告为准(GET /internal/release-notes/
// latest);挂载的 CHANGELOG.md 是只读的、永远不会前进,只按它推导会永远
// 得到同一个版本号、被幂等契约吞掉。任何一步失败都收敛为 outcome,由入口
// 进程降级为告警,绝不影响服务启动。
import { execFileSync } from 'node:child_process';
import { createHmac } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import {
  CHANGELOG_DRAFT_CAPABILITY,
  findReleaseAnchor,
  mergeCollectedCommits,
  parseGitLog,
  parseGitNumstat,
  renderChangelogBlock,
  suggestNextVersion,
  type CollectedCommit,
} from '../capabilities/changelog-draft/index.js';
import { loadConfig } from '../config.js';
import { createGenerator } from '../capabilities/registry.js';
import {
  buildDeterministicDraft,
  buildEmitPayload,
  contentCommits,
  decideVersion,
  DIFF_SAMPLE_MAX_CHARS,
  DIFF_SAMPLE_MAX_COMMITS,
  parseChangelogBlocks,
} from '../capabilities/release-notes/index.js';

const MAX_COMMITS = 300;
const MAX_BODY_CHARS = 400;

export type OnStartOptions = {
  repoDir: string;
  apiUrl: string;
  secret: string;
  useAi?: boolean;
  dryRun?: boolean;
  env?: NodeJS.ProcessEnv;
  log?: (message: string) => void;
};

export type OnStartOutcome = {
  action: 'skip' | 'uploaded' | 'dry-run' | 'error';
  version?: string;
  reason?: string;
};

export type AutoPublishGate = {
  run: boolean;
  reason?: string;
};

// 只有生产容器才自动发布公告:本地/CI 里挂载点通常不存在,且避免把
// 测试公告写进共享环境。PM_RELEASE_NOTES_AUTO=false 是显式跳过开关。
export function shouldAutoPublish(env: NodeJS.ProcessEnv): AutoPublishGate {
  if (env.PM_RELEASE_NOTES_AUTO === 'false') {
    return { run: false, reason: 'PM_RELEASE_NOTES_AUTO=false,跳过自动版本公告' };
  }
  if (env.PM_ENV !== 'production') {
    return { run: false, reason: `PM_ENV=${env.PM_ENV || '(空)'} 非生产环境,跳过自动版本公告` };
  }
  if (!env.PM_RELEASE_INTERNAL_SECRET && !env.AI_INTERNAL_SECRET) {
    return { run: false, reason: '缺少 AI_INTERNAL_SECRET,无法签名上传版本公告' };
  }
  if (!env.PM_RELEASE_API_URL) {
    return { run: false, reason: '缺少 PM_RELEASE_API_URL,无法上传版本公告' };
  }
  return { run: true };
}

export function signReleasePayload(secret: string, timestampMs: string, payloadJson: string): string {
  return createHmac('sha256', secret).update(`${timestampMs}.${payloadJson}`).digest('hex');
}

function signedHeaders(secret: string, bodyJson: string): Record<string, string> {
  const timestampMs = `${Date.now()}`;
  return {
    'content-type': 'application/json',
    'x-pm-timestamp': timestampMs,
    'x-pm-signature': signReleasePayload(secret, timestampMs, bodyJson),
  };
}

// pm-api 已入库的最新一期公告。启动器以它的 version + releaseSha 为基线
// 推导本轮版本与提交窗口,而不是只看永不前进的 CHANGELOG.md。
export type PublishedNote = { version: string; releaseSha: string };

// GET /internal/release-notes/latest:与上传同一套 HMAC 契约,GET 无 body,
// 签名只覆盖 `${timestamp}.`。非 2xx 一律抛错交由调用方重试/告警——绝不
// 静默回退到冻结的 CHANGELOG 锚点,否则版本号会被永久卡住。
export async function fetchLatestPublishedNote(apiUrl: string, secret: string): Promise<PublishedNote | null> {
  const response = await fetch(`${apiUrl.replace(/\/+$/, '')}/internal/release-notes/latest`, {
    headers: signedHeaders(secret, ''),
  });
  if (!response.ok) {
    throw new Error(`查询最新公告返回 HTTP ${response.status}`);
  }
  const body = (await response.json()) as { data?: { version?: string; release_sha?: string } | null };
  const note = body.data;
  if (!note?.version) return null;
  return { version: note.version, releaseSha: note.release_sha ?? '' };
}

function semverVersion(version: string | null | undefined): string | null {
  const trimmed = version?.trim();
  return trimmed && /^\d+\.\d+\.\d+$/.test(trimmed) ? trimmed : null;
}

function commitExists(repoDir: string, sha: string): boolean {
  try {
    git(repoDir, ['cat-file', '-e', `${sha}^{commit}`]);
    return true;
  } catch {
    return false;
  }
}

// 锚点决策:已入库公告的 releaseSha 优先(它正是上一期公告的窗口终点);
// 该 sha 不在挂载仓库历史里(异常场景)或 pm-api 尚无公告时,退回 CHANGELOG
// 的 pm:auto 标记 / docs(changelog): 发布 提交链。
function resolveAnchor(published: PublishedNote | null, repoDir: string, changelog: string): string | null {
  const sha = published?.releaseSha.trim();
  if (sha && commitExists(repoDir, sha)) return sha;
  return releaseAnchor(repoDir, changelog);
}

function git(repoDir: string, args: string[]): string {
  return execFileSync('git', ['-C', repoDir, '-c', 'core.quotepath=false', ...args], {
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

function collectCommits(repoDir: string, from: string): Array<CollectedCommit & { diff_sample?: string }> {
  const commits = mergeCollectedCommits(
    parseGitLog(git(repoDir, ['log', '--no-merges', '--format=%x1e%H%x1f%s%x1f%b%x1e', `${from}..HEAD`])),
    parseGitNumstat(git(repoDir, ['log', '--no-merges', '--format=%x1e%H%x1f', '--numstat', `${from}..HEAD`])),
  ).slice(0, MAX_COMMITS);
  const sampleTargets = commits.filter((commit) => ['feat', 'fix'].includes(commit.type))
    .slice(0, DIFF_SAMPLE_MAX_COMMITS);
  const samples = new Map(sampleTargets.map((commit) => [commit.sha, truncate(
    git(repoDir, ['show', '--format=', '--unified=1', '--no-color', commit.sha]),
    DIFF_SAMPLE_MAX_CHARS,
  )]));
  return commits.map((commit) => ({ ...commit, body: truncate(commit.body, MAX_BODY_CHARS) }))
    .map((commit) => ({ ...commit, diff_sample: samples.get(commit.sha) }));
}

function releaseAnchor(repoDir: string, changelog: string): string | null {
  const blocks = parseChangelogBlocks(changelog);
  if (blocks[0]?.autoReleaseSha) return blocks[0].autoReleaseSha;
  const anchor = findReleaseAnchor(parseGitLog(git(repoDir, ['log', '--format=%x1e%H%x1f%s%x1e', '-n', '1000'])));
  return anchor?.sha ?? null;
}

async function generateDraft(
  options: OnStartOptions,
  version: string,
  lastVersion: string | null,
  commits: Array<CollectedCommit & { diff_sample?: string }>,
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
  if (options.useAi && options.env?.AI_API_KEY) {
    try {
      const generate = createGenerator(loadConfig(options.env));
      const result = await generate({
        runId: Math.floor(Date.now() / 1000),
        workspaceId: 1,
        capability: CHANGELOG_DRAFT_CAPABILITY,
        input,
      });
      const draft = result.output.changelogDraft;
      if (draft?.summary.length) {
        options.log?.(`    [release-notes] AI 生成完成\n`);
        return { summary: draft.summary, sections: draft.sections, engine: 'ai' };
      }
      options.log?.('    [release-notes] AI 返回空草稿,降级为确定性生成\n');
    } catch (error) {
      options.log?.(`    [release-notes] AI 生成失败,降级为确定性生成:${error instanceof Error ? error.message : String(error)}\n`);
    }
  }
  return { ...buildDeterministicDraft(commits), engine: 'deterministic' };
}

// 挂载的发布仓库属主是部署用户(root),容器内运行身份是 node——
// git ≥2.35 的 dubious ownership 保护会拒绝一切操作。safe.directory
// 只认 system/global 作用域,因此写入独立文件并指到 GIT_CONFIG_GLOBAL。
export function ensureGitSafeDirectory(repoDir: string, env: NodeJS.ProcessEnv = process.env): string {
  const configPath = '/tmp/pm-release-notes-gitconfig';
  writeFileSync(configPath, `[safe]\n\tdirectory = ${repoDir}\n`);
  env.GIT_CONFIG_GLOBAL = configPath;
  return configPath;
}

export async function runReleaseNotesOnStart(options: OnStartOptions): Promise<OnStartOutcome> {
  const { repoDir, apiUrl, secret, log } = options;
  try {
    ensureGitSafeDirectory(repoDir, options.env);
    const changelog = readFileSync(join(repoDir, 'CHANGELOG.md'), 'utf8');
    const blocks = parseChangelogBlocks(changelog);

    const published = await fetchLatestPublishedNote(apiUrl, secret);
    const anchor = resolveAnchor(published, repoDir, changelog);
    if (!anchor) {
      return { action: 'skip', reason: '无发布锚点(pm-api 无已入库公告,且 CHANGELOG 区块无 pm:auto 标记、无 docs(changelog): 发布 提交)' };
    }
    const lastVersion = semverVersion(published?.version) ?? blocks[0]?.version ?? null;

    const collected = mergeCollectedCommits(
      parseGitLog(git(repoDir, ['log', '--no-merges', '--format=%x1e%H%x1f%s%x1f%b%x1e', `${anchor}..HEAD`])),
      parseGitNumstat(git(repoDir, ['log', '--no-merges', '--format=%x1e%H%x1f', '--numstat', `${anchor}..HEAD`])),
    ).slice(0, MAX_COMMITS);
    const commits = contentCommits(collected);
    if (!commits.length) {
      return { action: 'skip', reason: '发布窗口内没有可归档的常规提交' };
    }

    const derived = suggestNextVersion(lastVersion, commits);
    const decision = decideVersion({ blocks, derived, explicitVersion: null, force: false });
    if (decision.action === 'skip') {
      return { action: 'skip', reason: decision.reason };
    }
    const version = decision.version;

    const draft = await generateDraft(options, version, lastVersion, commits);
    const headSha = git(repoDir, ['rev-parse', 'HEAD']).trim();
    const rendered = renderChangelogBlock({
      version,
      date: todayInShanghai(),
      summary: draft.summary,
      sections: draft.sections,
    });
    const payload = buildEmitPayload({ version, releaseSha: headSha, contentMd: withReleaseMarker(rendered, headSha) });
    const payloadJson = JSON.stringify(payload);

    if (options.dryRun) {
      log?.(`${payloadJson}\n`);
      return { action: 'dry-run', version };
    }

    const response = await fetch(`${apiUrl.replace(/\/+$/, '')}/internal/release-notes`, {
      method: 'POST',
      headers: signedHeaders(secret, payloadJson),
      body: payloadJson,
    });
    if (response.status === 200 || response.status === 201) {
      log?.(`    [release-notes] 版本公告 ${version} 已入库 (HTTP ${response.status})\n`);
      return { action: 'uploaded', version };
    }
    if (response.status === 409) {
      log?.('    [release-notes] 版本相同但 releaseSha 不同 (409),视为已更新\n');
      return { action: 'skip', version, reason: '同版本异 releaseSha (409)' };
    }
    return { action: 'error', version, reason: `上传返回 HTTP ${response.status}` };
  } catch (error) {
    return { action: 'error', reason: error instanceof Error ? error.message : String(error) };
  }
}

function withReleaseMarker(rendered: string, releaseSha: string): string {
  return rendered.replace(
    /^## \[([^\]]+)\] - (\d{4}-\d{2}-\d{2})/,
    `## [$1] - $2 <!-- pm:auto release=${releaseSha} -->`,
  );
}
