import { useMemo, useState } from 'react'

import {
  AdaptiveFilterBar,
  Button,
  EmptyState,
  Input,
  LoadingSkeleton,
  ResponsivePanel,
  StatusPill,
  UserPreview,
  type UserSelectOption,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmBindingDTO } from '../api/contracts'
import {
  useBindingsQuery,
  useConfirmBindingMutation,
} from '../hooks'

type MemberBindingRosterProps = {
  members: readonly UserSelectOption[]
  membersPending: boolean
  canView: boolean
  canManage: boolean
  onError: (message: string) => void
}

// 按成员视角的作者绑定：平台全员名册（头像+姓名），每个成员一行，展示其
// 名下已归属的 Git 作者，并可把待确认队列里的作者直接绑到该成员身上。
// 与"按作者"视角共享同一组接口与缓存，只是消费方向相反。
export function MemberBindingRoster({ members, membersPending, canView, canManage, onError }: MemberBindingRosterProps) {
  const bindingsQuery = useBindingsQuery(undefined, canView)
  const confirmMutation = useConfirmBindingMutation(onError)
  const [keyword, setKeyword] = useState('')
  const [bindingMember, setBindingMember] = useState<UserSelectOption | null>(null)

  const allBindings = bindingsQuery.data ?? []
  const authorsByMember = useMemo(() => {
    const map = new Map<number, ScmBindingDTO[]>()
    for (const binding of allBindings) {
      if (!binding.user_id) continue
      const list = map.get(binding.user_id) ?? []
      list.push(binding)
      map.set(binding.user_id, list)
    }
    return map
  }, [allBindings])
  const unboundPool = useMemo(
    () => allBindings.filter((binding) => binding.status === 'unmatched' || binding.status === 'suggested'),
    [allBindings],
  )
  const visibleMembers = useMemo(() => {
    const query = keyword.trim().toLocaleLowerCase()
    if (!query) return members
    return members.filter((member) =>
      [member.name, member.username].some((text) => text?.toLocaleLowerCase().includes(query)),
    )
  }, [keyword, members])

  const assign = (binding: ScmBindingDTO, memberId: number) => {
    // 保留面板开启，管理员可以连续把多个作者绑到同一个人身上。
    confirmMutation.mutate({ id: binding.id, userId: memberId })
  }

  if (!canView) {
    return <EmptyState title={COPY.common.empty} />
  }
  if (membersPending) {
    return <LoadingSkeleton />
  }
  return (
    <div className="grid gap-3">
      <AdaptiveFilterBar controls={[{ id: 'search', label: COPY.bindings.memberSearch, placement: 'search', node: (<Input
          aria-label={COPY.bindings.memberSearch}
          placeholder={COPY.bindings.memberSearch}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          className="w-full"
        />) }]} endActions={<p className="text-xs text-muted-foreground">
          {COPY.bindings.rosterSummary(members.length, authorsByMember.size)}
        </p>} />
      {bindingsQuery.isError ? (
        <EmptyState
          title={COPY.errors.loadFailed}
          action={<Button variant="outline" onClick={() => void bindingsQuery.refetch()}>{COPY.common.retry}</Button>}
        />
      ) : visibleMembers.length === 0 ? (
        <EmptyState title={COPY.common.empty} />
      ) : (
        // 名册通栏贴卡：单容器分组列表 + 行分隔线，替代逐行描边的碎片观感。
        <div className="-mx-4 -mb-4">
          <ul className="divide-y border-t" aria-label={COPY.bindings.viewByMember}>
            {visibleMembers.map((member) => {
              const bound = authorsByMember.get(member.id) ?? []
              return (
                <li
                  key={member.id}
                  className="flex flex-wrap items-center gap-3 px-4 py-2.5 transition-colors hover:bg-muted/40"
                >
                  <UserPreview
                    name={member.name}
                    secondary={member.username}
                    avatarUrl={member.avatarUrl}
                    size="sm"
                    className="w-44 shrink-0"
                  />
                  <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
                    {bound.length === 0 ? (
                      <span className="text-xs text-muted-foreground">{COPY.bindings.boundEmpty}</span>
                    ) : (
                      bound.map((binding) => (
                        <span
                          key={binding.id}
                          title={binding.author_key}
                          className="inline-flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs"
                        >
                          {binding.display_name || binding.author_key}
                          {binding.status === 'suggested' ? (
                            <StatusPill tone="info">{COPY.bindings.statuses.suggested}</StatusPill>
                          ) : null}
                        </span>
                      ))
                    )}
                  </div>
                  {canManage ? (
                    <Button variant="outline" size="sm" onClick={() => setBindingMember(member)}>
                      {COPY.bindings.bindAuthor}
                    </Button>
                  ) : null}
                </li>
              )
            })}
          </ul>
        </div>
      )}
      {bindingMember !== null ? (
        <ResponsivePanel
          open
          onOpenChange={(open) => { if (!open) setBindingMember(null) }}
          title={COPY.bindings.poolTitle(bindingMember.name)}
          closeLabel={COPY.common.close}
        >
          <p className="text-xs text-muted-foreground">{COPY.bindings.description}</p>
          {unboundPool.length === 0 ? (
            <EmptyState title={COPY.bindings.poolEmpty} />
          ) : (
            <ul className="divide-y rounded-lg border" aria-label={COPY.bindings.poolTitle(bindingMember.name)}>
              {unboundPool.map((binding) => {
                const suggestedName = binding.suggested_user_id
                  ? members.find((member) => member.id === binding.suggested_user_id)?.name
                    ?? `ID ${binding.suggested_user_id}`
                  : undefined
                return (
                  <li
                    key={binding.id}
                    className="flex items-center gap-2 px-3 py-2.5"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">
                        {binding.display_name || binding.author_key}
                      </p>
                      <p className="truncate font-mono text-xs text-muted-foreground">
                        {binding.display_name ? `${binding.author_key} · ` : ''}
                        {COPY.bindings.identityLine(binding.key_kind, binding.provider)}
                      </p>
                      {suggestedName ? (
                        <p className="text-xs text-muted-foreground">
                          {COPY.bindings.suggestedLine(suggestedName)}
                        </p>
                      ) : null}
                    </div>
                    {binding.status === 'suggested' ? (
                      <StatusPill tone="info">{COPY.bindings.statuses.suggested}</StatusPill>
                    ) : null}
                    <Button
                      size="sm"
                      disabled={confirmMutation.isPending}
                      onClick={() => assign(binding, bindingMember.id)}
                    >
                      {COPY.bindings.assign}
                    </Button>
                  </li>
                )
              })}
            </ul>
          )}
        </ResponsivePanel>
      ) : null}
    </div>
  )
}
