import { z } from 'zod';

export const designReviewRegionSchema = z.object({
  x: z.number().min(0).max(1),
  y: z.number().min(0).max(1),
  width: z.number().positive().max(1),
  height: z.number().positive().max(1),
}).strict().refine((region) => region.x + region.width <= 1.000001)
  .refine((region) => region.y + region.height <= 1.000001);

const evidenceSchema = z.object({
  kind: z.enum(['design', 'implementation']),
  contentType: z.enum(['image/png', 'image/jpeg']),
  sha256: z.string().regex(/^[a-f0-9]{64}$/),
  base64: z.string().min(1).max(48 * 1024 * 1024),
}).strict();

const structureBoundsSchema = z.object({
  x: z.number().finite(), y: z.number().finite(), width: z.number().finite().nonnegative(), height: z.number().finite().nonnegative(),
}).strict();

const designStructureNodeSchema: z.ZodType<unknown> = z.lazy(() => z.object({
  id: z.string().min(1).max(200), name: z.string().max(500), type: z.string().min(1).max(120),
  visible: z.boolean().optional(), bounds: structureBoundsSchema.optional(),
  fills: z.unknown().optional(), strokes: z.unknown().optional(), effects: z.unknown().optional(), style: z.unknown().optional(),
  layout: z.object({
    mode: z.string().max(80).optional(), gap: z.number().finite().optional(),
    padding: z.array(z.number().finite().nullable()).length(4).optional(), cornerRadius: z.number().finite().optional(),
  }).strict().optional(),
  text: z.string().max(500).optional(), children: z.array(designStructureNodeSchema).max(200).optional(),
  childrenTruncated: z.boolean().optional(), truncated: z.boolean().optional(),
}).strict());

