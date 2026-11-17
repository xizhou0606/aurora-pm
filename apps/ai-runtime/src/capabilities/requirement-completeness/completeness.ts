import { z } from 'zod';
import { aiOutputSchema, type AIOutput } from '../../shared/ai/schema.js';
import { validEvidenceReferences, type EvidenceReference } from '../../shared/evidence.js';

const dimensionKeys = [
  'background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria',
] as const;

const dimensionContract = {
  background: { label: '背景', weight: 15 },
  goal: { label: '目标', weight: 15 },
  scope: { label: '范围', weight: 15 },
  boundaries: { label: '边界', weight: 10 },
  exceptions: { label: '异常场景', weight: 15 },
  dependencies: { label: '依赖', weight: 10 },
  acceptance_criteria: { label: '验收标准', weight: 20 },
} as const;

const draftDimensionSchema = z.object({
  key: z.enum(dimensionKeys),
  status: z.enum(['pass', 'partial', 'missing']),
  findings: z.array(z.string()),
  evidence: z.array(z.string()),
  remediation: z.array(z.string()),
});

export const completenessAnalysisSchema = z.object({
  summary: z.string().min(1),
  completenessReport: z.object({
    dimensions: z.array(draftDimensionSchema).length(7).refine((rows) => new Set(rows.map((row) => row.key)).size === 7, "Each completeness dimension must occur exactly once"),
    missingRequiredFields: z.array(z.string()),
    blockingIssues: z.array(z.string()),
    recommendations: z.array(z.string()),
  }),
});

export type CompletenessAnalysis = z.infer<typeof completenessAnalysisSchema>;

function unique(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
}

function chineseNarratives(values: string[], fallback?: string): string[] {
  const localized = unique(values).filter((value) => /[\u3400-\u9fff]/u.test(value));
  return localized.length ? localized : fallback ? [fallback] : [];
}

function subjectText(subject: Record<string, unknown>, field: string): string {
  const value = subject[field];
  return typeof value === 'string' ? value.trim() : '';
}

function evidenceSupportsDimension(input: Record<string, unknown>, key: typeof dimensionKeys[number], reference: EvidenceReference): boolean {
  if (reference.root === 'sources') {
    const sources = Array.isArray(input.sources) ? input.sources : [];
    const source = sources[Number(reference.segments[0])];
    return reference.segments.length === 2 && reference.segments[1] === 'content' &&
      Boolean(source && typeof source === 'object' && !Array.isArray(source) && isDocumentSource(source as Record<string, unknown>));
  }
  if (reference.root !== 'subject' || reference.segments.length !== 1) return false;
  if (key === 'acceptance_criteria') return reference.segments[0] === 'acceptance_criteria';
  return reference.segments[0] === 'description';
}

function isDocumentSource(row: Record<string, unknown>): boolean {
  return row.available !== false && (row.type === 'feishu_document' || row.type === 'internal_document') && typeof row.content === 'string' && row.content.trim().length > 0;
}

function hasDocument(input: Record<string, unknown>): boolean {
  const sources = Array.isArray(input.sources) ? input.sources : [];
  return sources.some((source) => {
    if (!source || typeof source !== 'object') return false;
    const row = source as Record<string, unknown>;
    return row.available !== false && isDocumentSource(row) && typeof row.content === 'string' && row.content.trim().length > 0;
  });
}

