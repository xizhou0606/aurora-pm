import { selectCalendarDate } from '@/shared/testing/select-calendar-date'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { pmKeys } from '@/shared/query'
import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeWorkspaceStore } from '@/shared/runtime'

import { AuthorizationScreen } from '../components/AuthorizationScreen'
import { ConfigurationScreen } from '../components/ConfigurationScreen'
import { DeviceAuthorizationScreen } from '../components/DeviceAuthorizationScreen'
import {
  useAuthorizationScreen,
  useConfigurationScreen,
  useDeviceAuthorizationScreen,
} from '../hooks/hooks'
import type {
  AuthorizationAccess,
  AuthorizationScreenModel,
  ConfigurationAccess,
  ConfigurationScreenModel,
} from '../model/model'

const fullConfigurationAccess: ConfigurationAccess = {
  viewCatalog: true,
  viewGrants: true,
  viewConfiguration: true,
  updateConfiguration: true,
}

const fullAuthorizationAccess: AuthorizationAccess = {
  createAuthorizationRequest: true,
  viewAuthorizationRequest: true,
  decideAuthorizationRequest: true,
}

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard')
const originalAdapter = platformApi.defaults.adapter

// Keep the grant window active as calendar time advances, without changing the
// process-wide clock used by other parallel test files.
const windowStart = new Date()
windowStart.setUTCDate(windowStart.getUTCDate() - 1)
windowStart.setUTCHours(1, 0, 0, 0)
const windowEnd = new Date(windowStart)
windowEnd.setUTCDate(windowEnd.getUTCDate() + 31)
const startInput = `${windowStart.toISOString().slice(0, 10)}T09:00`
const endInput = `${windowEnd.toISOString().slice(0, 10)}T09:00`
const startLabel = `${windowStart.getUTCFullYear()}年${windowStart.getUTCMonth() + 1}月${windowStart.getUTCDate()}日 01:00`
const endLabel = `${windowEnd.getUTCFullYear()}年${windowEnd.getUTCMonth() + 1}月${windowEnd.getUTCDate()}日 01:00`

beforeEach(() => {
  localStorage.clear()
  runtimeWorkspaceStore.getState().selectWorkspace(7)
})

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, 'clipboard', originalClipboard)
  else Reflect.deleteProperty(navigator, 'clipboard')
  platformApi.defaults.adapter = originalAdapter
  runtimeWorkspaceStore.getState().clearWorkspace()
  localStorage.clear()
})

function configurationModel(overrides: Partial<ConfigurationScreenModel> = {}): ConfigurationScreenModel {
  return {
    viewMode: 'manage',
    effectiveAccess: null,
    catalog: { status: 'ready', items: [
      {
        name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grantScope: 'project:read',
      },
      {
        name: 'project.set.get', version: '1.0.0', summary: 'Get project set', lifecycle: 'active', risk: 'read', grantScope: 'project:read',
      },
      {
        name: 'task.get', version: '1.0.0', summary: 'Get task', lifecycle: 'active', risk: 'read', grantScope: 'task:read',
      },
    ] },
    canViewRecords: true,
    records: {
      state: { status: 'ready', rows: [{ id: 'record_1', user_code: 'TEST-CODE', channel: 'mcp', client_name: 'Codex', principal_id: 109, principal_name: 'Admin', grant_tools: ['project.read'], created_at: '2026-08-27T01:00:00Z', decided_at: '2026-08-27T01:00:00Z', expires_at: '2026-09-27T01:00:00Z', status: 'approved', code_valid: false, credential_valid: true, can_revoke: true }] },
      page: 1, pageSize: 20, total: 1, setPage: vi.fn(), setPageSize: vi.fn(), selected: null, select: vi.fn(), close: vi.fn(), confirm: vi.fn(), pending: false,
    },
    mcpConfiguration: '{"mcpServers":{"pm":{"url":"https://pm.example.test/api/v1/mcp"}}}',
    mcpClient: { value: 'generic', setValue: vi.fn() },
    cliCommands: [
      'npm install --global @rc-pm/pm-cli',
      'pm skill install --global',
      'pm setup --from "https://pm.example.test/workspace/mcp-cli?workspace=7"',
      'pm tool list',
      "pm tool call workflow.template.list --input '{}'",
    ],
    copyState: { target: null, status: 'idle' },
    grantDraft: {
      mode: 'window',
      toolScopes: ['project.read'],
      cliGrantTtlHours: 168,
      effectiveFrom: '2026-08-27T09:00',
      expiresAt: '2026-09-27T09:00',
      validation: 'none',
      setMode: vi.fn(),
      setToolsSelected: vi.fn(),
      setCliGrantTtlHours: vi.fn(),
      setEffectiveFrom: vi.fn(),
      setExpiresAt: vi.fn(),
    },
    toolConfiguration: {
      status: 'idle',
      canSave: false,
      effectiveMode: 'window',
      effectiveToolCount: 1,
      catalogToolCount: 3,
      save: vi.fn(),
    },
    canEditConfiguration: true,
    copyConfiguration: vi.fn(),
    ...overrides,
  }
}

function renderConfigurationHook(
  access = fullConfigurationAccess,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Harness() {
    return <ConfigurationScreen model={useConfigurationScreen(access)} />
  }
  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <Harness />
      </QueryClientProvider>,
    ),
  }
}

function renderAuthorizationRequestHook(requestId = 'request_1') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Harness() {
    const model = useAuthorizationScreen(
      { requestId },
      fullAuthorizationAccess,
      vi.fn(),
    )
    return <AuthorizationScreen model={model} onReturn={vi.fn()} />
  }
  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <Harness />
      </QueryClientProvider>,
    ),
  }
}