const structureWarningSchema = z.object({ code: z.string().min(1).max(80), message: z.string().min(1).max(500) }).strict();
const structureViewportSchema = z.object({ width: z.number().int().positive().max(7680), height: z.number().int().positive().max(7680) }).strict();
const webControlStateSchema = z.object({
  checked: z.boolean().nullable(), aria_checked: z.string().max(80).nullable(), aria_pressed: z.string().max(80).nullable(),
  aria_selected: z.string().max(80).nullable(), data_state: z.string().max(80).nullable(),
}).strict();
const webStructureElementSchema = z.object({
  id: z.string().min(1).max(200), parent_id: z.string().max(200).nullable(), tag: z.string().min(1).max(80), role: z.string().max(80).nullable(),
  text: z.string().max(500).nullable(), bounds: structureBoundsSchema, styles: z.record(z.string(), z.string().max(500)), clipped: z.boolean(),
  structural_fingerprint: z.string().min(8).max(200), control_state: webControlStateSchema.nullable(),
  allowed_operations: z.array(z.enum(['set_text', 'set_control_state', 'repeat_template'])).max(3),
}).strict();
const nativeStructureElementSchema = z.object({
  id: z.string().min(1).max(200), parent_id: z.string().max(200).nullable(), native_type: z.string().min(1).max(120), role: z.string().max(80).nullable(),
  label: z.string().max(500).nullable(), text: z.string().max(500).nullable(), value: z.string().max(500).nullable(),
  identifier: z.string().max(300).nullable(), resource_id: z.string().max(300).nullable(), bounds: structureBoundsSchema,
  state: z.object({ visible: z.boolean(), enabled: z.boolean(), selected: z.boolean().nullable(), focused: z.boolean().nullable(), checked: z.boolean().nullable() }).strict(),
  clipped: z.boolean(), structural_fingerprint: z.string().min(8).max(200),
}).strict();
const topbarSchema = z.object({
  enabled: z.boolean(), platform: z.literal('ios').optional(), height: z.literal(44).optional(), time: z.string().max(16).optional(),
  background_color: z.string().regex(/^#[a-fA-F0-9]{6}$/).optional(), icon_theme: z.enum(['light', 'dark']).optional(),
}).strict();
const implementationStructureSchema = z.discriminatedUnion('source', [
  z.object({
    source: z.literal('web'), actualUrl: z.url().max(4000).optional(), elements: z.array(webStructureElementSchema).max(1500),
    structureTruncated: z.boolean(), syntheticMobileTopbar: topbarSchema.nullable(), contentViewport: structureViewportSchema,
    finalViewport: structureViewportSchema, warnings: z.array(structureWarningSchema).max(50).optional(),
  }).strict(),
  z.object({
    source: z.literal('mobile'), target: z.object({
      kind: z.literal('mobile'), platform: z.enum(['ios', 'android']), app_id: z.string().min(1).max(300),
      app_name: z.string().max(300).nullable().optional(), app_version: z.string().max(120).nullable().optional(),
      orientation: z.enum(['portrait', 'landscape']), device: z.record(z.string(), z.unknown()), screen: z.record(z.string(), z.unknown()),
      collector_version: z.string().min(1).max(40), mobilewright_version: z.string().min(1).max(40),
    }).strict(), elements: z.array(nativeStructureElementSchema).max(1500), structureTruncated: z.boolean(),
    syntheticMobileTopbar: z.null(), contentViewport: structureViewportSchema, finalViewport: structureViewportSchema,
    warnings: z.array(structureWarningSchema).max(50).optional(),
  }).strict(),
]).superRefine((structure, context) => {
  const finalViewport = structure.finalViewport;
  const contentViewport = structure.contentViewport;
  if (contentViewport.width !== finalViewport.width) {
    context.addIssue({ code: 'custom', path: ['contentViewport'], message: 'content and final viewport widths must match' });
  }
  if (structure.source === 'web') {
    if (structure.actualUrl && !isHTTPURL(structure.actualUrl)) {
      context.addIssue({ code: 'custom', path: ['actualUrl'], message: 'actualUrl must use HTTP or HTTPS without credentials' });
    }
    const topbar = structure.syntheticMobileTopbar;
    if (!topbar?.enabled && contentViewport.height !== finalViewport.height) {
      context.addIssue({ code: 'custom', path: ['contentViewport'], message: 'content and final viewport heights must match without a topbar' });
    }
    if (topbar?.enabled && contentViewport.height + 44 !== finalViewport.height) {
      context.addIssue({ code: 'custom', path: ['contentViewport'], message: 'enabled iOS topbar must reserve 44 pixels' });
    }
    if (topbar && !topbar.enabled && Object.keys(topbar).some((key) => key !== 'enabled')) {
      context.addIssue({ code: 'custom', path: ['syntheticMobileTopbar'], message: 'disabled topbar cannot carry presentation settings' });
    }
  } else {
    if (contentViewport.width !== finalViewport.width || contentViewport.height !== finalViewport.height) {
      context.addIssue({ code: 'custom', path: ['contentViewport'], message: 'mobile content and final viewports must match' });
    }
    const screen = structure.target.screen;
    if (screen.width !== finalViewport.width || screen.height !== finalViewport.height ||
      typeof screen.scale !== 'number' || !Number.isFinite(screen.scale) || screen.scale <= 0 || screen.scale > 8) {
      context.addIssue({ code: 'custom', path: ['target', 'screen'], message: 'mobile screen must match the final viewport and contain a valid scale' });
    }
  }

  const seen = new Set<string>();
  for (let index = 0; index < structure.elements.length; index += 1) {
    const element = structure.elements[index]!;
    if (seen.has(element.id)) {
      context.addIssue({ code: 'custom', path: ['elements', index, 'id'], message: 'element IDs must be unique' });
    }
    if (element.parent_id !== null && !seen.has(element.parent_id)) {
      context.addIssue({ code: 'custom', path: ['elements', index, 'parent_id'], message: 'parent must appear before child' });
    }
    seen.add(element.id);
    const bounds = element.bounds;
    const insideViewport = bounds.x >= 0 && bounds.y >= 0 &&
      bounds.x + bounds.width <= finalViewport.width + 1 && bounds.y + bounds.height <= finalViewport.height + 1;
    const intersectsViewport = bounds.x < finalViewport.width && bounds.y < finalViewport.height &&
      bounds.x + bounds.width > 0 && bounds.y + bounds.height > 0;
    if ((!element.clipped && !insideViewport) || (element.clipped && !intersectsViewport)) {
      context.addIssue({
        code: 'custom',
        path: ['elements', index, 'bounds'],
        message: element.clipped
          ? 'clipped element bounds must intersect the final viewport'
          : 'unclipped element bounds must stay inside the final viewport',
      });
    }
  }
});

function isHTTPURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && !parsed.username && !parsed.password;
  } catch {
    return false;
  }
}

