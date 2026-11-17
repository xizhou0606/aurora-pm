import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import {
  AlertBanner,
  Button,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'

import {
  resourceAccessChangeImpact,
  type ResourceAccessEditorIntent,
  type ResourceAccessEditorState,
  type ResourceAccessMode,
  type ResourceAccessSubjectControl,
} from '../model'
import { AccessLevelSelect } from './AccessLevelSelect'
import { SubjectRuleEditor } from './SubjectRuleEditor'

const modeCopy = copy.mode

export function PolicyForm({
  state,
  subjects,
  error,
  onEdit,
  onReload,
}: {
  state: ResourceAccessEditorState
  subjects: ResourceAccessSubjectControl
  error?: boolean
  onEdit: (action: ResourceAccessEditorIntent) => void
  onReload: () => void
}) {
  const readOnly = !state.base.canManageAccess || state.base.mode === 'legacy'
  const pending = state.status === 'submitting'
  const impact = resourceAccessChangeImpact(state.base, state.draft)
  return (
    <div className="grid gap-5">
      {state.base.mode === 'legacy' ? (
        <AlertBanner tone="warning">{modeCopy.legacyNotice}</AlertBanner>
      ) : !state.base.canManageAccess ? (
        <AlertBanner tone="info">{modeCopy.readOnlyNotice}</AlertBanner>
      ) : null}
      {state.status === 'conflict' ? (
        <AlertBanner tone="warning" action={(
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onEdit({ type: 'rebase' })}
          >
            {copy.change.rebase}
          </Button>
        )}>{copy.change.conflict}</AlertBanner>
      ) : null}
      {state.status === 'unknown_result' ? (
        <AlertBanner tone="error" action={(
          <Button type="button" variant="ghost" size="sm" onClick={onReload}>{copy.panel.retry}</Button>
        )}>{copy.change.unknownResult}</AlertBanner>
      ) : null}
      {error ? <AlertBanner tone="error">{copy.panel.errorReason}</AlertBanner> : null}
      {impact !== 'unchanged' && state.status !== 'conflict' && state.status !== 'unknown_result' ? (
        <AlertBanner tone={impact === 'expanded' ? 'warning' : 'info'}>
          {copy.change[impact]}
        </AlertBanner>
      ) : null}

      <p className="m-0 text-sm text-muted-foreground">
        {state.base.effectiveLevel
          ? copy.effective.summary(copy.level[state.base.effectiveLevel], state.base.effectiveSources.length)
          : copy.effective.none}
      </p>

      <div className="grid gap-2">
        <Label htmlFor="resource-access-mode">{modeCopy.label}</Label>
        <Select
          value={state.draft.mode === 'link' ? 'private' : state.draft.mode}
          disabled={readOnly || pending}
          onValueChange={(value) => {
            if (value === 'private' || value === 'restricted' || value === 'workspace') {
              onEdit({ type: 'set_mode', mode: value })
            }
          }}
        >
          <SelectTrigger id="resource-access-mode"><SelectValue>{modeCopy[state.draft.mode === 'link' ? 'private' : state.draft.mode]}</SelectValue></SelectTrigger>
          <SelectContent>
            <SelectItem value="private">{modeCopy.private}</SelectItem>
            <SelectItem value="restricted">{modeCopy.restricted}</SelectItem>
            <SelectItem value="workspace">{modeCopy.workspace}</SelectItem>
            {state.base.mode === 'legacy' ? (
              <SelectItem value="legacy" disabled>{modeCopy.legacy}</SelectItem>
            ) : null}
          </SelectContent>
        </Select>
      </div>

      {state.draft.mode === 'workspace' ? (
        <AccessLevelSelect
          id="resource-access-workspace-level"
          value={state.draft.workspaceLevel}
          disabled={readOnly || pending}
          onChange={(level) => onEdit({ type: 'set_workspace_level', level })}
        />
      ) : null}

      {state.draft.mode === 'restricted' ? (
        <SubjectRuleEditor
          rules={state.draft.rules}
          control={subjects}
          disabled={readOnly || pending}
          onEdit={onEdit}
        />
      ) : null}
    </div>
  )
}
