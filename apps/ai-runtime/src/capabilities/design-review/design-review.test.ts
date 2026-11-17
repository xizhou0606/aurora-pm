import { describe, expect, it } from 'bun:test';
import {
  DESIGN_RECHECK_PROMPT_VERSION,
  DESIGN_REVIEW_PROMPT_VERSION,
  designRecheckOutputSchema,
  designReviewRequestSchema,
  type DesignRecheckOutput,
  type DesignReviewRequest,
} from './index.js';

const reviewId = '11111111-1111-4111-8111-111111111111';
const sourceReviewId = '22222222-2222-4222-8222-222222222222';
const chainRootReviewId = '33333333-3333-4333-8333-333333333333';
const historicalDifferenceId = '44444444-4444-4444-8444-444444444444';
const lineageId = '55555555-5555-4555-8555-555555555555';

const region = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 };

function validRequest(): DesignReviewRequest {
  return {
    manifest: {
      version: '1.0', reviewId, workspaceId: 7, kind: 'full', model: 'vision-model',
      reasoningEffort: 'medium', imageDetail: 'high',
    },
    evidence: [
      { kind: 'design', contentType: 'image/png', sha256: 'a'.repeat(64), base64: 'aGVsbG8=' },
      { kind: 'implementation', contentType: 'image/png', sha256: 'b'.repeat(64), base64: 'aGVsbG8=' },
    ],
    scope: { enabled: false, focus: '' },
    designStructure: { id: '1:1', name: '页面', type: 'FRAME', bounds: { x: 0, y: 0, width: 1440, height: 900 } },
    implementationStructure: {
      source: 'web', actualUrl: 'https://example.com/review', elements: [], structureTruncated: true,
      syntheticMobileTopbar: null, contentViewport: { width: 1440, height: 900 }, finalViewport: { width: 1440, height: 900 },
    },
    recheck: {
      version: '1.0', sourceReviewId, chainRootReviewId,
      items: [{
        historicalDifferenceId, lineageRootDifferenceId: historicalDifferenceId, latestDifferenceId: historicalDifferenceId,
        title: '按钮偏移', description: '按钮向右偏移', recommendation: '按设计稿对齐', category: 'layout', severity: 'major',
        designRegion: region, implementationRegion: region, firstSeenReviewId: sourceReviewId, latestReviewId: sourceReviewId,
        persistenceRounds: 1, lineageSummary: '首轮发现按钮位置偏移',
      }],
      lineages: [{
        lineageId, latestDifferenceId: historicalDifferenceId, lineageRootDifferenceId: historicalDifferenceId,
        verificationStatus: 'confirmed', resolutionStatus: 'unresolved', title: '按钮偏移', description: '按钮向右偏移',
        category: 'layout', severity: 'major', designRegion: region, implementationRegion: region,
        designAnchor: null, implementationAnchor: null,
      }],
    },
  };
}

function validOutput(): DesignRecheckOutput {
  return {
    currentDifferences: [{
      candidateId: 'current-1', discoverySource: 'full_page', category: 'layout', severity: 'major', title: '按钮偏移',
      description: '按钮向右偏移', recommendation: '按设计稿对齐', confidence: 0.96,
      designRegion: region, implementationRegion: region, evidenceNotes: '两侧坐标可以直接比较',
    }],
    historicalVerifications: [{
      historicalDifferenceId, status: 'still_present', currentDifferenceCandidateId: 'current-1', confidence: 0.96,
      reason: '当前证据仍可见', evidenceSummary: '实现按钮相对设计稿向右偏移',
      currentDesignRegion: region, currentImplementationRegion: region,
    }],
    lineageMatches: [{ candidateId: 'current-1', lineageId, confidence: 0.96, reason: '类别、标题和区域一致' }],
    coverageComplete: true, coverageNotes: ['已检查完整页面'], summary: '历史差异仍然存在',
  };
}

