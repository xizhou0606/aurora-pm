import type { WorkflowInstance } from '../model/types'

export type WorkflowRuntimeNode = WorkflowInstance['timeline_nodes'][number]
export type WorkflowRuntimeEdge = WorkflowInstance['definition_edges'][number]
export type WorkflowRuntimeBranch = { id: string; label?: string; roots: string[] }
export type WorkflowRuntimeRegion = WorkflowRuntimeBranch & { nodes: WorkflowRuntimeNode[] }

function isRecoveryEdge(edge: WorkflowRuntimeEdge) {
  return edge.kind === 'failure' || edge.kind === 'reject'
}

export function runtimeDisplayEdges(instance: WorkflowInstance, nodes: WorkflowRuntimeNode[]) {
  const nodeIDs = new Set(nodes.map((node) => node.id))
  const selectedEdgeIDs = Array.isArray(instance.selected_edge_ids) ? new Set(instance.selected_edge_ids) : undefined
  return (instance.definition_edges ?? []).filter((edge) => nodeIDs.has(edge.source) && nodeIDs.has(edge.target) && (
    !selectedEdgeIDs || selectedEdgeIDs.has(edge.id)
  ))
}

export function runtimeTopologyEdges(instance: WorkflowInstance, nodes: WorkflowRuntimeNode[]) {
  const nodeIDs = new Set(nodes.map((node) => node.id))
  return (instance.definition_edges ?? []).filter((edge) => nodeIDs.has(edge.source) && nodeIDs.has(edge.target))
}

export function orderRuntimeNodes(instance: WorkflowInstance, visibleTimeline: WorkflowRuntimeNode[], edges: WorkflowRuntimeEdge[]) {
  const starts = visibleTimeline.filter((node) => node.type === 'start')
  const authoritative = Array.isArray(instance.selected_node_ids) && Array.isArray(instance.selected_edge_ids)
  if (!authoritative || !edges.length || !starts.length) return visibleTimeline

  const nodeByID = new Map(visibleTimeline.map((node) => [node.id, node]))
  const outgoing = new Map<string, WorkflowRuntimeEdge[]>()
  for (const edge of edges) outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge])

  const reachable = new Set<string>()
  const discoveryOrder: string[] = []
  const queue = starts.map((node) => node.id)
  while (queue.length) {
    const nodeID = queue.shift()!
    if (reachable.has(nodeID)) continue
    reachable.add(nodeID)
    discoveryOrder.push(nodeID)
    queue.push(...(outgoing.get(nodeID) ?? []).map((edge) => edge.target))
  }

  const indegree = new Map([...reachable].map((nodeID) => [nodeID, 0]))
  for (const edge of edges) {
    if (reachable.has(edge.source) && reachable.has(edge.target)) {
      indegree.set(edge.target, (indegree.get(edge.target) ?? 0) + 1)
    }
  }
  const ready = starts.filter((node) => reachable.has(node.id) && indegree.get(node.id) === 0).map((node) => node.id)
  const orderedIDs: string[] = []
  const emitted = new Set<string>()
  while (ready.length) {
    const nodeID = ready.shift()!
    if (emitted.has(nodeID)) continue
    emitted.add(nodeID)
    orderedIDs.push(nodeID)
    for (const edge of outgoing.get(nodeID) ?? []) {
      if (!reachable.has(edge.target)) continue
      const next = (indegree.get(edge.target) ?? 0) - 1
      indegree.set(edge.target, next)
      if (next === 0) ready.push(edge.target)
    }
  }
  if (emitted.size < reachable.size) orderedIDs.splice(0, orderedIDs.length, ...discoveryOrder)

  const result: WorkflowRuntimeNode[] = []
  const included = new Set<string>()
  for (const nodeID of orderedIDs) {
    const node = nodeByID.get(nodeID)
    if (!node || included.has(node.id)) continue
    included.add(node.id)
    result.push(node)
  }
  for (const node of visibleTimeline) {
    if (included.has(node.id)) continue
    included.add(node.id)
    result.push(node)
  }
  return result
}

