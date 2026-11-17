import { aiOutputSchema, historicalWorkflowNodeTypes, workflowAuthoringNodeTypes, type AIOutput } from '../../shared/ai/schema.js';

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

export const workflowAuthoringNodeTypeSet = new Set<string>(workflowAuthoringNodeTypes);
const preservedHistoricalNodeTypeSet = new Set(['form', 'timer', 'outbox_action', 'create_drafts']);

// Existing definitions remain parseable as generation context. New proposals
// are validated against the smaller authoring set before leaving this runtime.
const historicalWorkflowNodeTypeSet = new Set<string>(historicalWorkflowNodeTypes);

type AICapabilityMode = 'analysis' | 'proposal' | 'artifact';

const aiCapabilityModes = new Map<string, AICapabilityMode>([
  ['version.scope_proposal', 'proposal'],
  ['requirement.draft', 'proposal'],
  ['requirement.triage', 'proposal'],
  ['requirement.duplicate_check', 'analysis'],
  ['requirement.acceptance_criteria', 'proposal'],
  ['requirement.completeness', 'analysis'],
  ['requirement.design_classify', 'analysis'],
  ['design.requirement_coverage', 'analysis'],
  ['requirement.review_pack', 'artifact'],
  ['meeting.extract_decisions', 'artifact'],
  ['workflow.generate', 'proposal'],
  ['workflow.lint_explain', 'analysis'],
  ['workflow.simulate', 'analysis'],
  ['requirement.task_plan', 'artifact'],
  ['requirement.acceptance_cases', 'artifact'],
  ['delivery.schedule_proposal', 'proposal'],
  ['delivery.risk_monitor', 'analysis'],
  ['delivery.daily_brief', 'artifact'],
  ['delivery.change_impact', 'analysis'],
  ['delivery.test_handoff', 'artifact'],
  ['delivery.fix_plan', 'proposal'],
  ['qa.test_cases', 'artifact'],
  ['qa.result_summary', 'artifact'],
  ['ui.design_compare', 'analysis'],
  ['verification.aggregate', 'artifact'],
  ['release.readiness', 'analysis'],
  ['release.notes', 'artifact'],
  ['project.retrospective', 'artifact'],
]);

// Product-approved workflow-node capabilities. Other runtime capabilities are
// intentionally unavailable in template authoring.
export const workflowNodeCapabilitiesBySubject: Record<string, Set<string>> = {
  requirement: new Set(['requirement.completeness', 'requirement.task_plan']),
  plan_unit: new Set(['delivery.risk_monitor']),
};

export function aiNodeTypeForCapability(value: unknown): 'ai_check' | 'ai_generate' | undefined {
  const capability = typeof value === 'string' ? value.trim() : '';
  const mode = aiCapabilityModes.get(capability);
  if (mode) return mode === 'analysis' ? 'ai_check' : 'ai_generate';
  const semantic = capability.toLowerCase();
  if (/(?:check|classif|compar|completeness|coverage|risk|lint|simulate|triage|duplicate|readiness|verif)/.test(semantic)) {
    return 'ai_check';
  }
  if (/(?:generat|draft|proposal|plan|breakdown|extract|summar|brief|handoff|notes|retro|case|criteria)/.test(semantic)) {
    return 'ai_generate';
  }
  return undefined;
}

function normalizeNodeType(value: unknown, capability?: unknown, name?: unknown): string {
  const raw = typeof value === 'string' ? value.trim().toLowerCase() : '';
  if (/^ai(?:[_\s-]|$)/.test(raw)) {
    if (raw === 'ai_check' || raw === 'ai_generate') return raw;
    const capabilityType = aiNodeTypeForCapability(capability);
    if (capabilityType) return capabilityType;
    if (raw.includes('classif') || raw.includes('compar') || raw.includes('check')) return 'ai_check';
    if (raw.includes('generat') || raw.includes('summar') || raw.includes('extract') || raw.includes('plan')) return 'ai_generate';
    const semantics = `${typeof name === 'string' ? name : ''} ${raw}`;
    if (/(检查|校验|分类|判断|比对|比较|评估|监控|门禁)/.test(semantics)) return 'ai_check';
    if (/(生成|拆解|规划|计划|提取|汇总|总结|草稿|摘要)/.test(semantics)) return 'ai_generate';
    return 'ai_check';
  }
  if (historicalWorkflowNodeTypeSet.has(raw)) return raw;
  if (raw.includes('approv') || raw.includes('review') || raw.includes('gate')) return 'approval';
  if (raw.includes('condition') || raw.includes('decision')) return 'condition';
  if (raw.includes('form')) return 'form';
  if (raw.includes('timer') || raw.includes('wait')) return 'timer';
  return 'human_task';
}

