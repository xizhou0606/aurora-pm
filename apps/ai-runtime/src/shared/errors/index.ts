// 生成失败处理共享逻辑的对外入口(原 src/generation-error.ts)。
// 公开符号与搬迁前保持一致,外部统一从本目录入口导入。
export { safeGenerationDiagnostic, safeGenerationFailureMessage } from './generation-error.js';