function renderDeviceAuthorizationHook(userCode = 'ABCD-EFGH') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Harness() {
    return (
      <DeviceAuthorizationScreen
        model={useDeviceAuthorizationScreen(userCode)}
        onReturn={vi.fn()}
      />
    )
  }
  return render(
    <QueryClientProvider client={queryClient}>
      <Harness />
    </QueryClientProvider>,
  )
}

function renderAuthorizationLinkHook(onRequestStarted = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Harness() {
    const model = useAuthorizationScreen(
      { channel: 'mcp' },
      fullAuthorizationAccess,
      onRequestStarted,
    )
    return <AuthorizationScreen model={model} onReturn={vi.fn()} />
  }
  return {
    onRequestStarted,
    ...render(
      <QueryClientProvider client={queryClient}>
        <Harness />
      </QueryClientProvider>,
    ),
  }
}

function ruleRadio(mode: 'permanent' | 'disabled' | 'window'): HTMLElement {
  return screen.getByRole('radio', { name: copy.mcpCli.tools.rule[mode] })
}

type ConfigurationRequest = { method?: string; url?: string; data?: unknown; workspaceId?: string }

type SavedConfiguration = {
  workspace_id: number
  mode: 'permanent' | 'disabled' | 'window'
  tool_scopes: string[]
  grant_scopes: string[]
  cli_grant_ttl_hours: number
  effective_from: string | null
  expires_at: string | null
  updated_by: number
  created_at: string
  updated_at: string
  version: number
}

const savedConfiguration: SavedConfiguration = {
  workspace_id: 7,
  mode: 'window',
  tool_scopes: ['project.read'],
  grant_scopes: ['project:read'],
  cli_grant_ttl_hours: 168,
  effective_from: windowStart.toISOString(),
  expires_at: windowEnd.toISOString(),
  updated_by: 1,
  created_at: '2026-08-26T01:00:00Z',
  updated_at: '2026-08-26T01:00:00Z',
  version: 1,
}

