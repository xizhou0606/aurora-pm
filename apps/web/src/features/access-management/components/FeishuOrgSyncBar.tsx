import { useState } from 'react'

import { useActionPermissions } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, StatusPill } from '@/shared/ui'

import { useFeishuOrgStatus, useSyncFeishuOrg } from '../hooks'

function shortDateTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// 飞书组织架构同步条：只维护展示数据，不参与授权；同步失败时目录继续
// 使用上一份成功数据。没有同步权限的用户完全看不到这条操作条。
export function FeishuOrgSyncBar() {
  const permissions = useActionPermissions()
  const canSync = permissions.canAction(PM_ACTION.AccessManagementGroupsSyncFeishuOrg)
  const status = useFeishuOrgStatus(canSync)
  const sync = useSyncFeishuOrg()
  const [feedback, setFeedback] = useState<string | null>(null)
  if (!canSync) return null

  const copy = COPY.subjects.feishuOrg
  const data = status.data
  return (
    <div
      data-testid="feishu-org-sync-bar"
      className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-border bg-card px-3 py-2 text-xs text-muted-foreground"
    >
      <StatusPill>{copy.cardLabel}</StatusPill>
      <span>
        {data?.hasSynced ? copy.lastSyncedAt(shortDateTime(data.syncedAt)) : copy.lastSyncedNever}
      </span>
      {data?.hasSynced ? (
        <span>{copy.summaryLabel(data.departments, data.mappedUsers, data.unmappedUsers)}</span>
      ) : null}
      {status.isError ? <span className="text-destructive">{copy.loadError}</span> : null}
      <span className="flex-1" />
      {feedback ? <span>{feedback}</span> : null}
      <Button
        type="button"
        size="xs"
        disabled={sync.isPending}
        onClick={() => {
          setFeedback(null)
          sync.mutate(undefined, {
            onSuccess: (result) => {
              setFeedback(copy.syncSuccess(
                result.stats.departments,
                result.stats.matchedUsers,
                result.stats.unmatchedContacts,
              ))
            },
            onError: () => setFeedback(copy.syncError),
          })
        }}
      >
        {sync.isPending ? copy.syncing : copy.sync}
      </Button>
    </div>
  )
}
