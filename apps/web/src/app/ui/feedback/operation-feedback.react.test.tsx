import { QueryClientProvider, useMutation } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { createPMQueryClient } from '@/app/query'
import { PMSafeApiError } from '@/shared/api'
import {
  operationErrorDescription,
  operationFeedback,
  OperationFeedbackProvider,
} from '@/shared/ui/feedback/operation-feedback'

function MutationProbe() {
  const success = useMutation({
    meta: operationFeedback('接口调用成功', '接口调用失败'),
    mutationFn: async () => ({ ok: true }),
  })
  const failure = useMutation({
    meta: operationFeedback('不会显示', '需求保存失败'),
    mutationFn: async () => {
      throw new PMSafeApiError('REQUIREMENT_UPDATE_FAILED', 403)
    },
  })
  return (
    <>
      <button type="button" onClick={() => success.mutate()}>success</button>
      <button type="button" onClick={() => failure.mutate()}>failure</button>
    </>
  )
}

describe('operation feedback', () => {
  it('renders API mutation success and failure through the shared lifecycle', async () => {
    const client = createPMQueryClient()
    render(
      <QueryClientProvider client={client}>
        <OperationFeedbackProvider>
          <MutationProbe />
        </OperationFeedbackProvider>
      </QueryClientProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'success' }))
    const successToast = await screen.findByText('接口调用成功')
    expect(successToast.closest('[data-operation-feedback="success"]')?.getAttribute('role')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'failure' }))
    expect((await screen.findAllByText('需求保存失败')).length).toBeGreaterThan(0)
    expect((await screen.findByRole('alert')).getAttribute('data-operation-feedback')).toBe('error')
    expect(screen.getAllByText('当前账号没有执行此操作的权限。').length).toBeGreaterThan(0)
  })

  it('normalizes safe API errors without exposing raw server details', () => {
    expect(operationErrorDescription(new PMSafeApiError('CONFLICT', 409)))
      .toBe('数据已发生变化，请刷新后重试。')
    expect(operationErrorDescription(new PMSafeApiError('WORKFLOW_NEXT_ASSIGNEE_REQUIRED', 409)))
      .toBe('尚未指定下一节点负责人。')
    expect(operationErrorDescription(new PMSafeApiError('QUALITY_REGRESSION_NOTE_TOO_LONG', 422)))
      .toBe('重新打开原因不能超过 5000 个字符。')
    expect(operationErrorDescription(new PMSafeApiError('NETWORK_UNAVAILABLE')))
      .toBe('网络连接异常，请检查网络后重试。')
    expect(operationErrorDescription(new Error('private server detail')))
      .toBe('请求未完成，请稍后重试。')
  })
})