describe('design review recheck contracts', () => {
  it('uses stable prompt versions for first analysis and rechecks', () => {
    expect(DESIGN_REVIEW_PROMPT_VERSION).toBe('design-review-v1');
    expect(DESIGN_RECHECK_PROMPT_VERSION).toBe('design-recheck-v1');
  });

  it('accepts a complete recheck request and output', () => {
    expect(designReviewRequestSchema.safeParse(validRequest()).success).toBe(true);
    expect(designRecheckOutputSchema.safeParse(validOutput()).success).toBe(true);
  });

  it('accepts a consistent DOM normalization report and rejects forged counts', () => {
    const request = validRequest();
    request.normalizationManifest = {
      requested: true,
      report: {
        plan: { model: 'vision-model', prompt_version: 'v1', already_aligned: false, mismatch_detected: true, summary: 'normalized', skipped: [] },
        operations: [{ type: 'set_text', target_id: 'el-1', result: 'applied', reason: null }],
        counts: { applied: 1, skipped: 0 },
      },
    };
    expect(designReviewRequestSchema.safeParse(request).success).toBe(true);
    request.normalizationManifest.report.counts.applied = 2;
    expect(designReviewRequestSchema.safeParse(request).success).toBe(false);
  });

  it('rejects a historical verification with required evidence omitted', () => {
    const output = validOutput();
    const invalid = {
      ...output,
      historicalVerifications: output.historicalVerifications.map(({ evidenceSummary: _omitted, ...verification }) => verification),
    };
    expect(designRecheckOutputSchema.safeParse(invalid).success).toBe(false);
  });

  it('rejects regions that extend outside normalized image bounds', () => {
    const request = validRequest();
    request.recheck!.items[0]!.designRegion = { x: 0.9, y: 0.2, width: 0.3, height: 0.4 };
    expect(designReviewRequestSchema.safeParse(request).success).toBe(false);

    const output = validOutput();
    output.currentDifferences[0]!.implementationRegion = { x: 0.1, y: 0.9, width: 0.3, height: 0.4 };
    expect(designRecheckOutputSchema.safeParse(output).success).toBe(false);
  });

  it('rejects malformed and unknown candidate IDs', () => {
    const malformed = validOutput();
    malformed.currentDifferences[0]!.candidateId = 'candidate-1';
    expect(designRecheckOutputSchema.safeParse(malformed).success).toBe(false);

    const unknown = validOutput();
    unknown.lineageMatches[0]!.candidateId = 'current-2';
    expect(designRecheckOutputSchema.safeParse(unknown).success).toBe(false);
  });

  it('rejects status associations that violate recheck semantics', () => {
    const resolvedWithCandidate = validOutput();
    resolvedWithCandidate.historicalVerifications[0]!.status = 'resolved';
    expect(designRecheckOutputSchema.safeParse(resolvedWithCandidate).success).toBe(false);

    const stillPresentWithoutCandidate = validOutput();
    stillPresentWithoutCandidate.historicalVerifications[0]!.currentDifferenceCandidateId = null;
    expect(designRecheckOutputSchema.safeParse(stillPresentWithoutCandidate).success).toBe(false);

    const unreferencedRecovery = validOutput();
    unreferencedRecovery.currentDifferences[0]!.discoverySource = 'history_recovered';
    unreferencedRecovery.historicalVerifications[0]!.status = 'uncertain';
    unreferencedRecovery.historicalVerifications[0]!.currentDifferenceCandidateId = null;
    expect(designRecheckOutputSchema.safeParse(unreferencedRecovery).success).toBe(false);
  });

  it('requires both design and implementation structure evidence', () => {
    const { designStructure: _design, ...withoutDesign } = validRequest();
    expect(designReviewRequestSchema.safeParse(withoutDesign).success).toBe(false);

    const { implementationStructure: _implementation, ...withoutImplementation } = validRequest();
    expect(designReviewRequestSchema.safeParse(withoutImplementation).success).toBe(false);
  });

  it('rejects unknown fields in web and native elements', () => {
    const web = validRequest();
    web.implementationStructure = {
      ...web.implementationStructure,
      elements: [{
        id: 'web-1', parent_id: null, tag: 'button', role: 'button', text: '保存',
        bounds: { x: 10, y: 10, width: 100, height: 40 }, styles: {}, clipped: false,
        structural_fingerprint: 'stable-button', control_state: null, allowed_operations: [], unexpected: true,
      }],
    } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(web).success).toBe(false);

    const native = validRequest();
    native.implementationStructure = {
      source: 'mobile',
      target: {
        kind: 'mobile', platform: 'ios', app_id: 'com.example.app', app_name: null, app_version: null,
        orientation: 'portrait', device: {}, screen: { width: 390, height: 844, scale: 3 },
        collector_version: '0.1.0', mobilewright_version: '0.0.53',
      },
      elements: [{
        id: 'native-1', parent_id: null, native_type: 'Button', role: 'button', label: '保存', text: null,
        value: null, identifier: null, resource_id: null, bounds: { x: 10, y: 10, width: 100, height: 40 },
        state: { visible: true, enabled: true, selected: null, focused: null, checked: null },
        clipped: false, structural_fingerprint: 'stable-native-button', unexpected: true,
      }],
      structureTruncated: false, syntheticMobileTopbar: null,
      contentViewport: { width: 390, height: 844 }, finalViewport: { width: 390, height: 844 },
    } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(native).success).toBe(false);
  });

  it('rejects oversized, orphaned, duplicate, and out-of-bounds element inventories', () => {
    const element = {
      id: 'web-1', parent_id: null, tag: 'main', role: null, text: null,
      bounds: { x: 0, y: 0, width: 1440, height: 900 }, styles: {}, clipped: false,
      structural_fingerprint: 'stable-main', control_state: null, allowed_operations: [] as const,
    };
    const oversized = validRequest();
    oversized.implementationStructure = { ...oversized.implementationStructure, elements: Array.from({ length: 1501 }, (_, index) => ({ ...element, id: `web-${index + 1}` })) } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(oversized).success).toBe(false);

    const orphan = validRequest();
    orphan.implementationStructure = { ...orphan.implementationStructure, elements: [{ ...element, parent_id: 'missing' }] } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(orphan).success).toBe(false);

    const duplicate = validRequest();
    duplicate.implementationStructure = { ...duplicate.implementationStructure, elements: [element, { ...element }] } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(duplicate).success).toBe(false);

    const outside = validRequest();
    outside.implementationStructure = { ...outside.implementationStructure, elements: [{ ...element, bounds: { x: 1400, y: 0, width: 100, height: 40 } }] } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(outside).success).toBe(false);
  });

  it('accepts intersecting clipped bounds and rejects invalid clipping claims', () => {
    const element = {
      id: 'web-1', parent_id: null, tag: 'body', role: null, text: null,
      bounds: { x: 0, y: 0, width: 1440, height: 1802 }, styles: {}, clipped: true,
      structural_fingerprint: 'stable-body', control_state: null, allowed_operations: [] as const,
    };
    const clipped = validRequest();
    clipped.implementationStructure = {
      ...clipped.implementationStructure,
      elements: [element, {
        ...element, id: 'web-2', parent_id: 'web-1', tag: 'svg',
        bounds: { x: -120, y: 40, width: 300, height: 300 },
      }],
    } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(clipped).success).toBe(true);

    const unmarkedOverflow = validRequest();
    unmarkedOverflow.implementationStructure = {
      ...unmarkedOverflow.implementationStructure,
      elements: [{ ...element, bounds: { x: -10, y: 10, width: 100, height: 40 }, clipped: false }],
    } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(unmarkedOverflow).success).toBe(false);

    const fullyOutside = validRequest();
    fullyOutside.implementationStructure = {
      ...fullyOutside.implementationStructure,
      elements: [{ ...element, bounds: { x: -200, y: 10, width: 100, height: 40 } }],
    } as unknown as DesignReviewRequest['implementationStructure'];
    expect(designReviewRequestSchema.safeParse(fullyOutside).success).toBe(false);
  });

  it('accepts nullable mobile app metadata and enforces matching screen dimensions', () => {
    const request = validRequest();
    request.implementationStructure = {
      source: 'mobile',
      target: {
        kind: 'mobile', platform: 'android', app_id: 'com.example.app', app_name: null, app_version: null,
        orientation: 'portrait', device: {}, screen: { width: 390, height: 844, scale: 3 },
        collector_version: '0.1.0', mobilewright_version: '0.0.53',
      },
      elements: [], structureTruncated: true, syntheticMobileTopbar: null,
      contentViewport: { width: 390, height: 844 }, finalViewport: { width: 390, height: 844 },
    };
    expect(designReviewRequestSchema.safeParse(request).success).toBe(true);
    (request.implementationStructure.target.screen as Record<string, unknown>).width = 391;
    expect(designReviewRequestSchema.safeParse(request).success).toBe(false);
  });
});
