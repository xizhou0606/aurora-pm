import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'bun:test';

// 架构约束测试(生成约束,必须全过):
// a. src 根目录仅允许入口文件与目录,出现其他 .ts 文件即失败;
// b. 全部 .ts 文件 ≤300 行(白名单豁免见 LINE_LIMIT_EXEMPTIONS);
// c. src/capabilities/ 下任意能力目录禁止 import 另一能力目录(跨能力只允许经 registry/executor/shared);
// d. capabilities/ 每个能力目录必须有 index.ts。
// e. 任意目录下非测试 .ts 文件不得超过 MAX_FILES_PER_DIR(5)个,超限必须继续拆子目录(*.test.ts 不计入)。

const SRC_ROOT = resolve(fileURLToPath(new URL('..', import.meta.url)));

// 根目录白名单:入口文件 + app/config 的同目录测试(config.ts 本体受保护不可迁移)。
const ROOT_FILE_ALLOWLIST = new Set(['server.ts', 'app.ts', 'app.test.ts', 'config.ts', 'config.test.ts', 'release-notes-on-start.ts']);

// 行数豁免:固定 Skill 单文件内聚(用户指令保留完整)。
const LINE_LIMIT_EXEMPTIONS = new Set(
  [
    'capabilities/qa-test-case/skill/qa-test-case-skill.ts',
    'capabilities/qa-test-case/skill/qa-test-case-skill.test.ts',
  ],
);

const MAX_LINES = 300;

// 目录扁平度:单目录非测试 .ts 文件上限,超限必须继续拆子目录(*.test.ts 不计入)。
const MAX_FILES_PER_DIR = 5;

function listFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...listFiles(full));
      continue;
    }
    if (entry.endsWith('.ts')) out.push(full);
  }
  return out;
}

function rel(file: string): string {
  return relative(SRC_ROOT, file);
}

const allFiles = listFiles(SRC_ROOT);

describe('ai-runtime architecture constraints', () => {
  it('keeps the src root to entry files and directories only', () => {
    const offenders = readdirSync(SRC_ROOT)
      .filter((entry) => entry.endsWith('.ts') && !ROOT_FILE_ALLOWLIST.has(entry));
    expect(offenders).toEqual([]);
  });

  it('keeps every .ts file within the 300 line budget except the exempted fixed skill files', () => {
    const offenders = allFiles
      .filter((file) => !LINE_LIMIT_EXEMPTIONS.has(rel(file)))
      .map((file) => ({ file: rel(file), lines: readFileSync(file, 'utf8').split('\n').length }))
      .filter((row) => row.lines > MAX_LINES);
    expect(offenders.map((row) => `${row.file}: ${row.lines}`)).toEqual([]);
  });

  it('never lets one capability directory import another capability directory', () => {
    const capabilityDir = join(SRC_ROOT, 'capabilities');
    const violations: string[] = [];
    const dirs = readdirSync(capabilityDir)
      .filter((entry) => statSync(join(capabilityDir, entry)).isDirectory());
    const dirSet = new Set(dirs);
    for (const dir of dirs) {
      for (const file of listFiles(join(capabilityDir, dir))) {
        const source = readFileSync(file, 'utf8');
        const importSpecifiers = [...source.matchAll(/from\s+['"](\.[^'"]+)['"]/g)]
          .map((match) => match[1]);
        for (const specifier of importSpecifiers) {
          const target = resolve(file, '..', specifier);
          const targetRel = relative(capabilityDir, target);
          const [firstSegment] = targetRel.split('/');
          if (dirSet.has(firstSegment) && firstSegment !== dir) {
            violations.push(`${rel(file)} -> ${specifier}`);
          }
        }
      }
    }
    expect(violations).toEqual([]);
  });

  it('requires an index.ts entry in every capability directory', () => {
    const capabilityDir = join(SRC_ROOT, 'capabilities');
    const dirs = readdirSync(capabilityDir)
      .filter((entry) => statSync(join(capabilityDir, entry)).isDirectory());
    const missing = dirs.filter((dir) => {
      try {
        statSync(join(capabilityDir, dir, 'index.ts'));
        return false;
      } catch {
        return true;
      }
    });
    expect(missing).toEqual([]);
  });

  it('keeps every directory to at most 5 non-test .ts files', () => {
    const offenders: string[] = [];
    const visit = (dir: string): void => {
      const entries = readdirSync(dir);
      const nonTestTsFiles = entries.filter(
        (entry) => entry.endsWith('.ts') && !entry.endsWith('.test.ts'),
      );
      if (nonTestTsFiles.length > MAX_FILES_PER_DIR) {
        offenders.push(`${rel(dir) || '.'}: ${nonTestTsFiles.length}`);
      }
      for (const entry of entries) {
        const full = join(dir, entry);
        if (statSync(full).isDirectory()) visit(full);
      }
    };
    visit(SRC_ROOT);
    expect(offenders).toEqual([]);
  });
});
