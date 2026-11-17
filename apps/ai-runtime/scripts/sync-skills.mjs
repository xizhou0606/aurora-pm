#!/usr/bin/env node
/**
 * Build-time / local sync of every vendored AI skill under ai-runtime/skills.
 * Each entry resolves a moving ref (default: main) to a commit SHA, downloads
 * the skill's file set from the GitHub tree API, and rewrites its lock
 * manifest. Runtime never talks to GitHub. Category prefixes in the directory
 * name mirror the runtime capability families:
 *   qa-*     测试用例设计        (QA test-case design)
 *   report-* 汇报写作与绩效评语  (report writing / performance narrative)
 *   code-*   代码质量审查        (code contribution quality review)
 *
 * Per-skill commit pin: PM_SKILL_COMMIT_<NAME with - and * → _> (40-hex SHA).
 * Select a subset: PM_SKILLS="report-status-summary,code-review".
 * Without PM_SKILL_SYNC_STRICT=true a network failure keeps the vendored
 * package already on disk (and in the image) instead of failing the build.
 */
import { createHash } from 'node:crypto';
import { mkdirSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');

const SKILLS = [
  {
    name: 'qa-test-case-design',
    repository: 'https://github.com/QA19816-glitch/QA-agent',
    repositoryPath: 'codex-skills/skills/qa-test-case-design',
    ref: (process.env.QA_SKILL_REF || 'main').trim() || 'main',
  },
  {
    name: 'report-status-summary',
    repository: 'https://github.com/anthropics/knowledge-work-plugins',
    repositoryPath: 'operations/skills/status-report',
    ref: 'main',
  },
  {
    name: 'report-performance-review',
    repository: 'https://github.com/anthropics/knowledge-work-plugins',
    repositoryPath: 'human-resources/skills/performance-review',
    ref: 'main',
  },
  {
    name: 'code-review',
    repository: 'https://github.com/anthropics/knowledge-work-plugins',
    repositoryPath: 'engineering/skills/code-review',
    ref: 'main',
  },
];

const syncStrict = (process.env.PM_SKILL_SYNC_STRICT || '').trim() === 'true';
const selected = (process.env.PM_SKILLS || '')
  .split(',').map((entry) => entry.trim()).filter(Boolean);
const maxAttempts = Math.max(1, Number.parseInt(process.env.PM_SKILL_SYNC_ATTEMPTS || '3', 10) || 3);

function sha256(value) {
  return createHash('sha256').update(value).digest('hex');
}

function authHeaders() {
  return {
    'User-Agent': 'pm-ai-runtime-skill-sync',
    Accept: 'application/vnd.github+json',
    ...(process.env.GITHUB_TOKEN
      ? { Authorization: `Bearer ${process.env.GITHUB_TOKEN}` }
      : {}),
  };
}

async function sleep(ms) {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

async function fetchWithRetry(url, init = {}) {
  let lastError;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      const response = await fetch(url, {
        ...init,
        headers: {
          ...authHeaders(),
          ...(init.headers || {}),
        },
      });
      if (response.ok) return response;
      lastError = new Error(`HTTP ${response.status} for ${url}`);
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }
    if (attempt < maxAttempts) {
      await sleep(attempt * 1000);
    }
  }
  throw lastError instanceof Error ? lastError : new Error(String(lastError));
}

function ownerRepo(repository) {
  const match = repository.match(/github\.com\/([^/]+\/[^/]+)$/);
  if (!match) throw new Error(`unsupported repository URL: ${repository}`);
  return match[1];
}

async function resolveCommit(skill) {
  const pin = (process.env[`PM_SKILL_COMMIT_${skill.name.replaceAll(/-/g, '_').toUpperCase()}`] || '').trim();
  if (/^[a-f0-9]{40}$/.test(pin)) return pin;
  const url = `https://api.github.com/repos/${ownerRepo(skill.repository)}/commits/${encodeURIComponent(skill.ref)}`;
  const response = await fetchWithRetry(url);
  const payload = await response.json();
  const commit = typeof payload?.sha === 'string' ? payload.sha.trim() : '';
  if (!/^[a-f0-9]{40}$/.test(commit)) {
    throw new Error(`unable to resolve ${skill.ref} from ${url}`);
  }
  return commit;
}

async function listSkillFiles(ownerRepoPath, commit, repositoryPath) {
  const url = `https://api.github.com/repos/${ownerRepoPath}/git/trees/${commit}?recursive=1`;
  const response = await fetchWithRetry(url);
  const payload = await response.json();
  const blobs = Array.isArray(payload?.tree) ? payload.tree : [];
  const files = blobs
    .filter((entry) => entry.type === 'blob'
      && entry.path.startsWith(`${repositoryPath}/`)
      && !entry.path.includes('/.'))
    .map((entry) => ({
      relativePath: entry.path.slice(repositoryPath.length + 1),
      mode: entry.mode === '100755' ? '100755' : '100644',
    }));
  if (files.length === 0) {
    throw new Error(`no files found under ${repositoryPath} @ ${commit}`);
  }
  return files;
}

async function downloadFile(ownerRepoPath, commit, repositoryPath, relativePath) {
  const url = `https://raw.githubusercontent.com/${ownerRepoPath}/${commit}/${repositoryPath}/${relativePath}`;
  const response = await fetchWithRetry(url, {
    headers: { Accept: 'application/octet-stream' },
  });
  return Buffer.from(await response.arrayBuffer());
}

async function syncSkill(skill) {
  const repoPath = ownerRepo(skill.repository);
  const commit = await resolveCommit(skill);
  const files = await listSkillFiles(repoPath, commit, skill.repositoryPath);
  const skillRoot = join(root, 'skills', skill.name);
  const lockPath = join(root, 'skills', `${skill.name}.lock.json`);
  rmSync(skillRoot, { recursive: true, force: true });
  mkdirSync(skillRoot, { recursive: true });

  const manifest = {};
  const canonicalHashes = [];
  for (const file of files.sort((left, right) => left.relativePath.localeCompare(right.relativePath))) {
    const bytes = await downloadFile(repoPath, commit, skill.repositoryPath, file.relativePath);
    const absolutePath = join(skillRoot, file.relativePath);
    mkdirSync(dirname(absolutePath), { recursive: true });
    writeFileSync(absolutePath, bytes);
    chmodSync(absolutePath, file.mode === '100755' ? 0o755 : 0o644);
    const hash = sha256(bytes);
    manifest[file.relativePath] = { sha256: hash, mode: file.mode };
    canonicalHashes.push(`${file.relativePath}\t${file.mode}\t${hash}\n`);
  }

  const lock = {
    schemaVersion: 1,
    name: skill.name,
    source: {
      repository: skill.repository,
      path: skill.repositoryPath,
      commit,
      ref: skill.ref,
      syncedAt: new Date().toISOString(),
    },
    packageSha256: sha256(canonicalHashes.join('')),
    files: manifest,
  };
  writeFileSync(lockPath, `${JSON.stringify(lock, null, 2)}\n`);
  process.stdout.write(`synced ${skill.name}@${commit.slice(0, 12)} files=${files.length} packageSha256=${lock.packageSha256}\n`);
}

async function main() {
  const failures = [];
  for (const skill of SKILLS) {
    if (selected.length > 0 && !selected.includes(skill.name)) continue;
    try {
      await syncSkill(skill);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (syncStrict) throw error;
      failures.push(`${skill.name}: ${message}`);
    }
  }
  for (const failure of failures) {
    console.warn(`skill sync failed, keeping vendored package: ${failure}`);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
});
