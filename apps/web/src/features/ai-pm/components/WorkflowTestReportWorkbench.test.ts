import { describe, expect, it } from 'bun:test'

import { isTestReportWorkflowNode } from './WorkflowTestReportWorkbench'
import { isHttpURL, isWorkflowArtifactLink } from '../utils/workflow-artifact-file'

describe('workflow test report helpers', () => {
  it('recognizes configured and named test-acceptance nodes only', () => {
    expect(isTestReportWorkflowNode({ type: 'human_task', name: '测试验收' })).toBe(true)
    expect(isTestReportWorkflowNode({ type: 'form', name: 'QA 交付', config: { test_report_artifact: true } })).toBe(true)
    expect(isTestReportWorkflowNode({ type: 'human_task', name: '测试用例编写' })).toBe(false)
    expect(isTestReportWorkflowNode({ type: 'approval', name: '测试验收' })).toBe(false)
  })

  it('accepts http(s) report links and rejects other schemes', () => {
    expect(isHttpURL('https://wiki.example.com/qa/report')).toBe(true)
    expect(isHttpURL('http://intranet.local/report')).toBe(true)
    expect(isHttpURL('javascript:alert(1)')).toBe(false)
    expect(isHttpURL('ftp://files.example.com/report.pdf')).toBe(false)
    expect(isWorkflowArtifactLink({
      id: 1, name: 'wiki.example.com/qa/report', source: 'link', url: 'https://wiki.example.com/qa/report', content_type: 'text/uri-list',
    })).toBe(true)
    expect(isWorkflowArtifactLink({
      id: 2, name: 'report.pdf', source: 'file', url: '/api/v1/platform/pm/workflow-node-artifacts/2/download', content_type: 'application/pdf',
    })).toBe(false)
  })
})
