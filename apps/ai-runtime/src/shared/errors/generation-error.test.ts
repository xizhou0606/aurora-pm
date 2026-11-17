import { describe, expect, it } from 'bun:test';
import { safeGenerationDiagnostic } from './generation-error.js';

describe('safeGenerationDiagnostic', () => {
  it('keeps only bounded QA stage metadata', () => {
    const error = new Error('test case generation ended with length; refusing a partial artifact');
    Object.assign(error, {
      qaStage: 'draft_batch',
      qaBatchNumber: 3,
      qaBatchCount: 4,
      qaPhase: 'generation',
      qaFinishReason: 'length',
      qaRequestedMaxOutputTokens: 8000,
      qaOutputTokens: 7998,
      qaCallElapsedMs: 98_000,
      qaFullDraftRepairAttempt: 2,
      qaRepairKind: 'full_draft',
      qaFailureStep: 'finish',
      qaCoverageDiagnosticCount: 2,
      qaGlobalRefDiagnosticCount: 3,
      qaProviderFailureKind: 'retry_exhausted',
      qaProviderStatusCode: 503,
      qaProviderRetryable: true,
      qaRuleFailureCodes: ['risk_coverage', 'context_coverage', 'secret-code'],
      qaContextReferenceDiagnostics: [
        { code: 'unavailable_context_ref', pathCategory: 'case_context_refs', rawRef: 'secret-ref' },
        { code: 'unavailable_context_ref', pathCategory: 'case_context_refs', rawRef: 'secret-ref' },
        { code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links' },
        { code: 'duplicate_context_ref', pathCategory: 'source_assessment_context_refs' },
        { code: 'secret-code', pathCategory: 'source:0:content' },
      ],
      rawModelText: 'must never be logged',
    });

    expect(safeGenerationDiagnostic(error)).toEqual({
      name: 'Error',
      message: 'test case generation did not complete',
      stage: 'draft_batch',
      batchNumber: 3,
      phase: 'generation',
      repairKind: 'full_draft',
      failureStep: 'finish',
      finishReason: 'length',
      requestedMaxOutputTokens: 8000,
      outputTokens: 7998,
      callElapsedMs: 98_000,
      batchCount: 4,
      fullDraftRepairAttempt: 2,
      coverageDiagnosticCount: 2,
      globalRefDiagnosticCount: 3,
      providerFailureKind: 'retry_exhausted',
      providerStatusCode: 503,
      providerRetryable: true,
      ruleFailureCodes: ['risk_coverage', 'context_coverage'],
      contextReferenceDiagnostics: [
        { code: 'unavailable_context_ref', pathCategory: 'case_context_refs', count: 2 },
        { code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links', count: 1 },
        { code: 'duplicate_context_ref', pathCategory: 'source_assessment_context_refs', count: 1 },
      ],
    });
  });

  it('drops unknown metadata and sanitizes an unsafe finish reason', () => {
    const error = new Error('provider secret');
    Object.assign(error, {
      qaStage: 'other',
      qaBatchNumber: -1,
      qaPhase: 'other',
      qaFinishReason: 'secret value with spaces',
      qaRepairKind: 'secret-kind',
      qaFailureStep: 'secret-step',
      qaCoverageDiagnosticCount: 101,
      qaGlobalRefDiagnosticCount: 101,
      qaFullDraftRepairAttempt: 3,
      qaRuleFailureCodes: ['secret-code'],
      qaProviderFailureKind: 'secret-kind',
      qaProviderStatusCode: 200,
      qaProviderRetryable: 'yes',
    });

    expect(safeGenerationDiagnostic(error)).toEqual({
      name: 'Error',
      message: 'model provider request failed',
      finishReason: 'unknown',
    });
  });

  it('reports a repair attempt only for a bounded full Draft repair', () => {
    const coverage = Object.assign(new Error('test case repair failed'), {
      qaRepairKind: 'coverage',
      qaFullDraftRepairAttempt: 2,
    });
    const finalDraft = Object.assign(new Error('test case repair failed'), {
      qaRepairKind: 'full_draft',
      qaFullDraftRepairAttempt: 1,
    });

    expect(safeGenerationDiagnostic(coverage)).not.toHaveProperty('fullDraftRepairAttempt');
    expect(safeGenerationDiagnostic(finalDraft)).toMatchObject({
      repairKind: 'full_draft', fullDraftRepairAttempt: 1,
    });
  });
});
