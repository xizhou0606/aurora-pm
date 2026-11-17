import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'

import { useWorkspaceStore } from '@/shared/state'
import { getPMRuntimeConfig, safePMRedirect } from '@/shared/config'
import { copy } from '@/shared/copy'
import {
  Button,
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxInputGroup,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
  EmptyState,
  Label,
  LoadingSkeleton,
} from '@/shared/ui'
import { useWorkspacesQuery } from '../api/query'
import { normalizeWorkspaceOptions } from '../hooks/selection'
import { useWorkspaceSwitcher } from '../utils/workflows'

type WorkspaceComboboxItem = {
  label: string
  value: number
}

export function WorkspaceSelectPage() {
  const config = useMemo(() => getPMRuntimeConfig(), [])
  const navigate = useNavigate()
  const search = useSearch({ from: '/select-workspace' })
  const currentWorkspaceId = useWorkspaceStore(
    (state) => state.currentWorkspaceId,
  )
  const workspaces = useWorkspacesQuery()
  const options = useMemo(
    () => normalizeWorkspaceOptions(workspaces.data ?? []),
    [workspaces.data],
  )
  const comboboxItems = useMemo<WorkspaceComboboxItem[]>(
    () => options.map((workspace) => ({
      label: workspace.name,
      value: workspace.id,
    })),
    [options],
  )
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<number | null>(
    currentWorkspaceId > 0 ? currentWorkspaceId : null,
  )

  useEffect(() => {
    const previousTitle = document.title
    document.title = copy.workspace.documentTitle(config.productName)
    return () => {
      document.title = previousTitle
    }
  }, [config.productName])

  useEffect(() => {
    if (!workspaces.data) return
    if (
      selectedWorkspaceId &&
      options.some((workspace) => workspace.id === selectedWorkspaceId)
    ) {
      return
    }
    setSelectedWorkspaceId(
      options.some((workspace) => workspace.id === currentWorkspaceId)
        ? currentWorkspaceId
        : (options[0]?.id ?? null),
    )
  }, [currentWorkspaceId, options, selectedWorkspaceId, workspaces.data])

  const navigateToDestination = useCallback(
    () =>
      navigate({
        to: safePMRedirect(search.redirect),
        replace: true,
      } as never),
    [navigate, search.redirect],
  )
  const switcher = useWorkspaceSwitcher({
    navigate: navigateToDestination,
  })

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (selectedWorkspaceId) {
      await switcher.switchWorkspace(selectedWorkspaceId)
    }
  }

  const loading = workspaces.isPending
  const failed = workspaces.isError
  const empty = workspaces.isSuccess && options.length === 0
  const ready = workspaces.isSuccess && options.length > 0
  const selectedComboboxItem = comboboxItems.find(
    (item) => item.value === selectedWorkspaceId,
  ) ?? null

  return (
    <main className="grid min-h-dvh place-items-center bg-muted p-6">
      <section className="w-full max-w-[440px] rounded-lg border border-border bg-card p-7 text-card-foreground shadow-[var(--design-shadow-elevated)]">
        <div className="mb-5">
          <h1 className="text-[22px] font-semibold tracking-tight">
            {copy.workspace.selectorTitle}
          </h1>
          <p className="mt-1.75 text-sm leading-5 text-muted-foreground">
            {copy.workspace.selectorDescription}
          </p>
        </div>

        {loading ? (
          <LoadingSkeleton
            rows={3}
            label={copy.ui.loading.label}
            aria-live="polite"
          />
        ) : null}

        {failed ? (
          <EmptyState
            role="alert"
            aria-live="assertive"
            tabIndex={-1}
            title={copy.errors.genericTitle}
            action={
              <Button type="button" variant="outline" onClick={() => workspaces.refetch()}>
                {copy.workspace.retry}
              </Button>
            }
          />
        ) : null}

        {empty ? (
          <EmptyState
            role="status"
            aria-live="polite"
            tabIndex={-1}
            title={copy.ui.empty.title}
          />
        ) : null}

        {ready ? (
          <form className="grid gap-4" onSubmit={submit}>
            <div className="grid gap-1.5">
              <Label htmlFor="pm-workspace-combobox">
                {copy.workspace.selectorLabel}
              </Label>
              <Combobox<WorkspaceComboboxItem>
                items={comboboxItems}
                value={selectedComboboxItem}
                disabled={switcher.pending}
                onValueChange={(value) => {
                  setSelectedWorkspaceId(
                    value && value.value > 0 ? value.value : null,
                  )
                }}
              >
                <ComboboxInputGroup>
                  <ComboboxInput
                    id="pm-workspace-combobox"
                    name="workspace"
                    autoComplete="off"
                    placeholder={copy.workspace.selectorPlaceholder}
                    aria-describedby={
                      switcher.errorMessage ? 'pm-workspace-switch-error' : undefined
                    }
                  />
                  <ComboboxTrigger aria-label={copy.workspace.openOptions} />
                </ComboboxInputGroup>
                <ComboboxContent>
                  <ComboboxEmpty>{copy.workspace.noMatches}</ComboboxEmpty>
                  <ComboboxList>
                    {(item: WorkspaceComboboxItem) => (
                      <ComboboxItem key={item.value} value={item}>
                        {item.label}
                      </ComboboxItem>
                    )}
                  </ComboboxList>
                </ComboboxContent>
              </Combobox>
            </div>

            {switcher.errorMessage ? (
              <p
                id="pm-workspace-switch-error"
                role="alert"
                aria-live="assertive"
                className="text-sm leading-5 text-destructive"
              >
                {switcher.errorMessage}
              </p>
            ) : null}

            <Button
              type="submit"
              size="lg"
              aria-busy={switcher.pending || undefined}
              disabled={!selectedWorkspaceId || switcher.pending}
              className="w-full"
            >
              {switcher.pending
                ? copy.workspace.switching
                : copy.workspace.continueToPM}
            </Button>
          </form>
        ) : null}
      </section>
    </main>
  )
}