function normalizeCapability(value: unknown, nodeType: string, subjectType: string): string | undefined {
  const raw = typeof value === 'string' ? value.trim() : '';
  if (!nodeType.startsWith('ai_')) return undefined;
  const mode = aiCapabilityModes.get(raw);
  const modeMatchesNode = nodeType === 'ai_check' ? mode === 'analysis' : mode !== undefined && mode !== 'analysis';
  if (workflowNodeCapabilitiesBySubject[subjectType]?.has(raw) && modeMatchesNode) return raw;
  return undefined;
}

type NormalizedWorkflowEdge = {
  id?: unknown;
  source?: unknown;
  target?: unknown;
  kind?: string;
  condition?: string;
  loop?: true;
  [key: string]: unknown;
};

type HistoricalWorkflowNode = {
  id: string;
  type: string;
  name?: unknown;
  capability?: unknown;
  config?: unknown;
};

export function historicalWorkflowNodes(value: unknown): Map<string, HistoricalWorkflowNode> {
  const definition = record(value);
  const nodes = Array.isArray(definition.nodes) ? definition.nodes : [];
  return new Map(nodes.flatMap((value) => {
    const node = record(value);
    const id = typeof node.id === 'string' ? node.id : '';
    const type = typeof node.type === 'string' ? node.type : '';
    if (!id || !preservedHistoricalNodeTypeSet.has(type)) return [];
    return [[id, { id, type, name: node.name, capability: node.capability, config: node.config }] as const];
  }));
}

export function comparableValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(comparableValue);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value as Record<string, unknown>)
    .filter(([, item]) => item !== undefined)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, item]) => [key, comparableValue(item)]));
}

export function sameStructuredValue(left: unknown, right: unknown): boolean {
  return JSON.stringify(comparableValue(left)) === JSON.stringify(comparableValue(right));
}

function normalizeEdgeCondition(edge: NormalizedWorkflowEdge): NormalizedWorkflowEdge {
  const condition = edge.condition?.trim().toLowerCase();
  if (!condition || edge.kind) return edge;
  const approveAliases = new Set(['approve', 'approved', 'pass', 'passed', 'yes', 'ui_needed']);
  const rejectAliases = new Set(['reject', 'rejected', 'deny', 'denied', 'no', 'no_ui_needed']);
  if (approveAliases.has(condition)) {
    const { condition: _condition, ...rest } = edge;
    return { ...rest, kind: 'approve' };
  }
  if (rejectAliases.has(condition)) {
    const { condition: _condition, ...rest } = edge;
    return { ...rest, kind: 'reject' };
  }
  if (/^[a-z_][a-z0-9_]*$/i.test(condition)) {
    return { ...edge, condition: `context.${condition} == true` };
  }
  return edge;
}

function markCycleEdges(edges: NormalizedWorkflowEdge[]): NormalizedWorkflowEdge[] {
  const outgoing = new Map<string, string[]>();
  for (const edge of edges) {
    if (typeof edge.source !== 'string' || typeof edge.target !== 'string') continue;
    outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge.target]);
  }
  const canReach = (from: string, goal: string, visited = new Set<string>()): boolean => {
    if (from === goal) return true;
    if (visited.has(from)) return false;
    visited.add(from);
    return (outgoing.get(from) ?? []).some((next) => canReach(next, goal, visited));
  };
  return edges.map((edge) => {
    if (typeof edge.source !== 'string' || typeof edge.target !== 'string') return edge;
    return canReach(edge.target, edge.source) ? { ...edge, loop: true } : edge;
  });
}

function ensureAIFailureBranches(nodes: Array<Record<string, unknown>>, edges: NormalizedWorkflowEdge[]) {
  const repairedNodes = [...nodes];
  const repairedEdges = [...edges];
  const nodeIDs = new Set(nodes.map((node) => String(node.id)));
  const edgeIDs = new Set(edges.map((edge) => String(edge.id)));
  const uniqueID = (base: string, used: Set<string>) => {
    let value = base.slice(0, 90);
    let suffix = 2;
    while (used.has(value)) value = `${base.slice(0, 84)}-${suffix++}`;
    used.add(value);
    return value;
  };
  for (const node of nodes) {
    const nodeID = typeof node.id === 'string' ? node.id : '';
    const nodeType = typeof node.type === 'string' ? node.type : '';
    if (!nodeID || !nodeType.startsWith('ai_')) continue;
    const hasFailure = repairedEdges.some((edge) => edge.source === nodeID && edge.kind === 'failure');
    if (hasFailure) continue;
    const fallbackID = uniqueID(`ai-fallback-${nodeID}`, nodeIDs);
    repairedNodes.push({ id: fallbackID, type: 'human_task', name: `人工处理：${String(node.name || 'AI 节点')}失败` });
    repairedEdges.push(
      { id: uniqueID(`e-${nodeID}-failure`, edgeIDs), source: nodeID, target: fallbackID, kind: 'failure' },
      { id: uniqueID(`e-${fallbackID}-retry`, edgeIDs), source: fallbackID, target: nodeID, loop: true },
    );
  }
  return { nodes: repairedNodes, edges: markCycleEdges(repairedEdges) };
}