export function normalizeCompletenessOutput(
  input: Record<string, unknown>,
  analysis?: CompletenessAnalysis,
): AIOutput {
  const subject = input.subject as Record<string, unknown> | undefined ?? {};
  const version = typeof subject.version === 'number' && subject.version > 0 ? subject.version : 1;
  const documentAvailable = hasDocument(input);
  const supplied = new Map(analysis?.completenessReport.dimensions.map((dimension) => [dimension.key, dimension]) ?? []);

  const dimensions = dimensionKeys.map((key) => {
    const contract = dimensionContract[key];
    const draft = supplied.get(key);
    const evidence = unique(draft?.evidence ?? []).filter((value) => {
      const references = validEvidenceReferences(input, value);
      return references !== null && references.every((reference) => evidenceSupportsDimension(input, key, reference));
    });
    let status = draft?.status ?? 'missing';
    if (evidence.length === 0) status = 'missing';
    if (
      key === 'acceptance_criteria' &&
      !(
        subjectText(subject, 'acceptance_criteria') &&
        evidence.some((value) => validEvidenceReferences(input, value)?.some((reference) => reference.path === 'subject.acceptance_criteria'))
      ) &&
      !(
        documentAvailable &&
        evidence.some((value) => validEvidenceReferences(input, value)?.some((reference) => reference.root === 'sources' && reference.segments[1] === 'content'))
      )
    ) status = 'missing';
    const evidenceDowngradedStatus = Boolean(draft && status !== draft.status);
    const score = status === 'pass' ? contract.weight : status === 'partial' ? Math.floor(contract.weight / 2) : 0;
    return {
      key,
      label: contract.label,
      status,
      score,
      maxScore: contract.weight,
      findings: chineseNarratives(evidenceDowngradedStatus ? [] : draft?.findings ?? [], status === 'pass' ? `已找到可验证的${contract.label}信息` : status === 'partial' ? `${contract.label}信息不完整` : `未提供可验证的${contract.label}信息`),
      evidence,
      remediation: chineseNarratives(evidenceDowngradedStatus ? [] : draft?.remediation ?? [], status === 'pass' ? undefined : `补充${contract.label}及可验证证据`),
    };
  });

  const dimensionMap = new Map(dimensions.map((dimension) => [dimension.key, dimension]));
  const criticalMissing = ['goal', 'scope', 'acceptance_criteria']
    .filter((key) => dimensionMap.get(key as typeof dimensionKeys[number])?.status === 'missing');
  const acceptanceMissing = dimensionMap.get('acceptance_criteria')?.status === 'missing';
  const missingRequiredFields = unique([
    ...(analysis?.completenessReport.missingRequiredFields ?? []).filter((field) => (field !== 'acceptance_criteria' || acceptanceMissing) && (field !== 'product_doc_url' || !documentAvailable)),
    ...(acceptanceMissing ? ['acceptance_criteria'] : []),
    ...(!documentAvailable && !subjectText(subject, 'product_doc_url') ? ['product_doc_url'] : []),
  ]);
  const modelBlockingIssues = analysis?.completenessReport.blockingIssues ?? [];
  const blockingIssues = unique([
    ...chineseNarratives(modelBlockingIssues, modelBlockingIssues.length ? 'AI 识别到阻断问题，请补充需求信息后重新检查' : undefined),
    ...criticalMissing.map((key) => `${dimensionContract[key as keyof typeof dimensionContract].label}缺失，不能进入产品评审`),
  ]);
  const score = dimensions.reduce((sum, dimension) => sum + dimension.score, 0);
  const threshold = 80;
  const canProceed = score >= threshold && blockingIssues.length === 0 && criticalMissing.length === 0;
  const verdict = canProceed ? 'pass' : blockingIssues.length || criticalMissing.length ? 'blocked' : 'needs_revision';
  const modelRecommendations = analysis?.completenessReport.recommendations ?? [];
  const recommendations = unique([
    ...chineseNarratives(modelRecommendations, modelRecommendations.length ? '根据七维检查结果补充缺失的需求信息和可验证证据' : undefined),
    ...dimensions.filter((dimension) => dimension.status !== 'pass').flatMap((dimension) => dimension.remediation),
  ]);

  return aiOutputSchema.parse({
    summary: `${score} 分（门槛 ${threshold} 分），${canProceed ? '完整性检查通过，可以进入产品评审' : '完整性检查未通过，需要补充后重新检查'}。`,
    changes: [],
    items: dimensions.filter((dimension) => dimension.status !== 'pass').map((dimension) => ({
      title: `补充${dimension.label}`,
      description: dimension.remediation.join('；'),
      evidence: dimension.evidence,
    })),
    risks: blockingIssues,
    humanReviewRequired: true,
    completenessReport: {
      verdict, score, threshold, canProceed, checkedSubjectVersion: version,
      dimensions, missingRequiredFields, blockingIssues, recommendations,
    },
    workflowProposal: null,
  });
}

// Keep the blocking zero-score fallback, but never diagnose missing document
// content when the model output itself could not be verified.
export function createCompletenessFallback(input: Record<string, unknown>, reason: 'format' | 'evidence'): AIOutput {
  const cause = reason === 'format' ? 'AI 输出格式校验异常' : 'AI 输出证据引用校验异常';
  const warning = `${cause}，本次 0 分为兜底结果，不代表文档内容缺失，请重新运行完整性检查。`;
  const retry = '重新运行完整性检查；若持续异常，请联系管理员排查 AI 服务。';
  const output = normalizeCompletenessOutput(input);
  const report = output.completenessReport!;
  return aiOutputSchema.parse({
    ...output,
    summary: warning,
    items: [{ title: '重新运行完整性检查', description: retry, evidence: [] }],
    risks: [warning],
    completenessReport: {
      ...report,
      dimensions: report.dimensions.map(dimension => ({
        ...dimension, findings: [warning], remediation: [retry],
      })),
      missingRequiredFields: [],
      blockingIssues: [warning],
      recommendations: [retry],
    },
  });
}
export function normalizeVerifiedCompletenessOutput(input: Record<string, unknown>, analysis: CompletenessAnalysis): AIOutput {
  if (!completenessAnalysisSchema.safeParse(analysis).success) return createCompletenessFallback(input, 'format');
  for (const dimension of analysis.completenessReport.dimensions) {
    if (dimension.status === 'missing') continue;
    if (!dimension.evidence.length || dimension.evidence.some((value) => {
      const references = validEvidenceReferences(input, value);
      return references === null || !references.every((reference) => evidenceSupportsDimension(input, dimension.key, reference));
    })) return createCompletenessFallback(input, 'evidence');
  }
  return normalizeCompletenessOutput(input, analysis);
}
