// 容器启动入口:pm-ai-runtime 每次部署都会被重建,Jenkins 的
// `up -d --force-recreate pm-ai-runtime` 就是公告生成的触发点。
// 本进程以最佳努力运行:生成/上传失败只打日志,永不阻断服务启动
// (wrapper 里与 server.js 并行,见 Dockerfile CMD)。
import { runReleaseNotesOnStart, shouldAutoPublish } from './release-notes/pending-announcement.js';

const MAX_ATTEMPTS = 3;
const RETRY_DELAY_MS = 15_000;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function main(): Promise<void> {
  const env = process.env;
  const gate = shouldAutoPublish(env);
  if (!gate.run) {
    if (gate.reason) process.stdout.write(`    [release-notes] ${gate.reason}\n`);
    return;
  }
  const options = {
    repoDir: env.PM_REPO_DIR ?? '/repo',
    apiUrl: env.PM_RELEASE_API_URL ?? 'http://pm-api:8080',
    secret: env.PM_RELEASE_INTERNAL_SECRET ?? env.AI_INTERNAL_SECRET ?? '',
    useAi: env.PM_RELEASE_NOTES_NO_AI !== 'true',
    dryRun: env.PM_RELEASE_NOTES_DRY_RUN === 'true',
    env,
    log: (message: string) => {
      process.stdout.write(message);
    },
  };

  for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt += 1) {
    const outcome = await runReleaseNotesOnStart(options);
    if (outcome.action !== 'error') return;
    if (attempt === MAX_ATTEMPTS) {
      process.stdout.write(`    [release-notes] 版本公告生成失败(不影响服务):${outcome.reason ?? '未知原因'}\n`);
      return;
    }
    process.stdout.write(`    [release-notes] 第 ${attempt} 次生成失败(${outcome.reason ?? '未知原因'}),${RETRY_DELAY_MS / 1000}s 后重试\n`);
    await sleep(RETRY_DELAY_MS);
  }
}

void main();
