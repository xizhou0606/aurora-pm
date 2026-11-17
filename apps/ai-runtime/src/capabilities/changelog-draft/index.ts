// 版本公告草稿能力(changelog.draft)对外入口(原 src/changelog-draft.ts
// 与 src/changelog-draft-cli.ts)。公开符号与搬迁前保持一致,外部统一从
// 本目录入口导入;CLI 的 I/O 见 scripts/changelog-draft.ts。
export {
  CHANGELOG_DRAFT_CAPABILITY,
  CHANGELOG_SUMMARY_TITLE,
  buildChangelogDraftPrompt,
  changelogDraftOutputSchema,
  changelogDraftSkillMetadata,
  normalizeChangelogDraftOutput,
  recoverChangelogDraftOutput,
} from './changelog-draft.js';
export type { ChangelogDraftSkillMetadata } from './changelog-draft.js';
export {
  findReleaseAnchor,
  lastChangelogVersion,
  mergeCollectedCommits,
  parseConventionalSubject,
  parseDotEnv,
  parseGitLog,
  parseGitNumstat,
  renderChangelogBlock,
  suggestNextVersion,
} from './changelog-draft-cli.js';
export type {
  ChangelogBlockInput,
  CollectedCommit,
  NumstatSummary,
  RawLogCommit,
} from './changelog-draft-cli.js';
