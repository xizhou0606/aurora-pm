export type CoreRiskCoverageCategory = 'positive' | 'negative' | 'boundary';

export type RiskCoverageDiagnostic = {
  code: 'risk_coverage_decision_mismatch';
  riskId: string;
  priority: 'P0' | 'P1' | 'P2' | 'P3';
  actualCategories: CoreRiskCoverageCategory[];
  missingCategories: CoreRiskCoverageCategory[];
  expectedStatus: 'complete' | 'omitted' | 'not_required';
  message: string;
};

type RiskCoverageDraft = {
  riskId: string;
  priority: 'P0' | 'P1' | 'P2' | 'P3';
  coverageDecision:
    | { status: 'complete' }
    | { status: 'omitted'; reason: string }
    | { status: 'not_required' };
};

type RiskCaseDraft = {
  category: string;
  riskLinks: Array<{ riskId: string }>;
};

const coreCategories: CoreRiskCoverageCategory[] = ['positive', 'negative', 'boundary'];

export function compileRiskCoverageDecisions(
  risks: RiskCoverageDraft[],
  cases: RiskCaseDraft[],
  issues: string[],
  diagnostics: RiskCoverageDiagnostic[] = [],
): Map<string, string> {
  const gapNotes = new Map<string, string>();
  for (const risk of risks) {
    const linkedCategories = new Set(cases
      .filter((testCase) => testCase.riskLinks.some((link) => link.riskId === risk.riskId))
      .map((testCase) => testCase.category));
    const actualCategories = coreCategories.filter((category) => linkedCategories.has(category));
    const missingCategories = coreCategories.filter((category) => !linkedCategories.has(category));
    if (risk.priority === 'P2' || risk.priority === 'P3') {
      if (risk.coverageDecision.status !== 'not_required') {
        const message = `risk ${risk.riskId} coverage decision must be not_required for ${risk.priority}`;
        issues.push(message);
        diagnostics.push({
          code: 'risk_coverage_decision_mismatch',
          riskId: risk.riskId,
          priority: risk.priority,
          actualCategories,
          missingCategories,
          expectedStatus: 'not_required',
          message,
        });
      }
      continue;
    }
    if (!missingCategories.length) {
      if (risk.coverageDecision.status !== 'complete') {
        const message = `risk ${risk.riskId} coverage decision must be complete when no categories are missing`;
        issues.push(message);
        diagnostics.push({
          code: 'risk_coverage_decision_mismatch',
          riskId: risk.riskId,
          priority: risk.priority,
          actualCategories,
          missingCategories,
          expectedStatus: 'complete',
          message,
        });
      }
      continue;
    }
    if (risk.coverageDecision.status !== 'omitted') {
      const message = `risk ${risk.riskId} coverage decision must be omitted for missing categories: ${missingCategories.join(', ')}`;
      issues.push(message);
      diagnostics.push({
        code: 'risk_coverage_decision_mismatch',
        riskId: risk.riskId,
        priority: risk.priority,
        actualCategories,
        missingCategories,
        expectedStatus: 'omitted',
        message,
      });
      continue;
    }
    gapNotes.set(risk.riskId, `${missingCategories.join(', ')}: ${risk.coverageDecision.reason}`);
  }
  return gapNotes;
}
