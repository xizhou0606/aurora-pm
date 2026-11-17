import { MetricStrip, OperationalPage, type MetricStripItem } from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import {
  BindingsSection,
  ConnectionsSection,
  RepositoriesSection,
  SettingsSection,
} from '../components'
import {
  useBindingsQuery,
  useConnectionsQuery,
  useRepositoriesQuery,
  useScmConfigAccess,
  useSettingsQuery,
} from '../hooks'

// 顶部接入概览：与四个分节共享同一组 query 缓存，只读汇总不触发额外请求。
// 任一区块无权限或加载失败时该项显示占位符，不阻塞其余指标。
function ScmOverviewMetrics() {
  const access = useScmConfigAccess()
  const connectionsQuery = useConnectionsQuery(access.canView('connections'))
  const repositoriesQuery = useRepositoriesQuery(access.canView('repositories'))
  const bindingsQuery = useBindingsQuery(undefined, access.canView('bindings'))
  const settingsQuery = useSettingsQuery(access.canView('settings'))

  const countOf = (data: readonly unknown[] | undefined, ready: boolean) =>
    ready && data ? String(data.length) : '—'
  const connections = connectionsQuery.data
  const repositories = repositoriesQuery.data
  const bindings = bindingsQuery.data
  const settings = settingsQuery.data
  const pendingAuthors = bindings
    ? bindings.filter((binding) => binding.status === 'unmatched' || binding.status === 'suggested').length
    : undefined

  const items: MetricStripItem[] = [
    { id: 'connections', label: COPY.page.metrics.connections, value: countOf(connections, access.canView('connections')) },
    {
      id: 'repositories',
      label: COPY.page.metrics.repositories,
      value: countOf(repositories, access.canView('repositories')),
      description: repositories && repositories.length > 0
        ? COPY.page.metrics.collecting(repositories.filter((repo) => repo.enabled).length)
        : undefined,
    },
    { id: 'pending-authors', label: COPY.page.metrics.pendingAuthors, value: access.canView('bindings') && pendingAuthors !== undefined ? String(pendingAuthors) : '—' },
    {
      id: 'bot-rules',
      label: COPY.page.metrics.botRules,
      value: access.canView('settings') && settings ? String(settings.bot_patterns?.length ?? 0) : '—',
    },
  ]

  return (
    <MetricStrip
      aria-label={COPY.page.metrics.label}
      items={items}
      className="grid-cols-2 sm:grid-cols-4 lg:grid-cols-4 xl:grid-cols-4"
    />
  )
}

export function ScmConfigPage() {
  return (
    <OperationalPage title={COPY.page.title} description={COPY.page.description}>
      <div className="mx-auto grid w-full max-w-[96rem] gap-4 p-4">
        <ScmOverviewMetrics />
        <SettingsSection />
        <ConnectionsSection />
        <RepositoriesSection />
        <BindingsSection />
      </div>
    </OperationalPage>
  )
}
