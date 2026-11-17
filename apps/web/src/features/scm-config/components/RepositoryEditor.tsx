import { useState } from 'react'

import {
  Button,
  Checkbox,
  EmptyState,
  Input,
  Label,
  ResponsivePanel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import { SCM_CONFIG_COPY as COPY } from '@/shared/copy'

import type { ScmAccessibleRepoDTO, ScmConnectionDTO, ScmRepositoryDTO, ScmRepositoryInput } from '../api/contracts'
import {
  useAccessibleReposQuery,
  useCreateRepositoryMutation,
  useUpdateRepositoryMutation,
} from '../hooks'

type RepositoryFormState = {
  connection_id: number
  external_id: string
  path: string
  default_branch: string
  project_set_id: string
  enabled: boolean
}

const EMPTY_FORM: RepositoryFormState = {
  connection_id: 0,
  external_id: '',
  path: '',
  default_branch: '',
  project_set_id: '',
  enabled: true,
}

type RepositoryEditorProps = {
  open: boolean
  editing: ScmRepositoryDTO | null
  connections: ScmConnectionDTO[]
  onOpenChange: (open: boolean) => void
  onError: (message: string) => void
}

// 单个仓库绑定的新建/编辑面板。多选批量接入走 RepositoryScanPicker，
// 这里只负责微调单个仓库的分支、项目集与启停。父组件用 key 携带
// open/editing 重挂载本组件，表单因此总在打开时拿到最新初值。
export function RepositoryEditor({ open, editing, connections, onOpenChange, onError }: RepositoryEditorProps) {
  const [form, setForm] = useState<RepositoryFormState>(() => editing ? {
    connection_id: editing.connection_id,
    external_id: editing.external_id ?? '',
    path: editing.path,
    default_branch: editing.default_branch ?? '',
    project_set_id: editing.project_set_id ? String(editing.project_set_id) : '',
    enabled: editing.enabled,
  } : { ...EMPTY_FORM, connection_id: connections[0]?.id ?? 0 })
  const [pickerOpen, setPickerOpen] = useState(false)

  const accessibleReposQuery = useAccessibleReposQuery(form.connection_id, open && pickerOpen)
  const createMutation = useCreateRepositoryMutation(onError)
  const updateMutation = useUpdateRepositoryMutation(onError)

  const connectionItems = [
    { value: '0', label: COPY.common.placeholderEmpty },
    ...connections.map((connection) => ({ value: String(connection.id), label: connection.name })),
  ]

  const submit = () => {
    const body: ScmRepositoryInput = {
      connection_id: form.connection_id,
      external_id: form.external_id,
      path: form.path,
      default_branch: form.default_branch,
      project_set_id: form.project_set_id ? Number(form.project_set_id) : null,
      enabled: form.enabled,
    }
    const settle = () => onOpenChange(false)
    if (editing) {
      updateMutation.mutate({ id: editing.id, body }, { onSuccess: settle })
    } else {
      createMutation.mutate(body, { onSuccess: settle })
    }
  }
  const applyPickedRepo = (repo: ScmAccessibleRepoDTO) => {
    setForm((prev) => ({
      ...prev,
      external_id: repo.external_id,
      path: repo.path,
      default_branch: repo.default_branch || prev.default_branch,
    }))
    setPickerOpen(false)
  }

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? COPY.repositories.edit : COPY.repositories.create}
      closeLabel={COPY.common.close}
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {COPY.common.cancel}
          </Button>
          <Button
            onClick={submit}
            disabled={createMutation.isPending || updateMutation.isPending || form.path === '' || form.connection_id === 0}
          >
            {COPY.common.save}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <label className="grid gap-1.5">
          <Label htmlFor="scm-repo-connection">{COPY.repositories.fields.connection}</Label>
          <Select
            items={connectionItems}
            value={String(form.connection_id)}
            onValueChange={(value) => setForm((prev) => ({ ...prev, connection_id: Number(value) }))}
          >
            <SelectTrigger id="scm-repo-connection" name="scm-repo-connection" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {connectionItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        {form.connection_id > 0 ? (
          <div className="grid gap-1.5">
            <Button variant="outline" size="sm" className="justify-self-start" onClick={() => setPickerOpen(true)}>
              {COPY.connections.pickRepos}
            </Button>
            {pickerOpen && accessibleReposQuery.isSuccess ? (
              <ul className="max-h-40 divide-y divide-border overflow-auto rounded-lg border border-border bg-popover p-1 text-sm text-popover-foreground shadow-[var(--design-shadow-popover)]">
                {accessibleReposQuery.data.repos.length === 0 ? (
                  <li><EmptyState title={COPY.common.empty} /></li>
                ) : null}
                {(accessibleReposQuery.data?.repos ?? []).map((repo) => (
                  <li key={repo.external_id}>
                    <Button
                      type="button"
                      variant="ghost"
                      className="h-auto w-full justify-start whitespace-normal break-all px-3 py-1.5 text-left font-mono text-xs"
                      onClick={() => applyPickedRepo(repo)}
                    >
                      {repo.path}
                    </Button>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : null}
        <label className="grid gap-1.5">
          <Label htmlFor="scm-repo-path">{COPY.repositories.fields.path}</Label>
          <Input
            id="scm-repo-path"
            value={form.path}
            onChange={(event) => setForm((prev) => ({ ...prev, path: event.target.value }))}
          />
        </label>
        <label className="grid gap-1.5">
          <Label htmlFor="scm-repo-branch">{COPY.repositories.fields.branch}</Label>
          <Input
            id="scm-repo-branch"
            value={form.default_branch}
            onChange={(event) => setForm((prev) => ({ ...prev, default_branch: event.target.value }))}
          />
        </label>
        <label className="grid gap-1.5">
          <Label htmlFor="scm-repo-project-set">{COPY.repositories.fields.projectSet}</Label>
          <Input
            id="scm-repo-project-set"
            inputMode="numeric"
            value={form.project_set_id}
            onChange={(event) => setForm((prev) => ({ ...prev, project_set_id: event.target.value }))}
          />
        </label>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={form.enabled}
            onCheckedChange={(checked) => setForm((prev) => ({ ...prev, enabled: checked === true }))}
          />
          {COPY.common.enabled}
        </label>
      </div>
    </ResponsivePanel>
  )
}
