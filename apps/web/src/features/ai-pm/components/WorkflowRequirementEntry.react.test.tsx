import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import { copy } from '@/shared/copy'
import { REQUIREMENT_DOCUMENTS_COPY as documentCopy } from '@/shared/copy/requirement-documents'
import type { Requirement } from '../model/types'
import { isRequirementInputWorkflowNode, WorkflowRequirementEntry } from './WorkflowRequirementEntry'

const originalAdapter = platformApi.defaults.adapter

const requirement: Requirement = {
  id: 42,
  workspace_id: 7,
  team_id: 1,
  project_new_id: 11,
  version_task_new_id: null,
  version: 3,
  title: '支付流程改造',
  description: '优化收银台交互',
  acceptance_criteria: '支付成功可核验',
  product_doc_url: '',
  priority: 'high',
  status: 'grooming',
  task_count: 0,
  progress: 0,
  allowed_actions: [],
  created_at: '2026-08-06T00:00:00Z',
  updated_at: '2026-08-07T00:00:00Z',
}

afterEach(() => {
  runtimeSessionStore.setState({ token: '' })
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 0 })
  platformApi.defaults.adapter = originalAdapter
})

describe('WorkflowRequirementEntry', () => {
  it('recognizes configured and legacy requirement-entry nodes only', () => {
    expect(isRequirementInputWorkflowNode({ type: 'form', name: '业务信息', config: { requirement_input: true } })).toBe(true)
    expect(isRequirementInputWorkflowNode({ type: 'human_task', name: '需求录入' })).toBe(true)
    expect(isRequirementInputWorkflowNode({ type: 'approval', name: '需求录入审批' })).toBe(false)
  })

  it('saves requirement fields and validates the Feishu document from the node', async () => {
    runtimeSessionStore.setState({ token: 'test-session' })
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    let patchBody: Record<string, unknown> | undefined
    let validateBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url?.includes('/document-resources')) {
        return { data: { data: { items: [], requirement_version: patchBody ? 4 : 3, project_set_id: 11, product_doc_url: patchBody?.product_doc_url ?? '' } }, status: 200, statusText: 'OK', headers: {}, config }
      }
      if (config.url === '/requirements/42' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        return { data: { data: { ...requirement, ...patchBody, version: 4 } }, status: 200, statusText: 'OK', headers: {}, config }
      }
      if (config.url === '/requirements/42/feishu-document/validate' && config.method === 'post') {
        validateBody = JSON.parse(String(config.data))
        return { data: { data: { valid: true, fetch_enabled: true } }, status: 200, statusText: 'OK', headers: {}, config }
      }
      return { data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowRequirementEntry workspaceId={7} requirement={requirement} /></QueryClientProvider>)

    const validateButton = await screen.findByRole('button', { name: copy.aiPM.requirementDetail.validate })
    fireEvent.change(screen.getByRole('textbox', { name: documentCopy.title }), { target: { value: 'https://example.feishu.cn/wiki/payment' } })
    fireEvent.click(validateButton)
    await waitFor(() => expect(validateBody).toEqual({ url: 'https://example.feishu.cn/wiki/payment' }))
    expect(await screen.findByText(copy.aiPM.requirementDetail.validEnabled)).toBeTruthy()

    fireEvent.click(await screen.findByRole('button', { name: documentCopy.save }))
    await waitFor(() => expect(patchBody).toMatchObject({
      expected_version: 3,
      product_doc_url: 'https://example.feishu.cn/wiki/payment',
    }))
    expect(patchBody).not.toHaveProperty('title')
    expect(patchBody).not.toHaveProperty('description')
    expect(patchBody).not.toHaveProperty('acceptance_criteria')
    expect(patchBody).not.toHaveProperty('status')
    await waitFor(() => expect((screen.getByRole('button', { name: documentCopy.save }) as HTMLButtonElement).disabled).toBe(true))
  })
})