function installConfigurationAdapter(
  requests: ConfigurationRequest[],
  options: {
    configuration?: typeof savedConfiguration | null
    configurations?: Record<string, typeof savedConfiguration | null>
    failSave?: boolean
  } = {},
) {
  let serverConfiguration = options.configuration ?? null
  platformApi.defaults.adapter = async (config) => {
    const workspaceId = String(config.headers?.['X-Workspace-ID'] ?? '')
    requests.push({ method: config.method, url: config.url, data: config.data, workspaceId })
    if (config.url === '/external-tooling/effective-access' && config.method === 'get') {
      return {
        data: { data: {
          configured: true,
          mode: 'window',
          status: 'active',
          tools: [{
            name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grant_scope: 'project:read',
          }],
          cli_grant_ttl_hours: 168,
          effective_from: '2026-08-27T01:00:00Z',
          expires_at: '2026-09-27T01:00:00Z',
        } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    if (config.url === '/external-tooling/configuration' && config.method === 'get') {
      const workspaceConfiguration = options.configurations
        ? options.configurations[workspaceId]
        : serverConfiguration
      if (!workspaceConfiguration) {
        throw {
          response: {
            status: 404,
            data: { error: { code: 'EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND' } },
          },
        }
      }
      return { data: { data: workspaceConfiguration }, status: 200, statusText: 'OK', headers: {}, config }
    }
    if (config.url === '/external-tooling/configuration' && config.method === 'put') {
      if (options.failSave) {
        throw {
          response: {
            status: 500,
            data: { code: 'EXTERNAL_TOOLING_CONFIGURATION_SAVE_FAILED' },
          },
        }
      }
      const input = JSON.parse(String(config.data)) as {
        mode: 'permanent' | 'disabled' | 'window'
        tool_scopes: string[]
        cli_grant_ttl_hours: number
        effective_from: string | null
        expires_at: string | null
      }
      serverConfiguration = {
        ...savedConfiguration,
        mode: input.mode,
        tool_scopes: input.tool_scopes,
        cli_grant_ttl_hours: input.cli_grant_ttl_hours,
        effective_from: input.effective_from,
        expires_at: input.expires_at,
        version: (serverConfiguration?.version ?? 0) + 1,
      }
      return { data: { data: serverConfiguration }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const data = config.url === '/tool-runtime/catalog'
      ? { tools: [
          {
            name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grant_scope: 'project:read',
          },
          {
            name: 'project.set.get', version: '1.0.0', summary: 'Get project set', lifecycle: 'active', risk: 'read', grant_scope: 'project:read',
          },
          {
            name: 'task.get', version: '1.0.0', summary: 'Get task', lifecycle: 'active', risk: 'read', grant_scope: 'task:read',
          },
          {
            name: 'task.update', version: '1.0.0', summary: 'Update task', lifecycle: 'draft', risk: 'mutation', grant_scope: 'task:write',
          },
        ] }
      : config.url === '/external-tooling/grants'
        ? { grants: [] }
        : { request_id: 'request_1' }
    return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
  }
}

describe('MCP/CLI configuration screen', () => {
  it('shows full-width configuration, compact usage guidance, tools, grants and UAT time', () => {
    const model = configurationModel()
    render(<ConfigurationScreen model={model} />)

    expect(screen.getByRole('heading', { name: copy.mcpCli.mcp.title })).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.mcp.usageHint)).toBeTruthy()
    expect(screen.getByLabelText(copy.mcpCli.mcp.configurationLabel).textContent).toContain('/api/v1/mcp')
    const clientSelect = screen.getByRole('combobox', { name: copy.mcpCli.mcp.clientLabel })
    expect(clientSelect.textContent).toContain(copy.mcpCli.mcp.clients.generic)
    fireEvent.click(clientSelect)
    const gooseOption = screen.getByRole('option', { name: copy.mcpCli.mcp.clients.goose })
    fireEvent.pointerDown(gooseOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(gooseOption)
    expect(model.mcpClient.setValue).toHaveBeenCalledWith('goose')
    expect(screen.getByRole('heading', { name: copy.mcpCli.tools.title })).toBeTruthy()
    expect(screen.getByText(`${copy.mcpCli.tools.effectiveRule}${copy.mcpCli.fieldSeparator}${copy.mcpCli.tools.rule.window}`)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.tools.effectiveToolCount(1, 3))).toBeTruthy()
    expect(screen.getByRole('tab', { name: new RegExp(copy.mcpCli.tools.riskTabs.regular) })).toBeTruthy()
    expect(screen.getByRole('tab', { name: new RegExp(copy.mcpCli.tools.riskTabs.highRisk) })).toBeTruthy()
    expect(screen.getByText('project.read')).toBeTruthy()
    expect(screen.getAllByText(copy.mcpCli.grants.effectiveFrom, { exact: false }).length).toBeGreaterThan(0)

    const projectGroup = screen.getByRole('button', { name: copy.mcpCli.tools.collapseGroup('project') })
    expect(projectGroup.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('button', { name: copy.mcpCli.tools.collapseGroup('set') })).toBeTruthy()
    expect(screen.getByText('task.get')).toBeTruthy()
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.enableGroup('project'),
    }).getAttribute('aria-checked')).toBe('mixed')
    fireEvent.click(projectGroup)
    expect(screen.queryByText('project.read')).toBeNull()
    const collapsedProject = screen.getByRole('button', { name: copy.mcpCli.tools.expandGroup('project') })
    fireEvent.keyDown(collapsedProject, { key: 'ArrowRight' })
    expect(screen.getByText('project.read')).toBeTruthy()

    fireEvent.click(screen.getByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') }))
    expect(model.grantDraft.setToolsSelected).toHaveBeenCalledWith(['project.read'], false)

    const mcpSection = screen.getByRole('heading', { name: copy.mcpCli.mcp.title }).closest('section')!
    fireEvent.click(within(mcpSection).getByRole('button', { name: copy.mcpCli.configurationActions.copy }))
    fireEvent.click(within(mcpSection).getByRole('button', { name: copy.mcpCli.configurationActions.prompt }))
    expect(model.copyConfiguration).toHaveBeenNthCalledWith(1, 'mcp_configuration')
    expect(model.copyConfiguration).toHaveBeenNthCalledWith(2, 'mcp_prompt')

    const cliSection = screen.getByRole('heading', { name: copy.mcpCli.cli.title }).closest('section')!
    expect(within(cliSection).getByText(copy.mcpCli.cli.recommended)).toBeTruthy()
    expect([...cliSection.parentElement!.children].indexOf(cliSection)).toBeLessThan(
      [...cliSection.parentElement!.children].indexOf(mcpSection),
    )
    expect(within(cliSection).queryByRole('button', { name: copy.mcpCli.configurationActions.copy })).toBeNull()
    const commandLabels = [
      copy.mcpCli.cli.install,
      copy.mcpCli.cli.skills,
      copy.mcpCli.cli.setup,
      copy.mcpCli.cli.discover,
      copy.mcpCli.cli.verify,
    ]
    commandLabels.forEach((label) => {
      expect(within(cliSection).getByRole('button', {
        name: copy.mcpCli.configurationActions.copyCommand(label),
      })).toBeTruthy()
    })
    fireEvent.click(within(cliSection).getByRole('button', {
      name: copy.mcpCli.configurationActions.copyCommand(copy.mcpCli.cli.install),
    }))
    fireEvent.click(within(cliSection).getByRole('button', { name: copy.mcpCli.configurationActions.prompt }))
    expect(model.copyConfiguration).toHaveBeenNthCalledWith(3, 'cli_command_0')
    expect(model.copyConfiguration).toHaveBeenNthCalledWith(4, 'cli_prompt')
    expect(within(mcpSection).getByText(copy.mcpCli.mcp.usageHint)).toBeTruthy()
    expect(within(cliSection).getByText(copy.mcpCli.cli.usageHint)).toBeTruthy()
  })

  it('collapses and expands every tool group without changing selection', () => {
    const model = configurationModel()
    render(<ConfigurationScreen model={model} />)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.collapseAll }))
    expect(screen.queryByText('project.read')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.expandAll }))
    expect(screen.getByText('project.read')).toBeTruthy()
    expect(model.grantDraft.setToolsSelected).not.toHaveBeenCalled()
  })

  it('shows authorization codes and revocation actions with server pagination', async () => {
    const model = configurationModel()
    render(<ConfigurationScreen model={model} />)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.grants.openRecords }))
    const panel = await screen.findByRole('dialog', { name: copy.mcpCli.grants.title })
    expect(within(panel).getByText('TEST-CODE')).toBeTruthy()
    expect(within(panel).getByText('Admin')).toBeTruthy()
    expect(within(panel).getByRole('columnheader', { name: copy.mcpCli.grants.credentialValid })).toBeTruthy()
    expect(within(panel).getByText(copy.mcpCli.grants.valid, { exact: true })).toBeTruthy()
    fireEvent.click(within(panel).getByRole('button', { name: copy.mcpCli.grants.revoke }))
    expect(model.records.select).toHaveBeenCalledTimes(1)
  })

  it('opens authorization records in a modal and restores the page after closing', async () => {
    render(<ConfigurationScreen model={configurationModel()} />)

    expect(screen.queryByText(copy.mcpCli.grants.recordStatus.approved)).toBeNull()
    const trigger = screen.getByRole('button', { name: copy.mcpCli.grants.openRecords })
    fireEvent.click(trigger)

    const panel = await screen.findByRole('dialog', { name: copy.mcpCli.grants.title })
    expect(within(panel).getByText(copy.mcpCli.grants.recordStatus.approved)).toBeTruthy()
    fireEvent.click(within(panel).getByRole('button', { name: copy.mcpCli.grants.close }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: copy.mcpCli.grants.title })).toBeNull())
  })

  it.each([
    ['loading', { status: 'loading' } as const, copy.mcpCli.grants.loading],
    ['empty', { status: 'empty' } as const, copy.mcpCli.grants.emptyTitle],
  ])('shows the %s authorization-record state in the modal', async (_status, grants, expected) => {
    render(<ConfigurationScreen model={configurationModel({ records: { ...configurationModel().records, state: grants.status === 'loading' ? { ...grants, label: expected } : { ...grants, title: expected } } })} />)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.grants.openRecords }))
    const panel = await screen.findByRole('dialog', { name: copy.mcpCli.grants.title })
    if (_status === 'loading') {
      expect(within(panel).getByRole('status', { name: expected })).toBeTruthy()
    } else {
      expect(within(panel).getByText(expected)).toBeTruthy()
    }
  })

  it('keeps a failed authorization-record load retryable inside the modal', async () => {
    const retry = vi.fn()
    render(<ConfigurationScreen model={configurationModel({ records: { ...configurationModel().records, state: { status: 'error', title: copy.mcpCli.grants.loadFailedTitle, onRetry: retry, retryLabel: copy.mcpCli.grants.retry } } })} />)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.grants.openRecords }))
    const panel = await screen.findByRole('dialog', { name: copy.mcpCli.grants.title })
    expect(within(panel).getByRole('alert').textContent).toContain(copy.mcpCli.grants.loadFailedTitle)
    fireEvent.click(within(panel).getByRole('button', { name: copy.mcpCli.grants.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('copies executable Agent prompts built from the live MCP and CLI configurations', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    renderConfigurationHook()

    const mcpSection = (await screen.findByRole('heading', { name: copy.mcpCli.mcp.title })).closest('section')!
    fireEvent.click(within(mcpSection).getByRole('button', { name: copy.mcpCli.configurationActions.prompt }))
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1))
    expect(writeText.mock.calls[0]?.[0]).toContain('/api/v1/mcp')
    expect(writeText.mock.calls[0]?.[0]).toContain('不要整文件覆盖')
    expect(writeText.mock.calls[0]?.[0]).toContain('workflow.template.list')
    expect(writeText.mock.calls[0]?.[0]).toContain('items')

    const cliSection = screen.getByRole('heading', { name: copy.mcpCli.cli.title }).closest('section')!
    fireEvent.click(within(cliSection).getByRole('button', { name: copy.mcpCli.configurationActions.prompt }))
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(2))
    expect(writeText.mock.calls[1]?.[0]).toContain('npm install --global @rc-pm/pm-cli')
    expect(writeText.mock.calls[1]?.[0]).toContain('pm skill install --global')
    expect(writeText.mock.calls[1]?.[0]).toContain('pm setup --from')
    expect(writeText.mock.calls[1]?.[0]).toContain('--auth-mode device')
    expect(writeText.mock.calls[1]?.[0]).toContain('不会回跳 Agent 的 127.0.0.1')
    expect(writeText.mock.calls[1]?.[0]).toContain('pm auth ensure --json')
    expect(writeText.mock.calls[1]?.[0]).toContain('workspace=7')
    expect(writeText.mock.calls[1]?.[0]).toContain('不要再要求我提供 serverUrl 或 workspaceId')
    expect(writeText.mock.calls[1]?.[0]).toContain('不要展示 `auth ensure` 命令')
    expect(writeText.mock.calls[1]?.[0]).toContain('当前用户：<displayName> (@<username>, ID <id>)')
    expect(writeText.mock.calls[1]?.[0]).toContain('当前工作区：<workspaceId>')
    expect(writeText.mock.calls[1]?.[0]).toContain('pm auth ensure --json\n```')
    expect(writeText.mock.calls[1]?.[0]).not.toContain('\npm tool list\n')
    expect(writeText.mock.calls[1]?.[0]).not.toContain("pm tool call workflow.template.list --input '{}'")
    expect(writeText.mock.calls[1]?.[0]).toContain('CLI 必须直接调用 PM Tool Runtime')
  })

  it('copies each CLI command independently', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    renderConfigurationHook()

    const cliSection = (await screen.findByRole('heading', { name: copy.mcpCli.cli.title })).closest('section')!
    fireEvent.click(within(cliSection).getByRole('button', {
      name: copy.mcpCli.configurationActions.copyCommand(copy.mcpCli.cli.install),
    }))
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1))
    expect(writeText).toHaveBeenNthCalledWith(1, 'npm install --global @rc-pm/pm-cli')

    fireEvent.click(within(cliSection).getByRole('button', {
      name: copy.mcpCli.configurationActions.copyCommand(copy.mcpCli.cli.setup),
    }))
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(2))
    expect(writeText.mock.calls[1]?.[0]).toMatch(/^pm setup --from /)
    expect(writeText.mock.calls[1]?.[0]).toContain('workspace=7')
    expect(writeText.mock.calls[1]?.[0]).not.toContain('--auth-mode device')
  })

  it('derives parent all, none and indeterminate selection from leaf Tools', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    renderConfigurationHook()

    const projectParent = await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.enableGroup('project'),
    })
    expect(projectParent.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(projectParent)

    const enabledProjectParent = screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.disableGroup('project'),
    })
    expect(enabledProjectParent.getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.set.get'),
    }).getAttribute('aria-checked')).toBe('true')

    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }))
    const mixedProjectParent = screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.enableGroup('project'),
    })
    expect(mixedProjectParent.getAttribute('aria-checked')).toBe('mixed')
    expect(mixedProjectParent.hasAttribute('data-indeterminate')).toBe(true)

    fireEvent.click(mixedProjectParent)
    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.disableGroup('project'),
    }))
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.enableGroup('project'),
    }).getAttribute('aria-checked')).toBe('false')
    expect(requests.filter((request) => request.method === 'put' || request.method === 'post')).toHaveLength(0)
  })

  it('shows the complete catalog while keeping draft Tools unavailable', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    renderConfigurationHook()

    fireEvent.click(await screen.findByRole('tab', { name: new RegExp(copy.mcpCli.tools.riskTabs.highRisk) }))

    const draft = await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.update'),
    })
    expect(draft.hasAttribute('disabled') || draft.hasAttribute('data-disabled')).toBe(true)
    expect(screen.getByText(copy.mcpCli.lifecycle.draft)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.tools.unavailable)).toBeTruthy()
    expect(draft.getAttribute('aria-checked')).toBe('false')
    expect(screen.getByText(copy.mcpCli.tools.riskTabs.highRiskDescription)).toBeTruthy()

    fireEvent.click(screen.getByRole('tab', { name: new RegExp(copy.mcpCli.tools.riskTabs.regular) }))
    const taskGroup = screen.getByRole('checkbox', { name: copy.mcpCli.tools.enableGroup('task') })
    fireEvent.click(taskGroup)
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.get'),
    }).getAttribute('aria-checked')).toBe('true')
    expect(screen.queryByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.update'),
    })).toBeNull()
  })

  it('lets read-only users browse the full tree while disabling configuration controls', () => {
    render(<ConfigurationScreen model={configurationModel({ canEditConfiguration: false })} />)

    expect(screen.getByText('project.read')).toBeTruthy()
    const readOnlyTool = screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })
    expect(readOnlyTool.hasAttribute('disabled') || readOnlyTool.hasAttribute('data-disabled')).toBe(true)
  })

  it('shows save only after changes and persists permanent or disabled rules without a time window', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    renderConfigurationHook()

    await screen.findByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') })
    expect(screen.queryByRole('button', { name: copy.mcpCli.tools.save })).toBeNull()

    fireEvent.click(ruleRadio('permanent'))
    expect(screen.queryByLabelText(copy.mcpCli.grants.effectiveFrom)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))
    await waitFor(() => expect(requests.filter((request) => request.method === 'put')).toHaveLength(1))
    expect(JSON.parse(String(requests.filter((request) => request.method === 'put')[0]?.data))).toEqual({
      mode: 'permanent',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 168,
      effective_from: null,
      expires_at: null,
    })
    await waitFor(() => expect(screen.queryByRole('button', { name: copy.mcpCli.tools.save })).toBeNull())
    expect(screen.getByText(`${copy.mcpCli.tools.effectiveRule}${copy.mcpCli.fieldSeparator}${copy.mcpCli.tools.rule.permanent}`)).toBeTruthy()

    fireEvent.click(ruleRadio('disabled'))
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('true')
    fireEvent.click(screen.getByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') }))
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))
    await waitFor(() => expect(requests.filter((request) => request.method === 'put')).toHaveLength(2))
    expect(JSON.parse(String(requests.filter((request) => request.method === 'put')[1]?.data))).toEqual({
      mode: 'disabled',
      tool_scopes: [],
      cli_grant_ttl_hours: 168,
      effective_from: null,
      expires_at: null,
    })
    await waitFor(() => expect(screen.queryByRole('button', { name: copy.mcpCli.tools.save })).toBeNull())
    expect(screen.getByText(copy.mcpCli.tools.effectiveToolCount(0, 4))).toBeTruthy()
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)
  })

  it('persists the configuration on the PM server and restores it after reload without local draft storage', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    const first = renderConfigurationHook()
    const storageBefore = { ...localStorage }

    await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })
    fireEvent.click(ruleRadio('window'))
    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }))
    fireEvent.change(screen.getByLabelText(copy.mcpCli.grants.cliGrantTtl), { target: { value: '30' } })
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.effectiveFrom), startInput, true)
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.expiresAt), endInput, true)
    expect(screen.getByRole('status').textContent).toContain(copy.mcpCli.tools.saveStatus.dirty)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain(copy.mcpCli.tools.saveStatus.saved))
    const put = requests.find((request) => request.method === 'put')
    expect(JSON.parse(String(put?.data))).toEqual({
      mode: 'window',
      tool_scopes: ['project.read'],
      cli_grant_ttl_hours: 30,
      effective_from: new Date(startInput).toISOString(),
      expires_at: new Date(endInput).toISOString(),
    })
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)
    expect({ ...localStorage }).toEqual(storageBefore)

    first.unmount()
    renderConfigurationHook()
    expect((await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })).getAttribute('aria-checked')).toBe('true')
  })

  it('keeps unsaved changes dirty and retryable when the PM server save fails', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { failSave: true })
    renderConfigurationHook()

    await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })
    fireEvent.click(ruleRadio('window'))
    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }))
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.effectiveFrom), startInput, true)
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.expiresAt), endInput, true)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))

    expect((await screen.findByRole('alert')).textContent).toContain(copy.mcpCli.tools.saveStatus.error)
    expect(screen.getByRole('button', { name: copy.mcpCli.tools.save }).hasAttribute('disabled')).toBe(false)
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('true')
    expect(requests.filter((request) => request.method === 'put')).toHaveLength(1)
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)
  })

  it('does not present placeholder tools as ready data when the API fails', () => {
    const retry = vi.fn()
    render(<ConfigurationScreen model={configurationModel({ catalog: { status: 'error', retry } })} />)
    expect(screen.getByRole('alert').textContent).toContain(copy.mcpCli.tools.loadFailedTitle)
    expect(screen.queryByText('project.read')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('keeps configuration read-only while still explaining the client-originated OAuth flow', () => {
    render(<ConfigurationScreen model={configurationModel({
      canEditConfiguration: false,
    })} />)

    expect(screen.getByText(copy.mcpCli.mcp.usageHint)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.cli.usageHint)).toBeTruthy()
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-disabled')).toBe('true')
  })

  it('shows the current-user rule and usable Tools instead of disabled configuration controls', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    renderConfigurationHook({
      ...fullConfigurationAccess,
      updateConfiguration: false,
    })

    expect(await screen.findByRole('heading', { name: copy.mcpCli.effectiveAccess.title })).toBeTruthy()
    expect(await screen.findByText('project.read')).toBeTruthy()
    expect(screen.getByRole('heading', { name: copy.mcpCli.effectiveAccess.ruleTitle })).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.grants.cliGrantTtlValue(168))).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.effectiveAccess.toolCount(1))).toBeTruthy()
    expect(screen.queryByRole('radio')).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByText(copy.mcpCli.mcp.usageHint)).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.mcpCli.tools.save })).toBeNull()
  })

  it('saves configuration without creating a non-OAuth authorization request', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    const { queryClient } = renderConfigurationHook()
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')

    fireEvent.click(await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.get'),
    }))
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))
    await waitFor(() => expect(requests.filter((request) => request.method === 'put')).toHaveLength(1))
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'effectiveAccess'),
    })
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)
  })

  it('reloads a newer server configuration version without requiring a page reload', async () => {
    const requests: ConfigurationRequest[] = []
    const configurations: Record<string, SavedConfiguration | null> = {
      '7': savedConfiguration,
    }
    installConfigurationAdapter(requests, { configurations })
    const { queryClient } = renderConfigurationHook()

    expect((await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })).getAttribute('aria-checked')).toBe('true')

    configurations['7'] = {
      ...savedConfiguration,
      version: 2,
      mode: 'permanent',
      tool_scopes: ['task.get'],
      grant_scopes: ['task:read'],
      effective_from: null,
      expires_at: null,
    }
    await queryClient.invalidateQueries({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'configuration'),
    })

    await waitFor(() => expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.get'),
    }).getAttribute('aria-checked')).toBe('true'))
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('false')
    expect(screen.getByText(
      `${copy.mcpCli.tools.effectiveRule}${copy.mcpCli.fieldSeparator}${copy.mcpCli.tools.rule.permanent}`,
    )).toBeTruthy()
  })

  it('keeps the catalog and saved rule queries fresh while the configuration page stays open', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    const { queryClient } = renderConfigurationHook()

    await screen.findByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') })
    const catalogQuery = queryClient.getQueryCache().find({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'catalog'),
    })
    const configurationQuery = queryClient.getQueryCache().find({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'configuration'),
    })
    for (const query of [catalogQuery, configurationQuery]) {
      expect(query?.options).toMatchObject({
        refetchInterval: 10_000,
        refetchIntervalInBackground: false,
        refetchOnWindowFocus: 'always',
      })
    }
  })

  it('loads self effective access without management-view permissions', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    renderConfigurationHook({
      ...fullConfigurationAccess,
      viewConfiguration: false,
      updateConfiguration: false,
    })

    expect(await screen.findByText('project.read')).toBeTruthy()
    expect(requests.filter((request) => request.url === '/external-tooling/configuration')).toHaveLength(0)
    expect(requests.filter((request) => request.url === '/tool-runtime/catalog')).toHaveLength(0)
    expect(requests.filter((request) => request.url === '/external-tooling/effective-access')).toHaveLength(1)
  })

  it('keeps current-user effective access fresh across administrator changes', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    const { queryClient } = renderConfigurationHook({
      ...fullConfigurationAccess,
      updateConfiguration: false,
    })

    await screen.findByText('project.read')
    const query = queryClient.getQueryCache().find({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'effectiveAccess'),
    })
    expect(query?.options).toMatchObject({
      refetchInterval: 10_000,
      refetchIntervalInBackground: false,
      refetchOnWindowFocus: 'always',
    })
  })

  it('hydrates and saves the server configuration without creating a callback-less authorization', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, { configuration: savedConfiguration })
    renderConfigurationHook()

    expect((await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })).getAttribute('aria-checked')).toBe('true')
    expect(screen.getByLabelText(copy.mcpCli.grants.effectiveFrom).textContent).toContain(startLabel)
    expect(screen.getByLabelText(copy.mcpCli.grants.expiresAt).textContent).toContain(endLabel)

    fireEvent.click(screen.getByRole('checkbox', { name: copy.mcpCli.tools.selectTool('task.get') }))
    expect(screen.getByRole('status').textContent).toContain(copy.mcpCli.tools.saveStatus.dirty)
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)

    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain(copy.mcpCli.tools.saveStatus.saved))
    expect(requests.filter((request) => request.method === 'post')).toHaveLength(0)
    expect(screen.getByText(copy.mcpCli.mcp.usageHint)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.cli.usageHint)).toBeTruthy()
  })

  it('reloads the server baseline when the active workspace changes', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests, {
      configurations: {
        '7': savedConfiguration,
        '8': {
          ...savedConfiguration,
          workspace_id: 8,
          tool_scopes: ['task.get'],
          grant_scopes: ['task:read'],
        },
      },
    })
    renderConfigurationHook()

    expect((await screen.findByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    })).getAttribute('aria-checked')).toBe('true')
    act(() => runtimeWorkspaceStore.getState().selectWorkspace(8))
    await waitFor(() => expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('task.get'),
    }).getAttribute('aria-checked')).toBe('true'))
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('false')
    expect(requests.filter((request) => (
      request.method === 'get'
      && request.url === '/external-tooling/configuration'
      && request.workspaceId === '8'
    ))).toHaveLength(1)
  })

  it('keeps an unconfigured workspace fail-closed without selected Tools', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    renderConfigurationHook()

    await screen.findByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') })
    expect(screen.getByRole('checkbox', {
      name: copy.mcpCli.tools.selectTool('project.read'),
    }).getAttribute('aria-checked')).toBe('false')
    expect(ruleRadio('disabled').getAttribute('aria-checked')).toBe('true')
    expect(screen.queryByLabelText(copy.mcpCli.grants.effectiveFrom)).toBeNull()
    expect(screen.queryByLabelText(copy.mcpCli.grants.expiresAt)).toBeNull()
    expect(requests.filter((request) => request.method === 'put' || request.method === 'post')).toHaveLength(0)
  })

  it('does not save an invalid UAT time window', async () => {
    const requests: ConfigurationRequest[] = []
    installConfigurationAdapter(requests)
    renderConfigurationHook()

    await screen.findByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') })
    fireEvent.click(ruleRadio('window'))
    fireEvent.click(screen.getByRole('checkbox', { name: copy.mcpCli.tools.selectTool('project.read') }))
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.effectiveFrom), '2026-08-28T09:00', true)
    await selectCalendarDate(screen.getByLabelText(copy.mcpCli.grants.expiresAt), '2026-08-27T09:00', true)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.tools.save }))

    expect((await screen.findByRole('alert')).textContent).toContain(copy.mcpCli.grants.timeOrderInvalid)
    expect(requests.filter((request) => request.method === 'put' || request.method === 'post')).toHaveLength(0)
  })
})

