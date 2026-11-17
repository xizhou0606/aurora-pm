#!/usr/bin/env node
/**
 * Build-time / local sync of the upstream QA Agent Skill into ai-runtime/skills.
 * Resolves a moving ref (default: main) to a commit SHA, downloads the fixed file
 * set, then rewrites the lock manifest. Runtime never talks to GitHub.
 *
 * Prefer the GitHub REST API so Docker builds do not need the git CLI. When the
 * network is unreachable and QA_SKILL_SYNC_STRICT is not true, keep the vendored
 * package already copied into the image so release builds still succeed.
 */
import { createHash } from 'node:crypto';
import { mkdirSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const skillName = 'qa-test-case-design';
const repository = 'https://github.com/QA19816-glitch/QA-agent';
const repositoryPath = 'codex-skills/skills/qa-test-case-design';
const ownerRepo = 'QA19816-glitch/QA-agent';
const ref = (process.env.QA_SKILL_REF || 'main').trim() || 'main';
const pinCommit = (process.env.QA_SKILL_COMMIT || '').trim();
const syncStrict = (process.env.QA_SKILL_SYNC_STRICT || '').trim() === 'true';
const maxAttempts = Math.max(1, Number.parseInt(process.env.QA_SKILL_SYNC_ATTEMPTS || '3', 10) || 3);

const expectedFiles = {
  'SKILL.md': '100644',
  'agents/openai.yaml': '100644',
  'output-templates/feishu-test-cases.md': '100644',
  'output-templates/structured-test-cases.md': '100644',
  'prompts/advanced-test-case-design.md': '100644',
  'references/case-writing-rules.md': '100644',
  'scripts/validate_testcases.py': '100755',
};

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

async function resolveCommit() {
  if (/^[a-f0-9]{40}$/.test(pinCommit)) return pinCommit;
  const url = `https://api.github.com/repos/${ownerRepo}/commits/${encodeURIComponent(ref)}`;
  const response = await fetchWithRetry(url);
  const payload = await response.json();
  const commit = typeof payload?.sha === 'string' ? payload.sha.trim() : '';
  if (!/^[a-f0-9]{40}$/.test(commit)) {
    throw new Error(`unable to resolve ${ref} from ${url}`);
  }
  return commit;
}

async function downloadFile(commit, relativePath) {
  const url = `https://raw.githubusercontent.com/${ownerRepo}/${commit}/${repositoryPath}/${relativePath}`;
  const response = await fetchWithRetry(url, {
    headers: {
      Accept: 'application/octet-stream',
    },
  });
  return Buffer.from(await response.arrayBuffer());
}

async function syncOnce() {
  const commit = await resolveCommit();
  const skillRoot = join(root, 'skills', skillName);
  const lockPath = join(root, 'skills', `${skillName}.lock.json`);
  rmSync(skillRoot, { recursive: true, force: true });
  mkdirSync(skillRoot, { recursive: true });

  const files = {};
  const canonicalHashes = [];
  for (const relativePath of Object.keys(expectedFiles).sort()) {
    const mode = expectedFiles[relativePath];
    const bytes = await downloadFile(commit, relativePath);
    const absolutePath = join(skillRoot, relativePath);
    mkdirSync(dirname(absolutePath), { recursive: true });
    writeFileSync(absolutePath, bytes);
    if (mode === '100755') chmodSync(absolutePath, 0o755);
    else chmodSync(absolutePath, 0o644);
    const hash = sha256(bytes);
    files[relativePath] = { sha256: hash, mode };
    canonicalHashes.push(`${relativePath}\t${mode}\t${hash}\n`);
  }

  const lock = {
    schemaVersion: 1,
    name: skillName,
    source: {
      repository,
      path: repositoryPath,
      commit,
      ref,
      syncedAt: new Date().toISOString(),
    },
    packageSha256: sha256(canonicalHashes.join('')),
    files,
  };
  writeFileSync(lockPath, `${JSON.stringify(lock, null, 2)}\n`);
  process.stdout.write(
    `synced ${skillName}@${commit} packageSha256=${lock.packageSha256}\n`,
  );
}

async function main() {
  try {
    await syncOnce();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (syncStrict) {
      throw error;
    }
    console.warn(`skill sync failed, keeping vendored package: ${message}`);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exit(1);
});
