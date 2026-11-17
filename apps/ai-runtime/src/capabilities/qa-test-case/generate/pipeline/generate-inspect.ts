import { ZodError } from 'zod';
import { aiOutputSchema, type AIOutput } from '../../../../shared/ai/schema.js';
import { buildQATestCaseRepairInstruction, parseTestCaseGenerationText } from '../../index.js';

export function testCaseRepairSource(output: AIOutput): unknown {
  if (!output.testCaseDesign) return { summary: output.summary, testCases: output.testCases };
  return {
    summary: output.summary,
    design: {
      ...output.testCaseDesign,
      testCases: output.testCases.map(({ expectedResult: _expectedResult, ...testCase }) => testCase),
    },
  };
}

export const invalidTestCaseJSONMessage = 'test case model response was not a complete valid JSON artifact';

export type TestCaseTextInspection =
  | { output: AIOutput; failure: null; issues: [] }
  | { output: null; failure: 'parse' | 'structure'; issues: string[] };

const safeTestCaseSchemaPathSegments = new Set([
  'summary', 'design', 'objective', 'scope', 'exclusions', 'sourceInventory',
  'id', 'kind', 'status', 'evidence', 'assumptions', 'informationGaps',
  'coverageStrategy', 'priorityLogic', 'methods', 'riskHotspots', 'description',
  'priority', 'testCases', 'caseId', 'title', 'category', 'trace', 'preconditions',
  'testData', 'steps', 'expectedResults', 'notes', 'traceability', 'traceId',
  'caseIds', 'coverageTypes', 'gapNote', 'smokeCaseIds', 'executionOrder',
  'automationCandidateCaseIds', 'releaseBlockingCaseIds', 'risks', 'riskId',
  'contextRefs', 'cases', 'riskIds', 'actions', 'step', 'expectedResult',
  'smoke', 'automationCandidate', 'releaseBlocking', 'sourceAssessments',
  'targetRef', 'sourceFindings', 'interactions', 'noCrossInteractionReason',
  'executionPosition', 'riskLinks', 'contextRef', 'coverageDecision', 'reason',
]);

function safeZodPath(path: ZodError['issues'][number]['path']): string {
  let result = '';
  for (const segment of path) {
    if (typeof segment === 'number' && Number.isSafeInteger(segment) && segment >= 0) {
      result += `[${segment}]`;
      continue;
    }
    if (typeof segment === 'string' && safeTestCaseSchemaPathSegments.has(segment)) {
      result += result ? `.${segment}` : segment;
      continue;
    }
    result += result ? '.field' : 'field';
  }
  return result || 'root';
}

export function safeZodStructureIssue(issue: ZodError['issues'][number]): string {
  const path = safeZodPath(issue.path);
  if (issue.code === 'invalid_value') {
    const allowed = issue.values
      .flatMap((value) => ['string', 'number', 'boolean'].includes(typeof value) ? [String(value)] : [])
      .slice(0, 20);
    if (allowed.length) {
      return `test case output structure at ${path} must use one of: ${allowed.join(', ')}`;
    }
  }
  if (issue.code === 'invalid_type') {
    return `test case output structure at ${path} has the wrong JSON type`;
  }
  if (issue.code === 'too_small') {
    return `test case output structure at ${path} does not satisfy the required minimum`;
  }
  if (issue.code === 'too_big') {
    return `test case output structure at ${path} exceeds the allowed maximum`;
  }
  return `test case output structure at ${path} does not match the required schema`;
}

export function inspectTestCaseGenerationText(text: string): TestCaseTextInspection {
  try {
    return { output: parseTestCaseGenerationText(text), failure: null, issues: [] };
  } catch (error) {
    if (error instanceof ZodError) {
      return {
        output: null,
        failure: 'structure',
        issues: [...new Set(error.issues.map(safeZodStructureIssue))],
      };
    }
    return { output: null, failure: 'parse', issues: [invalidTestCaseJSONMessage] };
  }
}

export function parseCompleteTestCaseOutput(text: string): AIOutput {
  const inspected = inspectTestCaseGenerationText(text);
  if (inspected.output) return inspected.output;
  throw new Error(inspected.issues.join('; '));
}

export function buildQATestCaseJSONRewriteInstruction(): string {
  return [
    '上一版输出不是一个完整、可解析的测试用例 JSON 工件。请依据相同业务输入完整重写，不要续写、补写或解释上一版。',
    '只返回一个从左花括号开始、到右花括号结束的完整 JSON 对象，不得输出 Markdown、思考过程或额外文字。',
    buildQATestCaseRepairInstruction(['输出必须是完整合法的 JSON，且满足固定 Skill 的全部结构与质量规则']),
  ].join('\n');
}