describe('MCP/CLI authorization screen', () => {
  const ready: AuthorizationScreenModel = {
    status: 'ready',
    request: {
      id: 'request_1', clientId: 'codex', clientName: 'Codex', channel: 'cli', workspaceId: 7, workspaceName: 'JiGuang',
      mode: 'window',
      oauth: false,
      tools: [{ name: 'task.update', version: '1.0.0', summary: 'Update task', lifecycle: 'active', risk: 'high_impact', grantScope: 'task:write' }],
      effectiveFrom: '2026-08-27T01:00:00Z', expiresAt: '2026-09-27T01:00:00Z',
    },
    submit: 'idle',
    canDecide: true,
    authorize: vi.fn(),
    deny: vi.fn(),
  }

  it('shows the full authorization facts and explicit decisions', () => {
    render(<AuthorizationScreen model={ready} onReturn={vi.fn()} />)
    expect(screen.getByText('Codex')).toBeTruthy()
    expect(screen.getByText('JiGuang (#7)')).toBeTruthy()
    expect(screen.getAllByText(copy.mcpCli.risk.highImpact).length).toBeGreaterThan(0)
    expect(screen.getByText(copy.mcpCli.authorization.effectiveFrom)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.authorization.expiresAt)).toBeTruthy()
    expect(screen.getByText(copy.mcpCli.authorization.mutationWarning)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.authorization.authorize }))
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.authorization.deny }))
    expect(ready.status === 'ready' && ready.authorize).toHaveBeenCalledTimes(1)
    expect(ready.status === 'ready' && ready.deny).toHaveBeenCalledTimes(1)
  })

  it('keeps decisions in the footer while only the tool list scrolls', () => {
    if (ready.status !== 'ready') throw new Error('invalid authorization fixture')
    render(<AuthorizationScreen model={{
      ...ready,
      request: {
        ...ready.request,
        tools: Array.from({ length: 40 }, (_, index) => ({
          name: `task.read.${index + 1}`,
          version: '1.0.0',
          summary: `Read task ${index + 1}`,
          lifecycle: 'active' as const,
          risk: 'read' as const,
          grantScope: 'task:read',
        })),
      },
    }} onReturn={vi.fn()} />)

    const toolList = screen.getByRole('list', { name: copy.mcpCli.authorization.tools })
    const footer = screen.getByRole('button', {
      name: copy.mcpCli.authorization.authorize,
    }).closest('[data-slot="authorization-footer"]')

    expect(toolList.classList.contains('overflow-y-auto')).toBe(true)
    expect(toolList.classList.contains('flex-1')).toBe(true)
    expect(footer?.classList.contains('shrink-0')).toBe(true)
    expect(footer?.contains(screen.getByRole('button', {
      name: copy.mcpCli.authorization.deny,
    }))).toBe(true)
  })

  it('creates a current-session request after the user opens the generic MCP link', async () => {
    const requests: ConfigurationRequest[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push({ method: config.method, url: config.url, data: config.data })
      return {
        data: { data: { request_id: 'request_current_session' } },
        status: 201,
        statusText: 'Created',
        headers: {},
        config,
      }
    }
    const { onRequestStarted } = renderAuthorizationLinkHook()
    await waitFor(() => expect(onRequestStarted).toHaveBeenCalledWith('request_current_session'))
    expect(requests).toHaveLength(1)
    expect(requests[0]).toMatchObject({
      method: 'post',
      url: '/external-tooling/authorization-requests',
    })
    expect(JSON.parse(String(requests[0]?.data))).toEqual({ channel: 'mcp' })
  })

  it('invalidates current-user effective access after authorization succeeds', async () => {
    platformApi.defaults.adapter = async (config) => {
      const data = config.method === 'get'
        ? {
            id: 'request_1', client: { id: 'codex', name: 'Codex' }, channel: 'mcp',
            workspace: { id: 7, name: 'JiGuang' }, mode: 'permanent',
            tools: [{
              name: 'project.read', version: '1.0.0', summary: 'Read project',
              lifecycle: 'active', risk: 'read', grant_scope: 'project:read',
            }],
            effective_from: null, expires_at: null, oauth: false,
          }
        : {}
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const { queryClient } = renderAuthorizationRequestHook()
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries')

    fireEvent.click(await screen.findByRole('button', { name: copy.mcpCli.authorization.authorize }))

    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({
      queryKey: pmKeys.resource(7, 'workspace', 'externalTooling', 'effectiveAccess'),
    }))
    expect(await screen.findByText(copy.mcpCli.authorization.authorized)).toBeTruthy()
  })

  it('does not show a mutation warning for a read-only authorization', () => {
    if (ready.status !== 'ready') throw new Error('invalid authorization fixture')
    render(<AuthorizationScreen model={{
      ...ready,
      request: {
        ...ready.request,
        tools: [{
          name: 'project.read', version: '1.0.0', summary: 'Read project', lifecycle: 'active', risk: 'read', grantScope: 'project:read',
        }],
      },
    }} onReturn={vi.fn()} />)
    expect(screen.queryByText(copy.mcpCli.authorization.mutationWarning)).toBeNull()
  })

  it('shows a permanent authorization rule without fake sentinel dates', () => {
    if (ready.status !== 'ready') throw new Error('invalid authorization fixture')
    render(<AuthorizationScreen model={{
      ...ready,
      request: { ...ready.request, mode: 'permanent', effectiveFrom: null, expiresAt: null },
    }} onReturn={vi.fn()} />)
    expect(screen.getByText(copy.mcpCli.tools.rule.permanent)).toBeTruthy()
    expect(screen.queryByText(copy.mcpCli.authorization.effectiveFrom)).toBeNull()
    expect(screen.queryByText(copy.mcpCli.authorization.expiresAt)).toBeNull()
  })

  it('keeps the request visible but hides decisions without the decide action', () => {
    if (ready.status !== 'ready') throw new Error('invalid authorization fixture')
    render(<AuthorizationScreen model={{ ...ready, canDecide: false }} onReturn={vi.fn()} />)

    expect(screen.getByText('Codex')).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.mcpCli.authorization.authorize })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.mcpCli.authorization.deny })).toBeNull()
  })

  it('keeps failed authorization requests visible and retryable', async () => {
    const retry = vi.fn()
    render(<AuthorizationScreen model={{ status: 'error', retry }} onReturn={vi.fn()} />)
    expect(screen.getByRole('alert').textContent).toContain(copy.mcpCli.authorization.loadFailedTitle)
    fireEvent.click(screen.getByRole('button', { name: copy.mcpCli.authorization.retry }))
    await waitFor(() => expect(retry).toHaveBeenCalledTimes(1))
  })
})

