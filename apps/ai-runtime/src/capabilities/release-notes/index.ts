// 版本公告全自动构建能力对外入口(原 src/release-notes-auto.ts)。
// 公开符号与搬迁前保持一致,外部统一从本目录入口导入;I/O 见
// scripts/release-notes-auto.ts。
export {
  CONTENT_TYPES,
  DIFF_SAMPLE_MAX_CHARS,
  DIFF_SAMPLE_MAX_COMMITS,
  blockForVersion,
  buildDeterministicDraft,
  buildEmitPayload,
  contentCommits,
  decideVersion,
  mergeAutoBlock,
  parseChangelogBlocks,
  truncateDiff,
} from './release-notes-auto.js';
export type {
  ChangelogBlockMeta,
  DraftInput,
  ReleaseNotesEmitPayload,
  VersionDecision,
} from './release-notes-auto.js';
