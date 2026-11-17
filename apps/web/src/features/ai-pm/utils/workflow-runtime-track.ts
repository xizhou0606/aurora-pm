import { workflowEdgeDisplayLabel } from '../utils/workflow-edge-label'
import type { WorkflowInstance } from '../model/types'
import {
  orderRuntimeNodes,
  runtimeDisplayEdges,
  runtimeForkRegions,
  runtimeGraphHelpers,
  runtimeTopologyEdges,
  type WorkflowRuntimeEdge,
  type WorkflowRuntimeNode,
  type WorkflowRuntimeRegion,
} from '../utils/workflow-runtime-topology'

export type { WorkflowRuntimeNode } from '../utils/workflow-runtime-topology'

export type WorkflowRuntimeLane = {
  id: string
  label?: string
  items: WorkflowRuntimeTrackItem[]
  nodes: WorkflowRuntimeNode[]
  directTarget?: WorkflowRuntimeNode
}

export type WorkflowRuntimeTrackItem =
  | { kind: 'node'; node: WorkflowRuntimeNode }
  | { kind: 'parallel'; split: WorkflowRuntimeNode; join: WorkflowRuntimeNode; lanes: WorkflowRuntimeLane[] }
  | { kind: 'fanout'; id: string; merge: WorkflowRuntimeNode; lanes: WorkflowRuntimeLane[] }
  | { kind: 'choice'; condition: WorkflowRuntimeNode; lanes: WorkflowRuntimeLane[] }

const structuralNodeTypes = new Set(['start', 'end', 'parallel_split', 'parallel_join'])