function validateDesignStructure(root: unknown, context: z.RefinementCtx): void {
  const rootNode = root as { bounds?: { width: number; height: number }; children?: unknown[] };
  if (!rootNode.bounds || rootNode.bounds.width <= 0 || rootNode.bounds.height <= 0) {
    context.addIssue({ code: 'custom', path: ['designStructure', 'bounds'], message: 'design root requires positive bounds' });
    return;
  }
  const stack: Array<{ node: unknown; depth: number }> = [{ node: root, depth: 0 }];
  let count = 0;
  while (stack.length) {
    const current = stack.pop()!;
    count += 1;
    if (count > 2500) {
      context.addIssue({ code: 'custom', path: ['designStructure'], message: 'design structure exceeds 2500 nodes' });
      return;
    }
    if (current.depth > 13) {
      context.addIssue({ code: 'custom', path: ['designStructure'], message: 'design structure exceeds the maximum depth' });
      return;
    }
    const node = current.node as { children?: unknown[] };
    for (const child of node.children ?? []) stack.push({ node: child, depth: current.depth + 1 });
  }
}

const identityAnchorSchema = z.object({
  kind: z.enum(['figma', 'implementation']),
  stableId: z.string().min(1).max(200),
  nodeType: z.string().max(80).nullable(),
  region: designReviewRegionSchema,
}).strict();

const recheckLineageSchema = z.object({
  lineageId: z.uuid(), latestDifferenceId: z.uuid(), lineageRootDifferenceId: z.uuid(),
  verificationStatus: z.enum(['pending', 'confirmed', 'false_positive']),
  resolutionStatus: z.enum(['unresolved', 'resolved']),
  title: z.string().min(1).max(160), description: z.string().max(4000),
  category: z.enum(['layout', 'size', 'typography', 'color', 'component', 'icon_image', 'content', 'effect']),
  severity: z.enum(['critical', 'major', 'minor']),
  designRegion: designReviewRegionSchema.nullable(), implementationRegion: designReviewRegionSchema.nullable(),
  designAnchor: identityAnchorSchema.nullable(), implementationAnchor: identityAnchorSchema.nullable(),
}).strict();

const recheckItemSchema = z.object({
  historicalDifferenceId: z.uuid(), lineageRootDifferenceId: z.uuid(), latestDifferenceId: z.uuid(),
  title: z.string().min(1).max(160), description: z.string().max(4000), recommendation: z.string().max(4000),
  category: z.enum(['layout', 'size', 'typography', 'color', 'component', 'icon_image', 'content', 'effect']),
  severity: z.enum(['critical', 'major', 'minor']),
  designRegion: designReviewRegionSchema.nullable(), implementationRegion: designReviewRegionSchema.nullable(),
  firstSeenReviewId: z.uuid(), latestReviewId: z.uuid(), persistenceRounds: z.number().int().positive(),
  lineageSummary: z.string().min(1).max(1200),
}).strict();

