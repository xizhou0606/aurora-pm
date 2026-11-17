import { useMemo, useState } from 'react'

import {
  Button,
  Checkbox,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmConnectionDTO, ScmRepositoryDTO, ScmRepositoryInput } from '../api/contracts'
import {
  useAllAccessibleReposQuery,
  useApplyRepoSelectionMutation,
  type RepoSelectionChange,
} from '../hooks'

type RepositoryScanPickerProps = {
  open: boolean
  connections: ScmConnectionDTO[]
  repositories: ScmRepositoryDTO[]
  onOpenChange: (open: boolean) => void
  onError: (message: string) => void
}

function repoKey(path: string, externalId?: string): string {
  return externalId || path
}

function repoUpdateBody(repo: ScmRepositoryDTO, enabled: boolean): ScmRepositoryInput {
  return {
    connection_id: repo.connection_id,
    external_id: repo.external_id,
    path: repo.path,
    default_branch: repo.default_branch,
    filter_rules: repo.filter_rules,
    project_set_id: repo.project_set_id ?? null,
    enabled,
  }
}

// 令牌扫描出的仓库全量多选：勾选即采集该仓库代码用于汇报评分，取消勾选
// 停用采集（配置保留，可随时再勾回来）。勾选状态 = 用户覆盖值 ?? 当前已
// 接入基线，应用时按差异生成新建与启停变更顺序提交，任何一步失败即中断。
// 父组件用 open 重挂载本组件，覆盖值随之清零。
export function RepositoryScanPicker({ open, connections, repositories, onOpenChange, onError }: RepositoryScanPickerProps) {
  const [connectionId, setConnectionId] = useState(() => connections[0]?.id ?? 0)
  const scanQuery = useAllAccessibleReposQuery(connectionId, open && connectionId > 0)
  const applyMutation = useApplyRepoSelectionMutation(onError)
  const [overrides, setOverrides] = useState<Map<string, boolean>>(new Map())

  const scannedRepos = scanQuery.data?.repos ?? []
  const connectionItems = connections.map((connection) => ({ value: String(connection.id), label: connection.name }))
  const boundByConnection = useMemo(() => {
    const map = new Map<string, ScmRepositoryDTO>()
    for (const repo of repositories) {
      if (repo.connection_id === connectionId) map.set(repoKey(repo.path, repo.external_id), repo)
    }
    return map
  }, [connectionId, repositories])

  const isChecked = (key: string) =>
    overrides.get(key) ?? boundByConnection.get(key)?.enabled ?? false

  const toggle = (key: string, next: boolean) => {
    setOverrides((prev) => new Map(prev).set(key, next))
  }

  const changes = useMemo<RepoSelectionChange[]>(() => {
    const list: RepoSelectionChange[] = []
    for (const repo of scannedRepos) {
      const key = repoKey(repo.path, repo.external_id)
      const willCollect = isChecked(key)
      const bound = boundByConnection.get(key)
      if (willCollect && bound?.enabled !== true) {
        list.push(bound
          ? { kind: 'setEnabled', id: bound.id, body: repoUpdateBody(bound, true) }
          : {
              kind: 'create',
              body: {
                connection_id: connectionId,
                external_id: repo.external_id,
                path: repo.path,
                default_branch: repo.default_branch,
                enabled: true,
              },
            })
      } else if (!willCollect && bound?.enabled === true) {
        list.push({ kind: 'setEnabled', id: bound.id, body: repoUpdateBody(bound, false) })
      }
    }
    return list
    // isChecked 闭包依赖 overrides 与基线，二者都在依赖列表里。
  }, [scannedRepos, boundByConnection, overrides, connectionId])

  const selectedCount = scannedRepos.reduce((count, repo) => (
    count + (isChecked(repoKey(repo.path, repo.external_id)) ? 1 : 0)
  ), 0)

  const apply = () => {
    applyMutation.mutate(changes, { onSuccess: () => onOpenChange(false) })
  }

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      title={COPY.repositories.scanTitle}
      closeLabel={COPY.common.close}
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {COPY.common.cancel}
          </Button>
          <Button onClick={apply} disabled={applyMutation.isPending || changes.length === 0}>
            {COPY.repositories.scanApply(changes.length)}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <p className="text-xs text-muted-foreground">{COPY.repositories.scanDescription}</p>
        <label className="grid gap-1.5">
          <Label htmlFor="scm-scan-connection">{COPY.repositories.fields.connection}</Label>
          <Select
            items={connectionItems}
            value={String(connectionId)}
            onValueChange={(value) => setConnectionId(Number(value))}
          >
            <SelectTrigger id="scm-scan-connection" name="scm-scan-connection" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {connectionItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        {scanQuery.isPending ? (
          <p className="text-sm text-muted-foreground">{COPY.common.loading}</p>
        ) : scanQuery.isError ? (
          <div className="grid gap-1">
            <p className="text-sm text-muted-foreground">{COPY.errors.loadFailed}</p>
            <Button variant="outline" size="sm" onClick={() => void scanQuery.refetch()}>
              {COPY.common.retry}
            </Button>
          </div>
        ) : (
          <>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">
                {COPY.repositories.scanSelectedCount(selectedCount)}
              </span>
              <span className="text-xs text-muted-foreground">
                {COPY.repositories.scanTotal(scannedRepos.length)}
              </span>
            </div>
            {scanQuery.data?.truncated ? (
              <p className="text-xs text-muted-foreground">{COPY.repositories.scanTruncated}</p>
            ) : null}
            <ul className="max-h-96 divide-y overflow-auto rounded-lg border">
              {scannedRepos.map((repo) => {
                const key = repoKey(repo.path, repo.external_id)
                const bound = boundByConnection.get(key)
                return (
                  <li key={key}>
                    <label className="flex items-center gap-2 px-3 py-2 text-sm transition-colors hover:bg-muted">
                      <Checkbox
                        checked={isChecked(key)}
                        onCheckedChange={(next) => toggle(key, next === true)}
                      />
                      <span className="min-w-0 flex-1 truncate font-mono text-xs">{repo.path}</span>
                      <StatusPill tone={bound?.enabled ? 'success' : 'neutral'}>
                        {bound?.enabled ? COPY.repositories.collecting : bound ? COPY.common.disabled : COPY.repositories.notBound}
                      </StatusPill>
                    </label>
                  </li>
                )
              })}
            </ul>
          </>
        )}
      </div>
    </ResponsivePanel>
  )
}