describe('PM CLI device authorization screen', () => {
  const deviceRequest = {
    user_code: 'ABCD-EFGH',
    status: 'pending',
    client: { id: 'pm-cli', name: 'PM CLI' },
    channel: 'cli',
    workspace: { id: 7, name: 'JiGuang' },
    mode: 'window',
    tools: [{
      name: 'project.read',
      version: '1.0.0',
      summary: 'Read project',
      lifecycle: 'active',
      risk: 'read',
      grant_scope: 'project:read',
    }],
    effective_from: '2026-08-27T01:00:00Z',
    expires_at: '2026-09-03T01:00:00Z',
    request_expires_at: '2026-08-27T01:10:00Z',
  }

  it.each([
    ['authorize', 'approved', copy.mcpCli.deviceAuthorization.status.approved.title],
    ['deny', 'denied', copy.mcpCli.deviceAuthorization.status.denied.title],
  ] as const)('submits %s and stays on the PM completion screen', async (
    decision,
    status,
    completionTitle,
  ) => {
    const requests: ConfigurationRequest[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push({ method: config.method, url: config.url, data: config.data })
      const data = config.method === 'get'
        ? deviceRequest
        : { user_code: 'ABCD-EFGH', status }
      return { data: { data }, status: 200, statusText: 'OK', headers: {}, config }
    }
    renderDeviceAuthorizationHook()

    expect(await screen.findByText('PM CLI')).toBeTruthy()
    expect(screen.getByText('JiGuang (#7)')).toBeTruthy()
    expect(screen.getByText('ABCD-EFGH')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {
      name: decision === 'authorize'
        ? copy.mcpCli.authorization.authorize
        : copy.mcpCli.authorization.deny,
    }))

    expect(await screen.findByText(completionTitle)).toBeTruthy()
    const decisionRequest = requests.find((request) => request.method === 'post')
    expect(decisionRequest?.url).toBe('/external-tooling/device-authorizations')
    expect(JSON.parse(String(decisionRequest?.data))).toEqual({
      user_code: 'ABCD-EFGH',
      decision,
    })
  })
})