function conditionOutcomeKey(edge: WorkflowRuntimeEdge) {
  const kind = typeof edge.kind === 'string' ? edge.kind.trim().toLowerCase() : ''
  if (kind === 'yes' || kind === 'condition_yes') return 'yes'
  if (kind === 'no' || kind === 'condition_no') return 'no'
  const condition = typeof edge.condition === 'string' ? edge.condition.trim() : ''
  const booleanOutcome = condition.match(/^[\w.[\]"'-]+\s*==\s*(true|false)$/i)?.[1]?.toLowerCase()
  if (booleanOutcome === 'true') return 'yes'
  if (booleanOutcome === 'false') return 'no'
  return edge.id
}

export function workflowRuntimeBusinessNodes(track: WorkflowRuntimeTrackItem[]) {
  const result: WorkflowRuntimeNode[] = []
  const included = new Set<string>()
  const collect = (items: WorkflowRuntimeTrackItem[]) => {
    for (const item of items) {
      if (item.kind === 'node') {
        if (!structuralNodeTypes.has(item.node.type) && !included.has(item.node.id)) {
          included.add(item.node.id)
          result.push(item.node)
        }
        continue
      }
      item.lanes.forEach((lane) => collect(lane.items))
    }
  }
  collect(track)
  return result
}

export function workflowRuntimeTrack(instance: WorkflowInstance, visibleTimeline: WorkflowRuntimeNode[]): WorkflowRuntimeTrackItem[] {
  const edges = runtimeDisplayEdges(instance, visibleTimeline)
  const nodes = orderRuntimeNodes(instance, visibleTimeline, edges)
  const fullTimeline = instance.timeline_nodes?.length ? instance.timeline_nodes : visibleTimeline
  const { canReach, nearestCommon } = runtimeGraphHelpers(fullTimeline, runtimeTopologyEdges(instance, fullTimeline))
  const outgoing = new Map<string, WorkflowRuntimeEdge[]>()
  for (const edge of edges) outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge])

  const buildRegion = (regionNodes: WorkflowRuntimeNode[], boundary?: WorkflowRuntimeNode, depth = 0): WorkflowRuntimeTrackItem[] => {
    if (depth > nodes.length) {
      return regionNodes.filter((node) => !structuralNodeTypes.has(node.type)).map((node) => ({ kind: 'node', node }))
    }
    const regionIDs = new Set(regionNodes.map((node) => node.id))
    const consumed = new Set<string>()
    const track: WorkflowRuntimeTrackItem[] = []
    const candidateNodes = boundary && !regionIDs.has(boundary.id) ? [...regionNodes, boundary] : regionNodes
    const laneFor = (region: WorkflowRuntimeRegion, merge: WorkflowRuntimeNode) => {
      const items = buildRegion(region.nodes, merge, depth + 1)
      return {
        id: region.id,
        label: region.label,
        items,
        nodes: workflowRuntimeBusinessNodes(items),
        directTarget: region.roots.every((root) => root === merge.id) ? merge : undefined,
      }
    }
    const rootsFor = (nodeID: string) => {
      const relevantEdges = (outgoing.get(nodeID) ?? []).filter((edge) => regionIDs.has(edge.target) || edge.target === boundary?.id)
      return { relevantEdges, roots: [...new Set(relevantEdges.map((edge) => edge.target))] }
    }

    for (const node of regionNodes) {
      if (consumed.has(node.id) || node.type === 'start' || node.type === 'end' || node.type === 'parallel_join') continue
      if (node.type === 'parallel_split') {
        const { roots } = rootsFor(node.id)
        const join = roots.length > 1
          ? nearestCommon(roots, candidateNodes.filter((candidate) => candidate.type === 'parallel_join'))
          : undefined
        const regions = join ? runtimeForkRegions(roots.map((id) => ({ id, roots: [id] })), join, regionNodes, canReach, node.id, edges) : undefined
        if (!join || !regions) continue
        const lanes = regions.map((region) => laneFor(region, join))
        consumed.add(node.id)
        if (regionIDs.has(join.id)) consumed.add(join.id)
        regions.forEach((region) => region.nodes.forEach((laneNode) => consumed.add(laneNode.id)))
        track.push({ kind: 'parallel', split: node, join, lanes })
        continue
      }

      consumed.add(node.id)
      track.push({ kind: 'node', node })
      if (node.type !== 'condition') continue
      const { relevantEdges, roots } = rootsFor(node.id)
      const branchMap = new Map<string, { id: string; label?: string; roots: string[] }>()
      for (const edge of relevantEdges) {
        const key = conditionOutcomeKey(edge)
        const branch = branchMap.get(key) ?? { id: key, label: workflowEdgeDisplayLabel(edge), roots: [] }
        if (!branch.roots.includes(edge.target)) branch.roots.push(edge.target)
        branchMap.set(key, branch)
      }
      const branches = [...branchMap.values()]
      const needsBranchGroup = branches.length > 1 || branches.some((branch) => branch.roots.length > 1)
      const merge = needsBranchGroup && roots.length
        ? nearestCommon(roots, candidateNodes.filter((candidate) => candidate.id !== node.id))
        : undefined
      const regions = merge ? runtimeForkRegions(branches, merge, regionNodes, canReach, node.id, edges) : undefined
      if (!merge || !regions) continue
      const laneWithFanout = (region: WorkflowRuntimeRegion) => {
        if (region.roots.length < 2) return laneFor(region, merge)
        const regionCandidates = region.nodes.some((candidate) => candidate.id === merge.id) ? region.nodes : [...region.nodes, merge]
        const branchMerge = nearestCommon(region.roots, regionCandidates)
        const rootRegions = branchMerge ? runtimeForkRegions(
          region.roots.map((root) => ({ id: root, roots: [root] })),
          branchMerge,
          region.nodes,
          canReach,
          node.id,
          edges,
        ) : undefined
        if (!branchMerge || !rootRegions) return laneFor(region, merge)
        const rootNodeIDs = new Set(rootRegions.flatMap((rootRegion) => rootRegion.nodes.map((item) => item.id)))
        const fanoutLanes = rootRegions.map((rootRegion) => laneFor(rootRegion, branchMerge))
        const continuation = buildRegion(region.nodes.filter((item) => !rootNodeIDs.has(item.id)), merge, depth + 1)
        const items: WorkflowRuntimeTrackItem[] = [
          { kind: 'fanout', id: `${node.id}-${region.id}`, merge: branchMerge, lanes: fanoutLanes },
          ...continuation,
        ]
        return { id: region.id, label: region.label, items, nodes: workflowRuntimeBusinessNodes(items) }
      }
      const lanes = regions.map(laneWithFanout)
      regions.forEach((region) => region.nodes.forEach((laneNode) => consumed.add(laneNode.id)))
      if (branches.length === 1) track.push(...(lanes[0]?.items ?? []))
      else track.push({ kind: 'choice', condition: node, lanes })
    }

    const rendered = new Set(workflowRuntimeBusinessNodes(track).map((node) => node.id))
    for (const node of regionNodes) {
      if (!structuralNodeTypes.has(node.type) && !rendered.has(node.id)) track.push({ kind: 'node', node })
    }
    return track
  }

  return buildRegion(nodes)
}
