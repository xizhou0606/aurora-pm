import type { LanguageModel } from 'ai';
import { generateText } from 'ai';
import type { QATestCaseModelCallKind, QATestCaseModelCallTelemetry } from '../../../../shared/ai/types.js';
import {
  attachTestCaseModelTelemetry,
  attachTestCaseProviderFailureTelemetry,
  recoverTestCaseJSONModelResult,
  testCaseGenerationOptions,
  type GenerateTestCaseModelText,
} from './generate-runtime.js';
import { qaTestCaseModelCallSuccessTelemetry } from './generate-telemetry.js';

// 供编排层按批/按阶段创建带遥测的模型文本调用闭包(原 qa 分支内联的
// modelTextGenerator 工厂,逐字迁移;模型与预算由参数注入)。
export function createQATestCaseModelTextGenerator(options: {
  model: LanguageModel;
  configuredMaxOutputTokens: number;
  maxOutputTokens: number;
  defaultKind: QATestCaseModelCallKind;
  calls: QATestCaseModelCallTelemetry[];
}): GenerateTestCaseModelText {
  const { model, configuredMaxOutputTokens, maxOutputTokens, defaultKind, calls } = options;
  return async (modelRequest) => {
            const callStartedAt = Date.now();
            const requestedLimit = modelRequest.maxOutputTokens === undefined
              ? maxOutputTokens
              : Math.min(maxOutputTokens, modelRequest.maxOutputTokens);
            const generationOptions = testCaseGenerationOptions(
              configuredMaxOutputTokens,
              requestedLimit,
            );
            try {
              const result = await recoverTestCaseJSONModelResult(async () => {
                const common = {
                  model,
                  instructions: modelRequest.instructions,
                  ...generationOptions,
                  abortSignal: modelRequest.abortSignal,
                };
                return modelRequest.messages
                  ? generateText({ ...common, messages: modelRequest.messages })
                  : generateText({ ...common, prompt: modelRequest.prompt ?? '' });
              });
              const telemetered = attachTestCaseModelTelemetry(
                result,
                generationOptions.maxOutputTokens,
                Date.now() - callStartedAt,
              );
              calls.push(qaTestCaseModelCallSuccessTelemetry(
                calls.length + 1,
                modelRequest.telemetryKind ?? defaultKind,
                telemetered,
              ));
              return telemetered;
            } catch (error) {
              throw attachTestCaseProviderFailureTelemetry(
                error,
                generationOptions.maxOutputTokens,
                Date.now() - callStartedAt,
              );
            }
          }
}
