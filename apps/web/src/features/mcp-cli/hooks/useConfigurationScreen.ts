import { useAuthorizationRecords } from './useAuthorizationRecords'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { runtimeWorkspaceStore } from '@/shared/runtime'

import {
  fetchEffectiveToolingAccess,
  fetchExternalToolingConfiguration,
  fetchToolCatalog,
  updateExternalToolingConfiguration,
  type ExternalToolingConfigurationInput,
} from '../api/api'
import { DEFAULT_CLI_GRANT_TTL_HOURS, type ExternalToolingEffectMode } from '../model/contract'
import {
  buildCliAuthConfiguration,
  buildCliCommands,
  buildMcpConfiguration,
  CONNECTIVITY_TOOL_NAME,
  type ConfigurationScreenModel,
  type ConfigurationAccess,
  type CopyState,
  type ConfigurationCopyTarget,
  type GrantDraftValidation,
  type McpClientKind,
  isToolConfigurable,
} from '../model/model'
import {
  collectionState,
  configurationInput,
  configurationValidation,
  effectiveAccessQueryKey,
  formFromConfiguration,
  sameConfiguration,
  sameToolNames,
  writeClipboard,
  type ConfigurationForm,
} from './helpers'

const effectiveAccessRefreshIntervalMs = 10_000
const configurationRefreshIntervalMs = 10_000

