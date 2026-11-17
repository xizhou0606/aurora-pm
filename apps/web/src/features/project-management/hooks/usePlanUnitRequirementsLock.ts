import { useCallback, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { operationFeedback } from '@/shared/ui'
import { pmKeys } from '@/shared/query/keys'
import { useWorkspaceStore } from '@/shared/state'

import { lockPlanUnitRequirements } from '../api'

export type PlanUnitRequirementsLockTarget = { id: number; title: string }

// 纯锁版流：给已过需求录入阶段但仍未锁版的存量项目/版本直接落锁版事实，
// 工作流状态不动。结果统一走全局操作提示（右上角 toast）——空范围防呆等
// 失败原因由 operationErrorDescription 映射成具体文案；无论成败都关闭确认
// 弹窗，成功后失效 pm 工作区查询让列表与标签即时刷新。
export function usePlanUnitRequirementsLock() {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryClient = useQueryClient()
  const [target, setTarget] = useState<PlanUnitRequirementsLockTarget | null>(null)
  const mutation = useMutation({
    meta: operationFeedback({ success: '项目已锁版', failure: '锁版失败' }),
    mutationFn: (input: PlanUnitRequirementsLockTarget) => lockPlanUnitRequirements(input.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: pmKeys.workspace(workspaceId) })
    },
    onSettled: () => {
      setTarget(null)
    },
  })

  const open = useCallback((row: PlanUnitRequirementsLockTarget) => {
    mutation.reset()
    setTarget(row)
  }, [mutation])

  const close = useCallback(() => {
    mutation.reset()
    setTarget(null)
  }, [mutation])

  const confirm = useCallback(() => {
    if (!target) return
    mutation.mutate(target)
  }, [mutation, target])

  return { target, open, close, confirm, pending: mutation.isPending }
}
