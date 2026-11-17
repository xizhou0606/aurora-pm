import type { SummaryState } from './types'

export function selectInboxMetrics(summary: SummaryState) {
  return summary.status === 'ready'
    ? {
        total: summary.data.total,
        workflowAttention: summary.data.workflowAIFailed,
        taskAttention: summary.data.blockedTasks,
        bugAttention: summary.data.reviewBugs,
      }
    : { total: 0, workflowAttention: 0, taskAttention: 0, bugAttention: 0 }
}