export function useConfigurationScreen(
  access: ConfigurationAccess,
): ConfigurationScreenModel {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryWorkspaceId = workspaceId > 0 ? workspaceId : 1
  const queryClient = useQueryClient()
  const records = useAuthorizationRecords(access.viewGrants, access.revokeGrants === true)
  const [copyState, setCopyState] = useState<CopyState>({ target: null, status: 'idle' })
  const [mode, setMode] = useState<ExternalToolingEffectMode>('disabled')
  const [toolScopes, setToolScopes] = useState<string[]>([])
  const [serverBaseline, setServerBaseline] = useState<ConfigurationForm | null>(null)
  const [saveOutcome, setSaveOutcome] = useState<'idle' | 'saved' | 'error'>('idle')
  const [hydratedWorkspaceId, setHydratedWorkspaceId] = useState<number | null>(null)
  const hydratedWorkspaceRef = useRef<number | null>(null)
  const hydratedConfigurationVersionRef = useRef<number | null>(null)
  const [effectiveFrom, setEffectiveFrom] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [cliGrantTtlHours, setCliGrantTtlHours] = useState(DEFAULT_CLI_GRANT_TTL_HOURS)
  const [mcpClient, setMcpClient] = useState<McpClientKind>('generic')
  const [validation, setValidation] = useState<GrantDraftValidation>('none')
  const managementMode = access.updateConfiguration
  const effectiveAccess = useQuery({
    queryKey: effectiveAccessQueryKey(queryWorkspaceId),
    queryFn: ({ signal }) => fetchEffectiveToolingAccess(signal),
    enabled: workspaceId > 0 && !managementMode,
    retry: false,
    refetchInterval: effectiveAccessRefreshIntervalMs,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: 'always',
  })
  const catalog = useQuery({
    queryKey: pmKeys.resource(queryWorkspaceId, 'workspace', 'externalTooling', 'catalog'),
    queryFn: ({ signal }) => fetchToolCatalog(signal),
    enabled: workspaceId > 0 && managementMode && access.viewCatalog,
    retry: false,
    refetchInterval: configurationRefreshIntervalMs,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: 'always',
  })
  const configurationQueryKey = pmKeys.resource(
    queryWorkspaceId,
    'workspace',
    'externalTooling',
    'configuration',
  )
  const configuration = useQuery({
    queryKey: configurationQueryKey,
    queryFn: ({ signal }) => fetchExternalToolingConfiguration(signal),
    enabled: workspaceId > 0 && managementMode && access.viewConfiguration,
    retry: false,
    refetchInterval: configurationRefreshIntervalMs,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: 'always',
  })
  const saveConfiguration = useMutation({
    mutationFn: (variables: { workspaceId: number; input: ExternalToolingConfigurationInput }) => (
      updateExternalToolingConfiguration(variables.input)
    ),
    onSuccess: (saved, variables) => {
      queryClient.setQueryData(pmKeys.resource(variables.workspaceId, 'workspace', 'externalTooling', 'configuration'), saved)
      hydratedConfigurationVersionRef.current = saved.version
      void queryClient.invalidateQueries({ queryKey: effectiveAccessQueryKey(variables.workspaceId) })
      if (runtimeWorkspaceStore.getState().currentWorkspaceId !== variables.workspaceId) return
      const next = formFromConfiguration(saved)
      setMode(next.mode)
      setToolScopes(next.toolScopes)
      setEffectiveFrom(next.effectiveFrom)
      setExpiresAt(next.expiresAt)
      setCliGrantTtlHours(next.cliGrantTtlHours)
      setServerBaseline(next)
      setSaveOutcome('saved')
      setValidation('none')
    },
    onError: (_error, variables) => {
      if (runtimeWorkspaceStore.getState().currentWorkspaceId === variables.workspaceId) {
        setSaveOutcome('error')
      }
    },
  })
  useEffect(() => {
    hydratedWorkspaceRef.current = null
    hydratedConfigurationVersionRef.current = null
    setToolScopes([])
    setMode('disabled')
    setEffectiveFrom('')
    setExpiresAt('')
    setCliGrantTtlHours(DEFAULT_CLI_GRANT_TTL_HOURS)
    setServerBaseline(null)
    setSaveOutcome('idle')
    setHydratedWorkspaceId(null)
    setValidation('none')
  }, [workspaceId])
  useEffect(() => {
    if (!catalog.data || !configuration.data || workspaceId <= 0) return
    const available = new Set(catalog.data.filter(isToolConfigurable).map((tool) => tool.name))
    const currentForm: ConfigurationForm = { mode, toolScopes, effectiveFrom, expiresAt, cliGrantTtlHours }
    const hasUnsavedChanges = serverBaseline !== null && !sameConfiguration(currentForm, serverBaseline)
    const workspaceChanged = hydratedWorkspaceRef.current !== workspaceId
    const configurationChanged = hydratedConfigurationVersionRef.current !== configuration.data.version
    if (workspaceChanged || (configurationChanged && !hasUnsavedChanges)) {
      const baseline = formFromConfiguration(configuration.data)
      const restored = baseline.toolScopes.filter((name) => available.has(name))
      const hydratedBaseline = { ...baseline, toolScopes: restored }
      hydratedWorkspaceRef.current = workspaceId
      hydratedConfigurationVersionRef.current = configuration.data.version
      setToolScopes(restored)
      setMode(baseline.mode)
      setEffectiveFrom(baseline.effectiveFrom)
      setExpiresAt(baseline.expiresAt)
      setCliGrantTtlHours(baseline.cliGrantTtlHours)
      setServerBaseline(hydratedBaseline)
      setSaveOutcome('idle')
      setHydratedWorkspaceId(workspaceId)
      return
    }
    setToolScopes((current) => {
      const filtered = current.filter((name) => available.has(name))
      return sameToolNames(current, filtered) ? current : filtered
    })
  }, [catalog.data, cliGrantTtlHours, configuration.data, effectiveFrom, expiresAt, mode, serverBaseline, toolScopes, workspaceId])
  const mcpConfiguration = useMemo(() => buildMcpConfiguration(mcpClient), [mcpClient])
  const cliCommands = useMemo(() => buildCliCommands(workspaceId), [workspaceId])
  const cliAuthConfiguration = useMemo(() => buildCliAuthConfiguration(workspaceId), [workspaceId])
  const copyConfiguration = useCallback((target: ConfigurationCopyTarget) => {
    const artifacts: Record<ConfigurationCopyTarget, string> = {
      mcp_configuration: mcpConfiguration,
      mcp_prompt: copy.mcpCli.agentPrompt.mcp(mcpConfiguration, CONNECTIVITY_TOOL_NAME),
      cli_prompt: copy.mcpCli.agentPrompt.cli(cliAuthConfiguration),
      cli_command_0: cliCommands[0] ?? '',
      cli_command_1: cliCommands[1] ?? '',
      cli_command_2: cliCommands[2] ?? '',
      cli_command_3: cliCommands[3] ?? '',
      cli_command_4: cliCommands[4] ?? '',
    }
    void writeClipboard(artifacts[target]).then(
      () => setCopyState({ target, status: 'copied' }),
      () => setCopyState({ target, status: 'error' }),
    )
  }, [cliAuthConfiguration, cliCommands, mcpConfiguration])
  const form = useMemo<ConfigurationForm>(
    () => ({ mode, toolScopes, effectiveFrom, expiresAt, cliGrantTtlHours }),
    [cliGrantTtlHours, effectiveFrom, expiresAt, mode, toolScopes],
  )
  const configurationDirty = useMemo(
    () => serverBaseline !== null && !sameConfiguration(form, serverBaseline),
    [form, serverBaseline],
  )
  const resetValidation = useCallback(() => { setValidation('none'); setSaveOutcome('idle') }, [])
  const setToolsSelected = useCallback((toolNames: readonly string[], selected: boolean) => {
    const available = new Set(catalog.data?.filter(isToolConfigurable).map((tool) => tool.name) ?? [])
    const names = [...new Set(toolNames.filter((name) => available.has(name)))]
    if (names.length === 0) return
    resetValidation()
    setToolScopes((current) => {
      const next = new Set(current)
      names.forEach((name) => selected ? next.add(name) : next.delete(name))
      return [...next]
    })
  }, [catalog.data, resetValidation])
  const setEffectMode = useCallback((value: ExternalToolingEffectMode) => { resetValidation(); setMode(value) }, [resetValidation])
  const setEffectiveDate = useCallback((value: string) => { resetValidation(); setEffectiveFrom(value) }, [resetValidation])
  const setExpiry = useCallback((value: string) => { resetValidation(); setExpiresAt(value) }, [resetValidation])
  const setCliGrantTTL = useCallback((value: number) => { resetValidation(); setCliGrantTtlHours(value) }, [resetValidation])
  const save = useCallback(() => {
    if (
      !configurationDirty
      || workspaceId <= 0
      || !catalog.data
      || !access.updateConfiguration
      || saveConfiguration.isPending
    ) return
    const nextValidation = configurationValidation(form)
    if (nextValidation !== 'none') {
      setValidation(nextValidation)
      return
    }
    setValidation('none')
    saveConfiguration.mutate({ workspaceId, input: configurationInput(form) })
  }, [
    access.updateConfiguration,
    catalog.data,
    configurationDirty,
    form,
    saveConfiguration,
    workspaceId,
  ])
  const canEditConfiguration = access.updateConfiguration && hydratedWorkspaceId === workspaceId && Boolean(catalog.data && configuration.data)
  return {
    viewMode: managementMode ? 'manage' : 'effective',
    effectiveAccess: managementMode
      ? null
      : effectiveAccess.isPending
        ? { status: 'loading' }
        : effectiveAccess.isError || !effectiveAccess.data
          ? { status: 'error', retry: () => { void effectiveAccess.refetch() } }
          : { status: 'ready', data: effectiveAccess.data },
    catalog: managementMode
      ? collectionState({
          data: catalog.data,
          allowed: access.viewCatalog && access.viewConfiguration,
          pending: catalog.isPending
            || (access.viewConfiguration && configuration.isPending)
            || (Boolean(catalog.data && configuration.data) && hydratedWorkspaceId !== workspaceId),
          error: catalog.isError || configuration.isError,
          retry: () => {
            void catalog.refetch()
            void configuration.refetch()
          },
        })
      : collectionState({
          data: effectiveAccess.data?.tools,
          allowed: true,
          pending: effectiveAccess.isPending,
          error: effectiveAccess.isError,
          retry: () => { void effectiveAccess.refetch() },
        }),
    canViewRecords: access.viewGrants,
    records,
    mcpConfiguration,
    mcpClient: {
      value: mcpClient,
      setValue: setMcpClient,
    },
    cliCommands,
    copyState,
    grantDraft: {
      mode,
      toolScopes,
      effectiveFrom,
      expiresAt,
      cliGrantTtlHours,
      validation,
      setMode: setEffectMode,
      setToolsSelected,
      setEffectiveFrom: setEffectiveDate,
      setExpiresAt: setExpiry,
      setCliGrantTtlHours: setCliGrantTTL,
    },
    toolConfiguration: {
      status: saveConfiguration.isPending
        ? 'saving'
        : saveOutcome === 'error'
        ? 'error'
        : configurationDirty
          ? 'dirty'
          : saveOutcome,
      canSave: access.updateConfiguration
        && Boolean(catalog.data)
        && hydratedWorkspaceId === workspaceId
        && configurationDirty
        && !saveConfiguration.isPending,
      effectiveMode: managementMode
        ? serverBaseline?.mode ?? 'disabled'
        : effectiveAccess.data?.mode ?? 'disabled',
      effectiveToolCount: managementMode
        ? serverBaseline?.toolScopes.length ?? 0
        : effectiveAccess.data?.tools.length ?? 0,
      catalogToolCount: managementMode
        ? catalog.data?.length ?? 0
        : effectiveAccess.data?.tools.length ?? 0,
      save,
    },
    canEditConfiguration,
    copyConfiguration,
  }
}
