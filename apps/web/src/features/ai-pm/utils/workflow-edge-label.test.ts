import { describe, expect, it } from 'bun:test'
import { copy } from '@/shared/copy'
import { workflowEdgeDisplayLabel, workflowEdgeRouting } from '../utils/workflow-edge-label'

describe('workflowEdgeDisplayLabel', () => {
  it('uses business labels for known outcomes and never exposes raw conditions', () => {
    expect(workflowEdgeDisplayLabel({ kind: 'success' })).toBe(copy.aiPM.workflow.edgeSuccess)
    expect(workflowEdgeDisplayLabel({ kind: 'failure' })).toBe(copy.aiPM.workflow.edgeFailure)
    expect(workflowEdgeDisplayLabel({ kind: 'approve' })).toBe(copy.aiPM.workflow.edgeApprove)
    expect(workflowEdgeDisplayLabel({ kind: 'reject' })).toBe(copy.aiPM.workflow.edgeReject)
    expect(workflowEdgeDisplayLabel({ kind: 'yes' })).toBe(copy.aiPM.workflow.edgeYes)
    expect(workflowEdgeDisplayLabel({ kind: 'condition_no' })).toBe(copy.aiPM.workflow.edgeNo)
    expect(workflowEdgeDisplayLabel({})).toBeUndefined()
    expect(workflowEdgeDisplayLabel({ condition: 'context.decision == true' })).toBe(copy.aiPM.workflow.edgeYes)
    expect(workflowEdgeDisplayLabel({ condition: 'subject.needs_ui == false' })).toBe(copy.aiPM.workflow.edgeNo)
    expect(workflowEdgeDisplayLabel({ condition: 'subject.score > 80' })).toBe(copy.aiPM.workflow.edgeConditionPath)
    expect(workflowEdgeDisplayLabel({ condition: 'subject.secret_code' })).not.toContain('secret_code')
  })

  it('routes recovery outcomes separately from the normal business path', () => {
    const forward = workflowEdgeRouting({})
    const failure = workflowEdgeRouting({ kind: 'failure' })
    const reject = workflowEdgeRouting({ kind: 'reject' })
    const loop = workflowEdgeRouting({ loop: true })

    expect(forward).toEqual({ sourceHandle: 'forward-source', targetHandle: 'forward-target', type: 'smoothstep' })
    expect(failure).toEqual({ sourceHandle: 'recovery-source', targetHandle: 'recovery-target', type: 'bezier' })
    expect(reject).toEqual(failure)
    expect(loop).toEqual(forward)
  })
})
