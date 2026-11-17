import type { RiskCoverageDiagnostic } from '../qa-test-case-draft-risk.js';
import type {
  QATestCaseDraftContextReferenceDiagnostic,
  QATestCaseTargetCoverageDiagnostic,
} from './qa-test-case-draft-schema.js';

export function unique(values: string[]): string[] {
  return [...new Set(values)];
}

function draftContextReferenceDiagnostics(
  issues: string[],
): QATestCaseDraftContextReferenceDiagnostic[] {
  return issues.flatMap((issue): QATestCaseDraftContextReferenceDiagnostic[] => {
    if (/^source assessment .* context refs must be unique$/.test(issue)) {
      return [{ code: 'duplicate_context_ref', pathCategory: 'source_assessment_context_refs' }];
    }
    if (/^source assessment .* has unavailable context ref:/.test(issue)) {
      return [{ code: 'unavailable_context_ref', pathCategory: 'source_assessment_context_refs' }];
    }
    if (/^risk .* context refs must be unique$/.test(issue)) {
      return [{ code: 'duplicate_context_ref', pathCategory: 'risk_context_refs' }];
    }
    if (/^case \d+ context refs must be unique$/.test(issue)) {
      return [{ code: 'duplicate_context_ref', pathCategory: 'case_context_refs' }];
    }
    if (/^risk .* has unavailable context ref:/.test(issue)) {
      return [{ code: 'unavailable_context_ref', pathCategory: 'risk_context_refs' }];
    }
    if (/^case \d+ has unavailable context ref:/.test(issue)) {
      return [{ code: 'unavailable_context_ref', pathCategory: 'case_context_refs' }];
    }
    if (/^case \d+ risk link has unavailable context ref:/.test(issue)) {
      return [{ code: 'unavailable_context_ref', pathCategory: 'case_risk_links' }];
    }
    if (/^case \d+ risk link context ref is not declared by /.test(issue)) {
      return [{ code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links' }];
    }
    return [];
  });
}

export class QATestCaseDraftCompilationError extends Error {
  readonly issues: string[];
  readonly diagnostics: RiskCoverageDiagnostic[];
  readonly targetCoverageDiagnostics: QATestCaseTargetCoverageDiagnostic[];
  readonly contextReferenceDiagnostics: QATestCaseDraftContextReferenceDiagnostic[];

  constructor(
    issues: string[],
    diagnostics: RiskCoverageDiagnostic[] = [],
    targetCoverageDiagnostics: QATestCaseTargetCoverageDiagnostic[] = [],
  ) {
    const distinctIssues = unique(issues);
    super(`qa test case draft compilation failed: ${distinctIssues.join('; ')}`);
    this.name = 'QATestCaseDraftCompilationError';
    this.issues = distinctIssues;
    this.diagnostics = diagnostics.map((diagnostic) => ({
      ...diagnostic,
      actualCategories: [...diagnostic.actualCategories],
      missingCategories: [...diagnostic.missingCategories],
    }));
    this.targetCoverageDiagnostics = targetCoverageDiagnostics.map((diagnostic) => ({
      ...diagnostic,
      candidateContextRefs: [...diagnostic.candidateContextRefs],
    }));
    this.contextReferenceDiagnostics = draftContextReferenceDiagnostics(distinctIssues);
  }
}
