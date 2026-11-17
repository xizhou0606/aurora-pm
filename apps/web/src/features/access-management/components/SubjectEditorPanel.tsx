import { useEffect, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, Checkbox, Input, Label, ResponsivePanel, Textarea } from '@/shared/ui'
import { accessSubjectPolicy, type AccessSubject, type AccessSubjectKind, type SubjectDraft } from '../model'

type SubjectEditorPanelProps = {
  kind: AccessSubjectKind
  subject: AccessSubject | null
  open: boolean
  pending: boolean
  error: boolean
  onClose: () => void
  onSubmit: (draft: SubjectDraft) => void
}

export function SubjectEditorPanel({ kind, subject, open, pending, error, onClose, onSubmit }: SubjectEditorPanelProps) {
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [invalid, setInvalid] = useState(false)
  const policy = subject ? accessSubjectPolicy(kind, subject) : null

  useEffect(() => {
    if (!open) return
    setCode(subject?.code ?? '')
    setName(subject?.name ?? '')
    setDescription(subject?.description ?? '')
    setEnabled(subject?.enabled ?? true)
    setInvalid(false)
  }, [open, subject])

  const submit = () => {
    if (policy && !policy.canEditMetadata) return
    if (!name.trim() || (kind === 'group' && !code.trim())) {
      setInvalid(true)
      return
    }
    onSubmit({
      kind,
      ...(subject ? { id: subject.id } : {}),
      code: kind === 'role' ? name.trim() : code.trim(),
      name: name.trim(),
      description: description.trim(),
      enabled,
    })
  }
  const title = subject
    ? kind === 'role' ? COPY.subjects.editRole : COPY.subjects.editGroup
    : kind === 'role' ? COPY.subjects.createRole : COPY.subjects.createGroup

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={title}
      closeLabel={COPY.common.cancel}
      pending={pending}
      className="max-w-xl"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>{COPY.common.cancel}</Button>
          <Button type="button" disabled={pending || (policy != null && !policy.canEditMetadata)} onClick={submit}>{pending ? COPY.common.saving : COPY.common.save}</Button>
        </>
      )}
    >
      <div className="grid gap-4 p-5">
        {kind === 'group' ? (
          <div className="grid gap-2">
            <Label htmlFor="access-subject-code">{COPY.subjects.codeLabel}</Label>
            <Input
              id="access-subject-code"
              value={code}
              disabled={Boolean(subject)}
              placeholder={COPY.subjects.codePlaceholder}
              onChange={(event) => { setCode(event.target.value); setInvalid(false) }}
            />
          </div>
        ) : null}
        <div className="grid gap-2">
          <Label htmlFor="access-subject-name">{COPY.subjects.nameLabel}</Label>
          <Input
            id="access-subject-name"
            value={name}
            disabled={policy != null && !policy.canEditMetadata}
            placeholder={COPY.subjects.namePlaceholder}
            onChange={(event) => { setName(event.target.value); setInvalid(false) }}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="access-subject-description">{COPY.subjects.descriptionLabel}</Label>
          <Textarea
            id="access-subject-description"
            value={description}
            disabled={policy != null && !policy.canEditMetadata}
            placeholder={COPY.subjects.descriptionPlaceholder}
            onChange={(event) => setDescription(event.target.value)}
          />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={enabled}
            disabled={policy != null && !policy.canEditEnabled}
            onCheckedChange={(checked) => setEnabled(checked === true)}
            aria-label={COPY.subjects.enabledLabel}
          />
          <span>{COPY.subjects.enabledLabel}</span>
        </label>
        {policy?.protected ? (
          <p className="text-xs text-muted-foreground">
            {kind === 'group' ? COPY.subjects.defaultGroupProtected : COPY.subjects.systemSubjectProtected}
          </p>
        ) : null}
        {invalid ? <p role="alert" className="text-sm text-destructive">{kind === 'role' ? COPY.subjects.roleNameRequired : COPY.subjects.required}</p> : null}
        {error ? <p role="alert" className="text-sm text-destructive">{kind === 'role' ? COPY.subjects.roleSaveError : COPY.subjects.saveError}</p> : null}
      </div>
    </ResponsivePanel>
  )
}
