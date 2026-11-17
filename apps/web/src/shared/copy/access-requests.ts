export const ACCESS_REQUESTS_COPY = {
  request: '申请编辑权限', title: '申请文档编辑权限', review: '权限申请', owner: '文档所属者',
  denied: '你还没有此文档的访问权限', deniedHint: '可向文档所属者申请编辑权限，通过后即可访问。',
  waiting: '已提交申请，等待文档所属者审批', waitingHint: '申请有效期为 3 天，处理前无需重复申请。',
  expiredHint: '申请已超过 3 天有效期，可以重新申请。', rejectedHint: '申请未通过，可以补充理由后重新申请。',
  validity: '申请有效期为 3 天。', unavailable: '暂时无法查询文档访问状态',
  more: '优先显示待处理申请，当前最多显示 100 条。', noRequestPermission: '当前账号暂不能申请编辑权限，请联系文档所属者或管理员。',
  reason: '申请理由', placeholder: '向文档所属者说明申请编辑权限的原因', notify: '通知文档所属者',
  grantLevel: '授权权限', grantDuration: '授权有效期', grantUntil: '授权到期时间', ownerMissing: '此资源尚未设置所属者，请联系管理员。',
  submit: '提交申请', submitting: '正在提交', pending: '已申请，等待审批',
  unread: '未读', read: '已读', markRead: '标记已读', markUnread: '标记未读',
  approve: '通过', reject: '拒绝', revoke: '撤销授权', empty: '没有权限申请', loading: '加载权限申请',
  error: '权限申请操作失败，请重试', retry: '重试', close: '关闭',
  statuses: { pending: '待处理', approved: '已通过', rejected: '已拒绝', revoked: '授权已撤销', expired: '已过期' },
  pendingCount: (count: number) => `权限申请 · ${count}`,
  separator: ' · ',
} as const

export function accessRequestsCopy(resourceType: 'rich_text' | 'bitable' = 'rich_text') {
  if (resourceType === 'rich_text') return ACCESS_REQUESTS_COPY
  return { ...ACCESS_REQUESTS_COPY, title: '申请多维表格编辑权限', owner: '多维表格所属者',
    denied: '你还没有此多维表格的访问权限', deniedHint: '可向多维表格所属者申请编辑权限，通过后即可访问。',
    waiting: '已提交申请，等待多维表格所属者审批', unavailable: '暂时无法查询多维表格访问状态',
    noRequestPermission: '当前账号暂不能申请编辑权限，请联系多维表格所属者或管理员。',
    placeholder: '向多维表格所属者说明申请编辑权限的原因', notify: '通知多维表格所属者',
  }
}