const normalizationManifestSchema = z.object({
  requested: z.literal(true),
  report: z.object({
    plan: z.object({
      model: z.string().min(1).max(120), prompt_version: z.string().min(1).max(160),
      already_aligned: z.boolean(), mismatch_detected: z.boolean(), summary: z.string().min(1).max(2000),
      skipped: z.array(z.object({
        figma_source: z.string().min(1).max(500), target_id: z.string().regex(/^el-[1-9]\d*$/).nullable(),
        reason: z.string().min(1).max(1000),
      }).strict()).max(200),
    }).strict(),
    operations: z.array(z.object({
      type: z.enum(['set_text', 'set_control_state', 'replace_repeated_items']),
      target_id: z.string().regex(/^el-[1-9]\d*$/), result: z.enum(['applied', 'skipped']),
      reason: z.string().min(1).max(1000).nullable(),
    }).strict()).max(200),
    counts: z.object({ applied: z.number().int().nonnegative(), skipped: z.number().int().nonnegative() }).strict(),
  }).strict(),
}).strict().superRefine((manifest, context) => {
  const applied = manifest.report.operations.filter((item) => item.result === 'applied').length;
  const skipped = manifest.report.operations.length - applied + manifest.report.plan.skipped.length;
  if (manifest.report.counts.applied !== applied || manifest.report.counts.skipped !== skipped) {
    context.addIssue({ code: 'custom', path: ['report', 'counts'], message: 'normalization counts do not match the report' });
  }
  for (const [index, operation] of manifest.report.operations.entries()) {
    if ((operation.result === 'applied') !== (operation.reason === null)) {
      context.addIssue({ code: 'custom', path: ['report', 'operations', index, 'reason'], message: 'normalization result and reason are inconsistent' });
    }
  }
});

export const recheckSnapshotSchema = z.object({
  version: z.literal('1.0'), sourceReviewId: z.uuid(), chainRootReviewId: z.uuid(),
  items: z.array(recheckItemSchema).max(200), lineages: z.array(recheckLineageSchema).max(300),
}).strict();

export const designReviewRequestSchema = z.object({
  manifest: z.object({
    version: z.literal('1.0'),
    reviewId: z.uuid(),
    workspaceId: z.number().int().positive(),
    kind: z.enum(['full', 'deep']),
    model: z.string().min(1).max(120),
    reasoningEffort: z.enum(['low', 'medium', 'high']),
    imageDetail: z.enum(['low', 'high', 'auto']),
  }).strict(),
  evidence: z.array(evidenceSchema).length(2),
  scope: z.object({
    enabled: z.boolean(),
    designRegion: designReviewRegionSchema.optional(),
    implementationRegion: designReviewRegionSchema.optional(),
    focus: z.string().max(500).default(''),
  }).strict(),
  designStructure: designStructureNodeSchema,
  implementationStructure: implementationStructureSchema,
  normalizationManifest: normalizationManifestSchema.optional(),
  recheck: recheckSnapshotSchema.optional(),
}).strict().superRefine((request, context) => {
  validateDesignStructure(request.designStructure, context);
  if (request.evidence[0]?.kind === request.evidence[1]?.kind) {
    context.addIssue({ code: 'custom', path: ['evidence'], message: 'design and implementation evidence are both required' });
  }
  if (request.scope.enabled && (!request.scope.designRegion || !request.scope.implementationRegion)) {
    context.addIssue({ code: 'custom', path: ['scope'], message: 'enabled scope requires both regions' });
  }
  if (request.recheck && request.manifest.kind !== 'full') {
    context.addIssue({ code: 'custom', path: ['recheck'], message: 'recheck context is only valid for full analysis' });
  }
  if (request.normalizationManifest && request.implementationStructure.source !== 'web') {
    context.addIssue({ code: 'custom', path: ['normalizationManifest'], message: 'DOM normalization is only valid for web evidence' });
  }
});

export type DesignReviewRequest = z.infer<typeof designReviewRequestSchema>;