export function runtimeGraphHelpers(nodes: WorkflowRuntimeNode[], edges: WorkflowRuntimeEdge[]) {
  const order = new Map(nodes.map((node, index) => [node.id, index]))
  const baseOutgoing = new Map<string, WorkflowRuntimeEdge[]>()
  for (const edge of edges.filter((candidate) => !isRecoveryEdge(candidate))) {
    baseOutgoing.set(edge.source, [...(baseOutgoing.get(edge.source) ?? []), edge])
  }
  const reachesThrough = (outgoing: Map<string, WorkflowRuntimeEdge[]>, from: string, target: string) => {
    const visited = new Set<string>()
    const queue = [from]
    while (queue.length) {
      const nodeID = queue.shift()!
      if (nodeID === target) return true
      if (visited.has(nodeID)) continue
      visited.add(nodeID)
      queue.push(...(outgoing.get(nodeID) ?? []).map((edge) => edge.target))
    }
    return false
  }
  const outgoing = new Map<string, WorkflowRuntimeEdge[]>()
  for (const edge of edges.filter((candidate) => (
    !isRecoveryEdge(candidate) || !reachesThrough(baseOutgoing, candidate.target, candidate.source)
  ))) {
    outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge])
  }
  const canReach = (from: string, target: string) => reachesThrough(outgoing, from, target)
  const distances = (root: string) => {
    const result = new Map<string, number>([[root, 0]])
    const queue = [root]
    while (queue.length) {
      const nodeID = queue.shift()!
      const distance = result.get(nodeID) ?? 0
      for (const edge of outgoing.get(nodeID) ?? []) {
        if (result.has(edge.target)) continue
        result.set(edge.target, distance + 1)
        queue.push(edge.target)
      }
    }
    return result
  }
  const nearestCommon = (roots: string[], candidates: WorkflowRuntimeNode[]) => {
    const rootDistances = roots.map(distances)
    return candidates
      .filter((candidate) => rootDistances.every((distance) => distance.has(candidate.id)))
      .sort((left, right) => {
        const leftDistances = rootDistances.map((distance) => distance.get(left.id)!)
        const rightDistances = rootDistances.map((distance) => distance.get(right.id)!)
        return Math.max(...leftDistances) - Math.max(...rightDistances)
          || leftDistances.reduce((sum, value) => sum + value, 0) - rightDistances.reduce((sum, value) => sum + value, 0)
          || (order.get(left.id) ?? Number.MAX_SAFE_INTEGER) - (order.get(right.id) ?? Number.MAX_SAFE_INTEGER)
      })[0]
  }
  return { canReach, nearestCommon }
}

export function runtimeForkRegions(
  branches: WorkflowRuntimeBranch[],
  merge: WorkflowRuntimeNode,
  regionNodes: WorkflowRuntimeNode[],
  canReach: (from: string, target: string) => boolean,
  forkID: string,
  selectedEdges: WorkflowRuntimeEdge[],
) {
  const used = new Set<string>()
  const regions: WorkflowRuntimeRegion[] = []
  for (const branch of branches) {
    const nodes = branch.roots.every((root) => root === merge.id) ? [] : regionNodes.filter((node) => (
      node.id !== forkID
      && node.id !== merge.id
      && branch.roots.some((root) => root !== merge.id && canReach(root, node.id))
      && canReach(node.id, merge.id)
    ))
    if (nodes.some((node) => used.has(node.id))) return undefined
    nodes.forEach((node) => used.add(node.id))
    regions.push({ ...branch, nodes })
  }

  const allowed = new Set(regionNodes.map((node) => node.id).filter((id) => id !== forkID && id !== merge.id))
  const adjacent = new Map<string, string[]>()
  for (const edge of selectedEdges) {
    if (!allowed.has(edge.source) || !allowed.has(edge.target)) continue
    adjacent.set(edge.source, [...(adjacent.get(edge.source) ?? []), edge.target])
    adjacent.set(edge.target, [...(adjacent.get(edge.target) ?? []), edge.source])
  }
  const connected = (seeds: string[], target: string) => {
    const visited = new Set<string>()
    const queue = seeds.filter((seed) => allowed.has(seed))
    while (queue.length) {
      const nodeID = queue.shift()!
      if (nodeID === target) return true
      if (visited.has(nodeID)) continue
      visited.add(nodeID)
      queue.push(...(adjacent.get(nodeID) ?? []))
    }
    return false
  }
  for (const node of regionNodes) {
    if (!allowed.has(node.id) || used.has(node.id)) continue
    const matches = regions.filter((region) => connected([...region.roots, ...region.nodes.map((item) => item.id)], node.id))
    if (matches.length > 1) return undefined
    if (matches.length === 1) {
      matches[0]!.nodes.push(node)
      used.add(node.id)
    }
  }
  return regions
}
