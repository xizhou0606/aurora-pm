import {
  aiNodeTypeForCapability,
  comparableValue,
  historicalWorkflowNodes,
  sameStructuredValue,
  workflowAuthoringNodeTypeSet,
  workflowNodeCapabilitiesBySubject,
} from './normalize.js';
import type { AIOutput } from '../../shared/ai/schema.js';

export type WorkflowDefinition = NonNullable<AIOutput['workflowProposal']>['definition'];

export function workflowProposalIssues(definition: WorkflowDefinition, currentDefinition?: unknown, subjectType = 'requirement'): string[] {
  const issues: string[] = [];
  const nodes = definition.nodes;
  const edges = definition.edges;
  const historicalNodes = historicalWorkflowNodes(currentDefinition);
  const proposedNodes = new Map(nodes.map((node) => [node.id, node]));
  const nodeIDs = new Set<string>();
  const edgeIDs = new Set<string>();
  const outgoing = new Map<string, typeof edges>();
  const incoming = new Map<string, typeof edges>();
  for (const node of nodes) {
    if (nodeIDs.has(node.id)) issues.push(`节点 ID 重复：${node.id}`);
    nodeIDs.add(node.id);
    const historicalNode = historicalNodes.get(node.id);
    if (!workflowAuthoringNodeTypeSet.has(node.type) && historicalNode?.type !== node.type) {
      issues.push(`节点“${node.name}”使用当前编排器不支持的类型：${node.type}`);
    }
    if (node.type === 'ai_check' || node.type === 'ai_generate') {
      if (subjectType === 'plan_unit' && node.type === 'ai_generate') {
        issues.push(`项目/版本流程暂不开放 AI 生成节点：${node.name}`);
        continue;
      }
      const capability = node.capability ?? '';
      if (!workflowNodeCapabilitiesBySubject[subjectType]?.has(capability)) {
        issues.push(`AI 节点“${node.name}”使用未开放能力：${capability || '未配置'}`);
        continue;
      }
      const expectedType = aiNodeTypeForCapability(capability);
      if (expectedType !== node.type) {
        issues.push(`AI 节点“${node.name}”的类型与能力不匹配：${capability}`);
      }
    }
  }
  for (const historicalNode of historicalNodes.values()) {
    const proposed = proposedNodes.get(historicalNode.id);
    if (!proposed) {
      issues.push(`历史隐藏节点“${String(historicalNode.name || historicalNode.id)}”不得删除`);
      continue;
    }
    if (proposed.type !== historicalNode.type) {
      issues.push(`历史隐藏节点“${String(historicalNode.name || historicalNode.id)}”的类型必须原样保留`);
      continue;
    }
    if (proposed.name !== historicalNode.name) {
      issues.push(`历史隐藏节点“${String(historicalNode.name || historicalNode.id)}”的名称必须原样保留`);
    }
    if (!sameStructuredValue(proposed.capability, historicalNode.capability)) {
      issues.push(`历史隐藏节点“${String(historicalNode.name || historicalNode.id)}”的 capability 必须原样保留`);
    }
    if (!sameStructuredValue(proposed.config, historicalNode.config)) {
      issues.push(`历史隐藏节点“${String(historicalNode.name || historicalNode.id)}”的 config 必须原样保留`);
    }
  }
  for (const edge of edges) {
    if (edgeIDs.has(edge.id)) issues.push(`连线 ID 重复：${edge.id}`);
    edgeIDs.add(edge.id);
    if (!nodeIDs.has(edge.source) || !nodeIDs.has(edge.target)) {
      issues.push(`连线“${edge.id}”引用了不存在的节点`);
      continue;
    }
    outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge]);
    incoming.set(edge.target, [...(incoming.get(edge.target) ?? []), edge]);
  }
  const forwardEdges = (nodeID: string) => (outgoing.get(nodeID) ?? []).filter((edge) => edge.kind !== 'failure' && edge.kind !== 'reject');
  const canReach = (from: string, goal: string, visited = new Set<string>()): boolean => {
    if (from === goal) return true;
    if (visited.has(from)) return false;
    const nextVisited = new Set(visited).add(from);
    return forwardEdges(from).some((edge) => canReach(edge.target, goal, nextVisited));
  };

  const starts = nodes.filter((node) => node.type === 'start');
  const ends = nodes.filter((node) => node.type === 'end');
  if (starts.length !== 1) issues.push('必须且只能有一个开始节点');
  if (ends.length !== 1) issues.push('必须且只能有一个结束节点');

  if (starts.length === 1) {
    const reachable = new Set<string>();
    const visit = (nodeID: string) => {
      if (reachable.has(nodeID)) return;
      reachable.add(nodeID);
      for (const edge of outgoing.get(nodeID) ?? []) visit(edge.target);
    };
    visit(starts[0].id);
    for (const node of nodes) {
      if (!reachable.has(node.id)) issues.push(`节点“${node.name}”无法从开始节点到达`);
    }
  }

  for (const node of nodes) {
    if (node.type === 'start' && (incoming.get(node.id)?.length ?? 0) > 0) issues.push(`开始节点“${node.name}”不能有入边`);
    if (node.type !== 'start' && !(incoming.get(node.id)?.length)) issues.push(`节点“${node.name}”没有前驱`);
    if (node.type === 'end' && (outgoing.get(node.id)?.length ?? 0) > 0) issues.push(`结束节点“${node.name}”不能有出边`);
    if (node.type !== 'end' && !(outgoing.get(node.id)?.length)) issues.push(`节点“${node.name}”没有后继`);
    if (!['condition', 'parallel_split'].includes(node.type) && forwardEdges(node.id).length > 1) {
      issues.push(`节点“${node.name}”使用多条向前连线伪装并行`);
    }
    if (node.type !== 'end' && !nodes.some((candidate) => candidate.type === 'end' && canReach(node.id, candidate.id))) {
      issues.push(`节点“${node.name}”无法到达结束节点`);
    }
  }

  const splits = nodes.filter((node) => node.type === 'parallel_split');
  const joins = nodes.filter((node) => node.type === 'parallel_join');
  for (const split of splits) {
    const branches = forwardEdges(split.id);
    if (branches.length < 2) {
      issues.push(`并行分支“${split.name}”至少需要两条支路`);
      continue;
    }
    const matchingJoins = joins.filter((join) => branches.every((branch) => canReach(branch.target, join.id)));
    if (matchingJoins.length !== 1) issues.push(`并行分支“${split.name}”必须且只能汇入一个共同汇聚节点`);
  }
  for (const join of joins) {
    if ((incoming.get(join.id)?.filter((edge) => edge.kind !== 'failure' && edge.kind !== 'reject').length ?? 0) < 2) {
      issues.push(`并行汇聚“${join.name}”至少需要两条输入支路`);
    }
    const matchingSplits = splits.filter((split) => forwardEdges(split.id).length >= 2 && forwardEdges(split.id).every((branch) => canReach(branch.target, join.id)));
    if (!matchingSplits.length) issues.push(`并行汇聚“${join.name}”没有匹配的并行分支`);
  }
  return [...new Set(issues)];
}