export function parseWorkflowOutput(text: string, subjectType = 'requirement', currentDefinition?: unknown): AIOutput {
  const cleaned = text.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
  const value = JSON.parse(cleaned) as Record<string, unknown>;
  const proposal = (value.workflowProposal ?? value.workflow_proposal ?? value.proposal) as Record<string, unknown> | null | undefined;
  const definition = (proposal?.definition ?? proposal?.workflowDefinition ?? proposal?.workflow_definition ?? proposal) as Record<string, unknown> | undefined;
  const historicalNodes = historicalWorkflowNodes(currentDefinition);
  const nodes = Array.isArray(definition?.nodes) ? definition.nodes.map((raw) => {
    const node = raw as Record<string, unknown>;
    const { capability: _capability, config: _config, ...nodeBase } = node;
    const nodeType = normalizeNodeType(node.type, node.capability, node.name);
    const config = node.config && typeof node.config === 'object' ? node.config as Record<string, unknown> : null;
    const historicalNode = typeof node.id === 'string' ? historicalNodes.get(node.id) : undefined;
    if (historicalNode) {
      return {
        ...nodeBase,
        type: typeof node.type === 'string' ? node.type : nodeType,
        ...(typeof node.capability === 'string' ? { capability: node.capability } : {}),
        ...(config ? { config } : {}),
      };
    }
    const normalizedConfig: Record<string, unknown> = {};
    if (typeof config?.critical_gate === 'boolean') normalizedConfig.critical_gate = config.critical_gate;
    if (typeof config?.delay_seconds === 'number') normalizedConfig.delay_seconds = config.delay_seconds;
    if (typeof config?.require_design_image === 'boolean') normalizedConfig.require_design_image = config.require_design_image;
    if (typeof config?.test_case_artifact === 'boolean') normalizedConfig.test_case_artifact = config.test_case_artifact;
    if (typeof config?.test_report_artifact === 'boolean') normalizedConfig.test_report_artifact = config.test_report_artifact;
    return {
      ...nodeBase,
      type: nodeType,
      ...(normalizeCapability(node.capability, nodeType, subjectType) ? { capability: normalizeCapability(node.capability, nodeType, subjectType) } : {}),
      ...(Object.keys(normalizedConfig).length ? { config: normalizedConfig } : {}),
    };
  }) : [];
  const normalizedEdges = Array.isArray(definition?.edges) ? definition.edges.map((raw) => {
    const edge = raw as Record<string, unknown>;
    const { kind: _kind, condition: _condition, loop: _loop, ...edgeBase } = edge;
    return normalizeEdgeCondition({
      ...edgeBase,
      ...(typeof edge.kind === 'string' ? { kind: edge.kind } : {}),
      ...(typeof edge.condition === 'string' ? { condition: edge.condition } : {}),
      ...(edge.loop === true ? { loop: true } : {}),
    });
  }) : [];
  const graph = ensureAIFailureBranches(nodes, normalizedEdges);
  return aiOutputSchema.parse({
    summary: typeof value.summary === 'string' && value.summary.trim() ? value.summary : '已生成流程草案',
    // Workflow generation only applies a proposal after a separate compiler
    // check. Generic field changes/items are irrelevant and models sometimes
    // fill them with node-shaped objects, so never let those extras reject an
    // otherwise valid workflow.
    changes: [],
    items: [],
    risks: Array.isArray(value.risks) ? value.risks.filter((risk): risk is string => typeof risk === 'string') : [],
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: proposal ? {
      mode: proposal.mode === 'merge' ? 'merge' : 'replace',
      explanation: typeof proposal.explanation === 'string' ? proposal.explanation : '',
      warnings: Array.isArray(proposal.warnings) ? proposal.warnings : [],
      definition: graph,
    } : null,
  });
}
