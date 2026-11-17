export const WORK_ITEM_ENUM_COPY = Object.freeze({
  statuses: {
    draft: '待处理',
    in_dev: '进行中',
    pending_test: '待验证',
    completed: '已完成',
    cancelled: '已取消',
  } as const satisfies Readonly<Record<string, string>>,
  priorities: {
    low: '低',
    normal: '普通',
    high: '高',
    urgent: '紧急',
  } as Readonly<Record<string, string>>,
  kinds: {
    requirement: '需求',
    epic: '需求',
    story: '任务',
    task: '任务',
    subtask: '子任务',
    bug: '缺陷',
    test: '测试',
  } as Readonly<Record<string, string>>,
  fallback: '—',
} as const)
