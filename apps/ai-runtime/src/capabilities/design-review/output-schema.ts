import { z } from 'zod';

import { designReviewRegionSchema } from './request-schema.js';

export const designDifferenceSchema = z.object({
  category: z.enum(['layout', 'size', 'typography', 'color', 'component', 'icon_image', 'content', 'effect']),
  severity: z.enum(['critical', 'major', 'minor']),
  title: z.string().min(1).max(160),
  description: z.string().min(1).max(4000),
  recommendation: z.string().min(1).max(4000),
  confidence: z.number().min(0).max(1),
  designRegion: designReviewRegionSchema.nullable(),
  implementationRegion: designReviewRegionSchema.nullable(),
  evidenceNotes: z.string().max(2000),
}).strict();

export const designReviewOutputSchema = z.object({
  summary: z.string().min(1).max(4000),
  modelCoverageComplete: z.boolean(),
  differences: z.array(designDifferenceSchema).max(100),
}).strict();

const recheckCurrentDifferenceSchema = designDifferenceSchema.extend({
  candidateId: z.string().regex(/^current-[1-9]\d*$/),
  discoverySource: z.enum(['full_page', 'history_recovered']),
}).strict();

const historicalVerificationSchema = z.object({
  historicalDifferenceId: z.uuid(),
  status: z.enum(['still_present', 'resolved', 'uncertain', 'not_applicable']),
  currentDifferenceCandidateId: z.string().regex(/^current-[1-9]\d*$/).nullable(),
  confidence: z.number().min(0).max(1), reason: z.string().min(1).max(2000),
  evidenceSummary: z.string().min(1).max(2000),
  currentDesignRegion: designReviewRegionSchema.nullable(),
  currentImplementationRegion: designReviewRegionSchema.nullable(),
}).strict();

const lineageMatchSchema = z.object({
  candidateId: z.string().regex(/^current-[1-9]\d*$/), lineageId: z.uuid().nullable(),
  confidence: z.number().min(0).max(1), reason: z.string().min(1).max(2000),
}).strict();

export const designRecheckOutputSchema = z.object({
  currentDifferences: z.array(recheckCurrentDifferenceSchema).max(100),
  historicalVerifications: z.array(historicalVerificationSchema).max(200),
  lineageMatches: z.array(lineageMatchSchema).max(100),
  coverageComplete: z.boolean(), coverageNotes: z.array(z.string().min(1).max(500)).max(20),
  summary: z.string().min(1).max(4000),
}).strict().superRefine((output, context) => {
  const candidates = new Map(output.currentDifferences.map((difference) => [difference.candidateId, difference]));
  if (candidates.size !== output.currentDifferences.length) {
    context.addIssue({ code: 'custom', path: ['currentDifferences'], message: 'candidateId must be unique' });
  }

  const historicalIds = new Set<string>();
  const linkedCandidates = new Set<string>();
  for (const [index, verification] of output.historicalVerifications.entries()) {
    if (historicalIds.has(verification.historicalDifferenceId)) {
      context.addIssue({ code: 'custom', path: ['historicalVerifications', index, 'historicalDifferenceId'], message: 'historicalDifferenceId must be unique' });
    }
    historicalIds.add(verification.historicalDifferenceId);
    const linkedCandidateId = verification.currentDifferenceCandidateId;
    if (verification.status === 'still_present') {
      if (!linkedCandidateId || !candidates.has(linkedCandidateId)) {
        context.addIssue({ code: 'custom', path: ['historicalVerifications', index, 'currentDifferenceCandidateId'], message: 'still_present must reference a current candidate' });
      } else if (linkedCandidates.has(linkedCandidateId)) {
        context.addIssue({ code: 'custom', path: ['historicalVerifications', index, 'currentDifferenceCandidateId'], message: 'a current candidate can verify only one historical difference' });
      } else {
        linkedCandidates.add(linkedCandidateId);
      }
    } else if (linkedCandidateId !== null) {
      context.addIssue({ code: 'custom', path: ['historicalVerifications', index, 'currentDifferenceCandidateId'], message: 'only still_present may reference a current candidate' });
    }
  }

  const matchedCandidates = new Set<string>();
  const matchedLineages = new Set<string>();
  for (const [index, match] of output.lineageMatches.entries()) {
    if (!candidates.has(match.candidateId)) {
      context.addIssue({ code: 'custom', path: ['lineageMatches', index, 'candidateId'], message: 'lineage match must reference a current candidate' });
    }
    if (matchedCandidates.has(match.candidateId)) {
      context.addIssue({ code: 'custom', path: ['lineageMatches', index, 'candidateId'], message: 'each current candidate requires exactly one lineage match' });
    }
    matchedCandidates.add(match.candidateId);
    if (match.lineageId !== null) {
      if (matchedLineages.has(match.lineageId)) {
        context.addIssue({ code: 'custom', path: ['lineageMatches', index, 'lineageId'], message: 'a lineage can match only one current candidate' });
      }
      matchedLineages.add(match.lineageId);
    }
  }
  for (const [candidateId, candidate] of candidates) {
    if (!matchedCandidates.has(candidateId)) {
      context.addIssue({ code: 'custom', path: ['lineageMatches'], message: `missing lineage match for ${candidateId}` });
    }
    if (candidate.discoverySource === 'history_recovered' && !linkedCandidates.has(candidateId)) {
      context.addIssue({ code: 'custom', path: ['currentDifferences'], message: `${candidateId} must be referenced by a historical verification` });
    }
  }
});

export type DesignReviewOutput = z.infer<typeof designReviewOutputSchema>;
export type DesignRecheckOutput = z.infer<typeof designRecheckOutputSchema>;
