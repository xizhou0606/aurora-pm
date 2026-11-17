import { aiOutputSchema, type AIOutput } from '../../shared/ai/schema.js';

// requirement.design_classify 输出归一(原 generator.ts 内实现,逐字迁移):
// 兼容多种模型别名,归一失败时退回显式人工复核结果。

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function stringArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap((item) => {
    if (typeof item === 'string' && item.trim()) return [item.trim()];
    const row = record(item);
    const nested = row.action ?? row.step ?? row.description ?? row.content ?? row.value;
    return typeof nested === 'string' && nested.trim() ? [nested.trim()] : [];
  });
  if (typeof value === 'string' && value.trim()) return [value.trim()];
  return [];
}

function booleanDecision(value: unknown): boolean | null {
  if (typeof value === 'boolean') return value;
  const normalized = String(value ?? '').trim().toLowerCase();
  if (['true', 'yes', 'required', 'need', 'needed', 'ui_needed', '需要', '是'].includes(normalized)) return true;
  if (['false', 'no', 'not_required', 'not_needed', 'no_ui_needed', '不需要', '否'].includes(normalized)) return false;
  return null;
}

export function normalizeDesignClassificationOutput(value: unknown): AIOutput {
  const root = record(value);
  const nested = record(root.data);
  const source = ['summary', 'requiresDesign', 'requires_design', 'needDesign', 'need_design', 'uiNeeded', 'ui_needed', 'decision']
    .some((key) => key in root)
    ? root
    : Object.keys(nested).length ? nested : root;
  const rawDecision = source.requiresDesign ?? source.requires_design ?? source.needDesign
    ?? source.need_design ?? source.uiNeeded ?? source.ui_needed ?? source.decision;
  const requiresDesign = booleanDecision(rawDecision);
  const evidence = stringArray(source.evidence ?? source.basis ?? source.reasons ?? source.reason);
  const missingMaterials = stringArray(
    source.missingMaterials ?? source.missing_materials ?? source.requiredMaterials
      ?? source.required_materials ?? source.materialsNeeded,
  );
  const modelRisks = stringArray(source.risks ?? source.risk);
  const summary = typeof source.summary === 'string' && source.summary.trim()
    ? source.summary.trim()
    : requiresDesign === null
      ? '设计介入判断的模型输出格式异常，请人工复核。'
      : requiresDesign
        ? '该需求需要 UI、交互或视觉设计介入，请人工确认。'
        : '当前事实未显示必须由 UI、交互或视觉设计介入，请人工确认。';
  const risks = requiresDesign === null
    ? ['未能可靠解析设计介入判断结果，请勿据此跳过人工评审。', ...modelRisks]
    : modelRisks;
  return aiOutputSchema.parse({
    summary,
    changes: [],
    items: [
      ...(requiresDesign === null ? [] : [{
        title: `设计介入结论：${requiresDesign ? '需要' : '暂不需要'}`,
        description: summary,
        evidence,
      }]),
      ...(requiresDesign === null && evidence.length ? [{ title: '设计介入判断依据', description: evidence.join('；'), evidence }] : []),
      ...missingMaterials.map((material) => ({ title: material, description: '设计评审前需要补充的材料。', evidence: [] })),
    ],
    risks,
    testCases: [],
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
  });
}

export function parseDesignClassificationText(text: string): AIOutput {
  const withoutThinking = text.trim().replace(/<think>[\s\S]*?<\/think>/gi, '').trim();
  const fenced = [...withoutThinking.matchAll(/```(?:json)?\s*([\s\S]*?)```/gi)].map((match) => match[1].trim());
  const objectStart = withoutThinking.indexOf('{');
  const objectEnd = withoutThinking.lastIndexOf('}');
  const candidates = [
    withoutThinking,
    ...fenced,
    objectStart >= 0 && objectEnd > objectStart ? withoutThinking.slice(objectStart, objectEnd + 1) : '',
  ].filter((candidate, index, all) => Boolean(candidate) && all.indexOf(candidate) === index);
  let lastError: unknown = new Error('design classification output did not contain JSON');
  for (const candidate of candidates) {
    try {
      return normalizeDesignClassificationOutput(JSON.parse(candidate) as unknown);
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}

export function recoverDesignClassificationOutput(error: unknown): AIOutput {
  const text = (error as { text?: unknown })?.text;
  if (typeof text === 'string' && text.trim()) {
    try {
      return parseDesignClassificationText(text);
    } catch {
      // Fall through to an explicit human-review result. A format failure must
      // never be interpreted as permission to skip design review.
    }
  }
  return normalizeDesignClassificationOutput(null);
}
