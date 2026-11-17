import type { AuthorizationRecordsControl } from './authorization-records'
import { getPMRuntimeConfig } from '@/shared/config'

import type {
  AuthorizationRequest,
  DeviceAuthorization,
  ExternalToolingEffectMode,
  EffectiveToolingAccess,
  ToolCatalogItem,
} from './contract'

export type CollectionState<T> =
  | { status: 'loading' }
  | { status: 'denied' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty' }
  | { status: 'ready'; items: T[] }

export type CopyStatus = 'idle' | 'copied' | 'error'
export type CliCommandCopyTarget = `cli_command_${0 | 1 | 2 | 3 | 4}`
export type ConfigurationCopyTarget = 'mcp_configuration' | 'mcp_prompt' | 'cli_prompt' | CliCommandCopyTarget
export type CopyState = {
  target: ConfigurationCopyTarget | null
  status: CopyStatus
}
export const mcpClientKinds = ['codex', 'claude_code', 'goose', 'opencode', 'generic'] as const
export type McpClientKind = typeof mcpClientKinds[number]
export type GrantDraftValidation = 'none' | 'tools_required' | 'time_required' | 'time_order' | 'time_expired' | 'cli_ttl_invalid'
export type ToolConfigurationSaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

export type ToolTreeNode =
  | {
      kind: 'group'
      id: string
      label: string
      toolCount: number
      children: ToolTreeNode[]
    }
  | {
      kind: 'tool'
      id: string
      tool: ToolCatalogItem
    }

export type GrantDraftControl = {
  mode: ExternalToolingEffectMode
  toolScopes: string[]
  effectiveFrom: string
  expiresAt: string
  cliGrantTtlHours: number
  validation: GrantDraftValidation
  setMode: (mode: ExternalToolingEffectMode) => void
  setToolsSelected: (toolNames: readonly string[], selected: boolean) => void
  setEffectiveFrom: (value: string) => void
  setExpiresAt: (value: string) => void
  setCliGrantTtlHours: (value: number) => void
}

export type ToolConfigurationControl = {
  status: ToolConfigurationSaveStatus
  canSave: boolean
  effectiveMode: ExternalToolingEffectMode
  effectiveToolCount: number
  catalogToolCount: number
  save: () => void
}

export type ConfigurationScreenModel = {
  viewMode: 'manage' | 'effective'
  effectiveAccess:
    | { status: 'loading' }
    | { status: 'error'; retry: () => void }
    | { status: 'ready'; data: EffectiveToolingAccess }
    | null
  catalog: CollectionState<ToolCatalogItem>
  canViewRecords: boolean
  records: AuthorizationRecordsControl
  mcpConfiguration: string
  mcpClient: {
    value: McpClientKind
    setValue: (value: McpClientKind) => void
  }
  cliCommands: readonly string[]
  copyState: CopyState
  grantDraft: GrantDraftControl
  toolConfiguration: ToolConfigurationControl
  canEditConfiguration: boolean
  copyConfiguration: (target: ConfigurationCopyTarget) => void
}

export type AuthorizationScreenModel =
  | { status: 'invalid' }
  | { status: 'forbidden' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | {
      status: 'ready'
      request: AuthorizationRequest
      submit: 'idle' | 'pending' | 'error'
      canDecide: boolean
      authorize: () => void
      deny: () => void
    }
  | { status: 'authorized'; request: AuthorizationRequest }
  | { status: 'denied'; request: AuthorizationRequest }
  | { status: 'redirecting'; request: AuthorizationRequest }

export type DeviceAuthorizationScreenModel =
  | { status: 'invalid' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | {
      status: 'ready'
      request: DeviceAuthorization
      submit: 'idle' | 'pending' | 'error'
      authorize: () => void
      deny: () => void
    }
  | { status: 'approved'; request: DeviceAuthorization }
  | { status: 'denied'; request: DeviceAuthorization }
  | { status: 'consumed'; request: DeviceAuthorization }
  | { status: 'expired'; request: DeviceAuthorization }

export type ConfigurationAccess = {
  viewCatalog: boolean
  viewGrants: boolean
  revokeGrants?: boolean
  viewConfiguration: boolean
  updateConfiguration: boolean
}

export type AuthorizationAccess = {
  createAuthorizationRequest: boolean
  viewAuthorizationRequest: boolean
  decideAuthorizationRequest: boolean
}

function mcpEndpoint(): string {
  const configured = getPMRuntimeConfig().apiBaseUrl
  const base = configured || window.location.origin
  if (/\/api\/v1\/(?:core|platform)$/i.test(base)) {
    return base.replace(/\/(?:core|platform)$/i, '/mcp')
  }
  if (/\/api\/v1$/i.test(base)) return `${base}/mcp`
  return `${base.replace(/\/+$/, '')}/api/v1/mcp`
}

export function buildMcpConfiguration(client: McpClientKind = 'generic'): string {
  const endpoint = mcpEndpoint()
  if (client === 'codex') {
    return `[mcp_servers.pm]\nurl = ${JSON.stringify(endpoint)}`
  }
  if (client === 'goose') {
    return [
      'extensions:',
      '  pm:',
      '    type: streamable_http',
      '    name: PM',
      '    enabled: true',
      `    uri: ${JSON.stringify(endpoint)}`,
      '    timeout: 300',
    ].join('\n')
  }
  if (client === 'opencode') {
    return JSON.stringify({
      $schema: 'https://opencode.ai/config.json',
      mcp: { pm: { type: 'remote', url: endpoint } },
    }, null, 2)
  }
  return JSON.stringify({
    mcpServers: {
      pm: {
        type: 'http',
        url: endpoint,
      },
    },
  }, null, 2)
}

export const CONNECTIVITY_TOOL_NAME = 'workflow.template.list'
export const CONNECTIVITY_TOOL_COMMAND = `pm tool call ${CONNECTIVITY_TOOL_NAME} --input '{}'`
const CLI_INSTALL_COMMAND = 'npm install --global @rc-pm/pm-cli'
const CLI_SKILL_INSTALL_COMMAND = 'pm skill install --global'
const CLI_AUTH_ENSURE_COMMAND = 'pm auth ensure --json'

function cliSetupURL(workspaceId: number): string {
  const url = new URL('/workspace/mcp-cli', window.location.origin)
  url.searchParams.set('workspace', String(workspaceId))
  return url.toString()
}

export function buildCliCommands(workspaceId: number): readonly string[] {
  if (!Number.isSafeInteger(workspaceId) || workspaceId <= 0) return []
  return [
    CLI_INSTALL_COMMAND,
    CLI_SKILL_INSTALL_COMMAND,
    `pm setup --from "${cliSetupURL(workspaceId)}"`,
    'pm tool list',
    CONNECTIVITY_TOOL_COMMAND,
  ]
}

export function buildCliAuthCommands(workspaceId: number): readonly string[] {
  if (!Number.isSafeInteger(workspaceId) || workspaceId <= 0) return []
  return [
    CLI_INSTALL_COMMAND,
    CLI_SKILL_INSTALL_COMMAND,
    `pm setup --from "${cliSetupURL(workspaceId)}" --auth-mode device`,
    CLI_AUTH_ENSURE_COMMAND,
  ]
}

export function buildCliConfiguration(workspaceId: number): string {
  return buildCliCommands(workspaceId).join('\n')
}

export function buildCliAuthConfiguration(workspaceId: number): string {
  return buildCliAuthCommands(workspaceId).join('\n')
}

export function formatToolingDateTime(value: string): string {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function riskRank(risk: ToolCatalogItem['risk']): number {
  return risk === 'high_impact' ? 3 : risk === 'mutation' ? 2 : 1
}

export type ToolRiskCategory = 'regular' | 'high_risk'

export function toolRiskCategory(tool: ToolCatalogItem): ToolRiskCategory {
  return tool.risk === 'read' ? 'regular' : 'high_risk'
}

export function isToolConfigurable(tool: ToolCatalogItem): boolean {
  return tool.lifecycle !== 'draft'
}

type MutableToolGroup = {
  id: string
  label: string
  groups: Map<string, MutableToolGroup>
  tools: ToolCatalogItem[]
}

function presentToolGroup(group: MutableToolGroup): ToolTreeNode {
  const groupChildren = [...group.groups.values()]
    .sort((left, right) => left.label.localeCompare(right.label))
    .map(presentToolGroup)
  const toolChildren: ToolTreeNode[] = group.tools
    .sort((left, right) => left.name.localeCompare(right.name))
    .map((tool) => ({ kind: 'tool', id: `tool:${tool.name}`, tool }))
  const children = [...groupChildren, ...toolChildren]
  return {
    kind: 'group',
    id: group.id,
    label: group.label,
    toolCount: children.reduce(
      (count, child) => count + (child.kind === 'tool' ? 1 : child.toolCount),
      0,
    ),
    children,
  }
}

export function buildToolTree(tools: readonly ToolCatalogItem[]): ToolTreeNode[] {
  const root: MutableToolGroup = {
    id: 'namespace:root',
    label: 'root',
    groups: new Map(),
    tools: [],
  }
  tools.forEach((tool) => {
    const segments = tool.name.split('.').map((segment) => segment.trim()).filter(Boolean)
    if (segments.length < 2) {
      root.tools.push(tool)
      return
    }
    let parent = root
    segments.slice(0, -1).forEach((segment) => {
      const id = `${parent.id}/${segment}`
      let group = parent.groups.get(segment)
      if (!group) {
        group = { id, label: segment, groups: new Map(), tools: [] }
        parent.groups.set(segment, group)
      }
      parent = group
    })
    parent.tools.push(tool)
  })
  return [
    ...[...root.groups.values()]
      .sort((left, right) => left.label.localeCompare(right.label))
      .map(presentToolGroup),
    ...root.tools
      .sort((left, right) => left.name.localeCompare(right.name))
      .map((tool): ToolTreeNode => ({ kind: 'tool', id: `tool:${tool.name}`, tool })),
  ]
}

export function toolNamesInTree(node: ToolTreeNode): string[] {
  if (node.kind === 'tool') return [node.tool.name]
  return node.children.flatMap(toolNamesInTree)
}

export function configurableToolNamesInTree(node: ToolTreeNode): string[] {
  if (node.kind === 'tool') return isToolConfigurable(node.tool) ? [node.tool.name] : []
  return node.children.flatMap(configurableToolNamesInTree)
}

export function toolTreeSelection(
  node: ToolTreeNode,
  selectedToolNames: ReadonlySet<string>,
): false | true | 'mixed' {
  const names = configurableToolNamesInTree(node)
  if (names.length === 0) return false
  const selectedCount = names.reduce(
    (count, name) => count + (selectedToolNames.has(name) ? 1 : 0),
    0,
  )
  if (selectedCount === 0) return false
  return selectedCount === names.length ? true : 'mixed'
}
