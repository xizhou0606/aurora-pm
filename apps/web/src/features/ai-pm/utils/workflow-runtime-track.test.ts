import { describe, expect, it } from 'bun:test'
import type { WorkflowInstance } from '../model/types'
import { workflowEdgeDisplayLabel } from '../utils/workflow-edge-label'
import { workflowRuntimeTrack, type WorkflowRuntimeNode, type WorkflowRuntimeTrackItem } from '../utils/workflow-runtime-track'

type Edge = WorkflowInstance['definition_edges'][number]

function node(id: string, type: string, status: WorkflowRuntimeNode['status'] = 'pending'): WorkflowRuntimeNode {
  return { id, type, name: id, status }
}

function instance(nodes: WorkflowRuntimeNode[], edges: Edge[], selectedEdges = edges.map((edge) => edge.id)): WorkflowInstance {
  return {
    id: 1, template_id: 1, template_name: 'test', template_version: 1, status: 'running', subject_type: 'plan_unit', subject_id: 1,
    current_node_ids: [], current_nodes: [], timeline_nodes: nodes, definition_edges: edges,
    selected_node_ids: nodes.map((item) => item.id), selected_edge_ids: selectedEdges,
    node_runs: [], approvals: [], completed_steps: 0, total_steps: 0, progress: 0, started_at: '2026-08-10T00:00:00Z',
  }
}

function businessIDs(track: WorkflowRuntimeTrackItem[]) {
  return track.flatMap((item) => item.kind === 'node' ? [item.node.id] : item.lanes.flatMap((lane) => lane.nodes.map((item) => item.id)))
}

describe('workflow runtime compact track', () => {
  it('keeps the real project development and risk paths in separate lanes', () => {
    const nodes = [
      node('acceptance', 'human_task'), node('handle', 'human_task'), node('start', 'start', 'completed'),
      node('review', 'approval'), node('join', 'parallel_join', 'waiting'), node('risk', 'ai_check'),
      node('input', 'form', 'active'), node('develop', 'human_task'), node('split', 'parallel_split'), node('end', 'end'),
    ]
    const edges: Edge[] = [
      { id: 'start-input', source: 'start', target: 'input' }, { id: 'input-review', source: 'input', target: 'review' },
      { id: 'review-split', source: 'review', target: 'split' }, { id: 'split-develop', source: 'split', target: 'develop', loop: true },
      { id: 'split-risk', source: 'split', target: 'risk', loop: true }, { id: 'risk-handle', source: 'risk', target: 'handle', loop: true },
      { id: 'risk-failure', source: 'risk', target: 'handle', kind: 'failure', loop: true },
      { id: 'develop-join', source: 'develop', target: 'join', loop: true }, { id: 'handle-join', source: 'handle', target: 'join', loop: true },
      { id: 'join-acceptance', source: 'join', target: 'acceptance', loop: true }, { id: 'acceptance-end', source: 'acceptance', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges, edges.filter((edge) => edge.id !== 'risk-failure').map((edge) => edge.id)), nodes)
    const parallel = track.find((item) => item.kind === 'parallel')
    expect(parallel?.kind === 'parallel' ? parallel.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['develop'], ['risk', 'handle'],
    ])
    expect(businessIDs(track)).toEqual(['input', 'review', 'develop', 'risk', 'handle', 'acceptance'])
  })

  it('shows unresolved alternatives before a shared parallel group and removes them after no is selected', () => {
    const nodes = [node('start', 'start', 'completed'), node('choice', 'condition', 'active'), node('design', 'human_task'), node('split', 'parallel_split'), node('dev', 'human_task'), node('qa', 'human_task'), node('join', 'parallel_join'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-choice', source: 'start', target: 'choice' }, { id: 'yes', source: 'choice', target: 'design', condition: 'context.decision == true' },
      { id: 'no', source: 'choice', target: 'split', condition: 'context.decision == false' }, { id: 'design-split', source: 'design', target: 'split' },
      { id: 'split-dev', source: 'split', target: 'dev' }, { id: 'split-qa', source: 'split', target: 'qa' },
      { id: 'dev-join', source: 'dev', target: 'join' }, { id: 'qa-join', source: 'qa', target: 'join' }, { id: 'join-end', source: 'join', target: 'end' },
    ]
    const unresolved = workflowRuntimeTrack(instance(nodes, edges), nodes)
    expect(unresolved.map((item) => item.kind)).toEqual(['node', 'choice', 'parallel'])
    expect(unresolved.find((item) => item.kind === 'choice')?.kind === 'choice'
      ? unresolved.find((item) => item.kind === 'choice')!.lanes.map((lane) => lane.nodes.map((item) => item.id))
      : []).toEqual([['design'], []])

    const selectedNodes = nodes.filter((item) => item.id !== 'design')
    const selectedEdges = edges.filter((edge) => !['yes', 'design-split'].includes(edge.id))
    const decided = workflowRuntimeTrack(instance(selectedNodes, selectedEdges), selectedNodes)
    expect(decided.map((item) => item.kind)).toEqual(['node', 'parallel'])
    expect(businessIDs(decided)).toEqual(['choice', 'dev', 'qa'])
  })

  it('keeps a selected forward failure path inside its parallel lane', () => {
    const nodes = [node('start', 'start'), node('split', 'parallel_split'), node('delivery', 'human_task'), node('risk', 'ai_check'), node('repair', 'human_task', 'active'), node('join', 'parallel_join'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-split', source: 'start', target: 'split' }, { id: 'split-delivery', source: 'split', target: 'delivery' }, { id: 'split-risk', source: 'split', target: 'risk' },
      { id: 'delivery-join', source: 'delivery', target: 'join' }, { id: 'risk-repair', source: 'risk', target: 'repair', kind: 'failure' },
      { id: 'repair-join', source: 'repair', target: 'join' }, { id: 'join-end', source: 'join', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges), nodes)
    const parallel = track.find((item) => item.kind === 'parallel')
    expect(parallel?.kind === 'parallel' ? parallel.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['delivery'], ['risk', 'repair'],
    ])
    expect(businessIDs(track)).toEqual(['delivery', 'risk', 'repair'])
    expect(new Set(businessIDs(track)).size).toBe(3)
  })

  it('keeps an unresolved choice nested inside its parallel lane', () => {
    const nodes = [node('start', 'start'), node('split', 'parallel_split'), node('plain', 'human_task'), node('choice', 'condition'), node('a', 'human_task'), node('b', 'human_task'), node('join', 'parallel_join'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-split', source: 'start', target: 'split' }, { id: 'split-plain', source: 'split', target: 'plain' }, { id: 'split-choice', source: 'split', target: 'choice' },
      { id: 'choice-a', source: 'choice', target: 'a' }, { id: 'choice-b', source: 'choice', target: 'b' },
      { id: 'plain-join', source: 'plain', target: 'join' }, { id: 'a-join', source: 'a', target: 'join' }, { id: 'b-join', source: 'b', target: 'join' }, { id: 'join-end', source: 'join', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges), nodes)
    const parallel = track.find((item) => item.kind === 'parallel')
    expect(parallel?.kind).toBe('parallel')
    expect(parallel?.kind === 'parallel' ? parallel.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['plain'], ['choice', 'a', 'b'],
    ])
    const nestedChoice = parallel?.kind === 'parallel'
      ? parallel.lanes[1]?.items.find((item) => item.kind === 'choice')
      : undefined
    expect(nestedChoice?.kind === 'choice' ? nestedChoice.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['a'], ['b'],
    ])
    expect(businessIDs(track)).toEqual(['plain', 'choice', 'a', 'b'])
    expect(new Set(businessIDs(track)).size).toBe(4)
  })

  it('uses the hidden end node as the merge for a direct condition path', () => {
    const nodes = [node('start', 'start'), node('choice', 'condition'), node('risk', 'human_task'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-choice', source: 'start', target: 'choice' },
      { id: 'yes', source: 'choice', target: 'risk', condition: 'context.decision == true' },
      { id: 'no', source: 'choice', target: 'end', condition: 'context.decision == false' },
      { id: 'risk-end', source: 'risk', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges), nodes)
    const choice = track.find((item) => item.kind === 'choice')
    expect(choice?.kind === 'choice' ? choice.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['risk'], [],
    ])
    expect(businessIDs(track)).toEqual(['choice', 'risk'])
  })

  it('groups multiple targets by condition outcome before a shared downstream node', () => {
    const nodes = [node('start', 'start'), node('choice', 'condition'), node('split', 'parallel_split'), node('develop', 'human_task'), node('qa', 'approval'), node('join', 'parallel_join'), node('delivery', 'human_task'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-choice', source: 'start', target: 'choice' },
      { id: 'yes-split', source: 'choice', target: 'split', kind: 'yes' },
      { id: 'yes-delivery', source: 'choice', target: 'delivery', kind: 'yes' },
      { id: 'no-delivery', source: 'choice', target: 'delivery', kind: 'no' },
      { id: 'split-develop', source: 'split', target: 'develop' },
      { id: 'split-qa', source: 'split', target: 'qa' },
      { id: 'develop-join', source: 'develop', target: 'join' },
      { id: 'qa-join', source: 'qa', target: 'join' },
      { id: 'join-delivery', source: 'join', target: 'delivery' },
      { id: 'delivery-end', source: 'delivery', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges), nodes)
    const choice = track.find((item) => item.kind === 'choice')
    expect(choice?.kind === 'choice' ? choice.lanes.map((lane) => lane.label) : []).toEqual([
      workflowEdgeDisplayLabel({ kind: 'yes' }), workflowEdgeDisplayLabel({ kind: 'no' }),
    ])
    expect(choice?.kind === 'choice' ? choice.lanes[0]?.items.map((item) => item.kind) : []).toEqual(['fanout'])
    const fanout = choice?.kind === 'choice' && choice.lanes[0]?.items[0]?.kind === 'fanout'
      ? choice.lanes[0].items[0]
      : undefined
    expect(fanout?.lanes.map((lane) => lane.items.map((item) => item.kind))).toEqual([['parallel'], []])
    expect(fanout?.lanes[1]?.directTarget?.id).toBe('delivery')
    expect(choice?.kind === 'choice' ? choice.lanes[1]?.items : []).toEqual([])
    expect(businessIDs(track)).toEqual(['choice', 'develop', 'qa', 'delivery'])
    expect(new Set(businessIDs(track)).size).toBe(4)
  })

  it('keeps a selected reject loop inside its original parallel lane', () => {
    const nodes = [node('start', 'start'), node('split', 'parallel_split'), node('review', 'approval'), node('revise', 'human_task', 'active'), node('work', 'human_task'), node('join', 'parallel_join'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-split', source: 'start', target: 'split' },
      { id: 'split-review', source: 'split', target: 'review' },
      { id: 'split-work', source: 'split', target: 'work' },
      { id: 'review-join', source: 'review', target: 'join', kind: 'approve' },
      { id: 'review-revise', source: 'review', target: 'revise', kind: 'reject' },
      { id: 'revise-review', source: 'revise', target: 'review' },
      { id: 'work-join', source: 'work', target: 'join' },
      { id: 'join-end', source: 'join', target: 'end' },
    ]
    const selectedEdges = edges.filter((edge) => edge.id !== 'review-join').map((edge) => edge.id)
    const track = workflowRuntimeTrack(instance(nodes, edges, selectedEdges), nodes)
    const parallel = track.find((item) => item.kind === 'parallel')
    expect(parallel?.kind === 'parallel' ? parallel.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['review', 'revise'], ['work'],
    ])
    expect(businessIDs(track)).toEqual(['review', 'revise', 'work'])
    expect(new Set(businessIDs(track)).size).toBe(3)
  })

  it('keeps concurrent targets for one condition outcome in separate lanes', () => {
    const nodes = [node('start', 'start'), node('choice', 'condition'), node('design', 'human_task'), node('develop', 'human_task'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-choice', source: 'start', target: 'choice' },
      { id: 'yes-design', source: 'choice', target: 'design', kind: 'yes' },
      { id: 'yes-develop', source: 'choice', target: 'develop', kind: 'yes' },
      { id: 'no-end', source: 'choice', target: 'end', kind: 'no' },
      { id: 'design-end', source: 'design', target: 'end' },
      { id: 'develop-end', source: 'develop', target: 'end' },
    ]
    const track = workflowRuntimeTrack(instance(nodes, edges), nodes)
    const choice = track.find((item) => item.kind === 'choice')
    const fanout = choice?.kind === 'choice' && choice.lanes[0]?.items[0]?.kind === 'fanout'
      ? choice.lanes[0].items[0]
      : undefined
    expect(fanout?.lanes.map((lane) => lane.nodes.map((item) => item.id))).toEqual([['design'], ['develop']])
    expect(choice?.kind === 'choice' ? choice.lanes[1]?.directTarget?.id : undefined).toBe('end')
    expect(businessIDs(track)).toEqual(['choice', 'design', 'develop'])

    const decidedEdges = edges.filter((edge) => edge.id !== 'no-end')
    const decided = workflowRuntimeTrack(instance(nodes, decidedEdges), nodes)
    const decidedFanout = decided.find((item) => item.kind === 'fanout')
    expect(decided.some((item) => item.kind === 'choice')).toBe(false)
    expect(decidedFanout?.kind === 'fanout' ? decidedFanout.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['design'], ['develop'],
    ])
    expect(businessIDs(decided)).toEqual(['choice', 'design', 'develop'])
  })

  it('uses hidden normal-path nodes to keep a reject loop in its parallel lane', () => {
    const nodes = [node('start', 'start'), node('split', 'parallel_split'), node('review', 'approval'), node('post-review', 'human_task'), node('revise', 'human_task', 'active'), node('work', 'human_task'), node('join', 'parallel_join'), node('end', 'end')]
    const edges: Edge[] = [
      { id: 'start-split', source: 'start', target: 'split' },
      { id: 'split-review', source: 'split', target: 'review' },
      { id: 'split-work', source: 'split', target: 'work' },
      { id: 'review-post', source: 'review', target: 'post-review', kind: 'approve' },
      { id: 'post-join', source: 'post-review', target: 'join' },
      { id: 'review-revise', source: 'review', target: 'revise', kind: 'reject' },
      { id: 'revise-review', source: 'revise', target: 'review' },
      { id: 'work-join', source: 'work', target: 'join' },
      { id: 'join-end', source: 'join', target: 'end' },
    ]
    const selectedEdges = edges.filter((edge) => !['review-post', 'post-join'].includes(edge.id)).map((edge) => edge.id)
    const workflow = instance(nodes, edges, selectedEdges)
    workflow.selected_node_ids = nodes.filter((item) => item.id !== 'post-review').map((item) => item.id)
    const visibleNodes = nodes.filter((item) => item.id !== 'post-review')
    const track = workflowRuntimeTrack(workflow, visibleNodes)
    const parallel = track.find((item) => item.kind === 'parallel')
    expect(parallel?.kind === 'parallel' ? parallel.lanes.map((lane) => lane.nodes.map((item) => item.id)) : []).toEqual([
      ['review', 'revise'], ['work'],
    ])
    expect(businessIDs(track)).toEqual(['review', 'revise', 'work'])
  })
})
