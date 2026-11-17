import { useEffect, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
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
} from '@/shared/ui'
import type { AccessGroup, AccessRole, ManagedUser } from '../model'

type UserAccessPanelProps = {
  user: ManagedUser | null
  groups: AccessGroup[]
  roles: AccessRole[]
  pending: boolean
  error: boolean
  onClose: () => void
  onSubmit: (input: { userId: number; groupId: number; roleIds: number[] }) => void
}

export function UserAccessPanel({ user, groups, roles, pending, error, onClose, onSubmit }: UserAccessPanelProps) {
  const [groupId, setGroupId] = useState<number | null>(null)
  const [roleIds, setRoleIds] = useState<number[]>([])
  const enabledGroups = groups.filter((group) => group.enabled)
  const groupItems = enabledGroups.map((group) => ({ value: group.id, label: group.name }))
  useEffect(() => {
    if (!user) return
    setGroupId(user.group?.id ?? groups.find((group) => group.isDefault)?.id ?? null)
    setRoleIds(user.roles.map((role) => role.id))
  }, [groups, user])

  const toggleRole = (roleId: number, checked: boolean) => {
    setRoleIds((current) => checked
      ? [...new Set([...current, roleId])]
      : current.filter((id) => id !== roleId))
  }
  const submit = () => {
    if (!user || !groupId) return
    onSubmit({ userId: user.id, groupId, roleIds })
  }
  const name = user?.name || user?.username || COPY.common.unnamed

  return (
    <ResponsivePanel
      open={Boolean(user)}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={COPY.people.accessTitle(name)}
      description={COPY.people.accessDescription}
      closeLabel={COPY.common.cancel}
      pending={pending}
      className="max-w-xl"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>{COPY.common.cancel}</Button>
          <Button type="button" disabled={pending || !groupId} onClick={submit}>
            {pending ? COPY.common.saving : COPY.common.save}
          </Button>
        </>
      )}
    >
      <div className="grid gap-5 p-5">
        <div className="grid gap-2">
          <Label>{COPY.people.groupLabel}</Label>
          <Select<number>
            items={groupItems}
            value={groupId}
            onValueChange={(value) => setGroupId(Number(value) || null)}
          >
            <SelectTrigger aria-label={COPY.people.groupLabel}>
              <SelectValue placeholder={COPY.people.groupPlaceholder} />
            </SelectTrigger>
            <SelectContent>
              {enabledGroups.map((group) => (
                <SelectItem key={group.id} value={group.id}>{group.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <fieldset className="grid gap-2">
          <legend className="text-sm font-medium text-foreground">{COPY.people.rolesLabel}</legend>
          {roles.filter((role) => role.enabled).map((role) => (
            <div key={role.id} className="flex items-start gap-2 rounded-md border border-border px-3 py-2 text-sm">
              <Checkbox
                checked={roleIds.includes(role.id)}
                onCheckedChange={(checked) => toggleRole(role.id, checked === true)}
                aria-label={role.name}
              />
              <span className="grid min-w-0 gap-0.5">
                <span>{role.name}</span>
                {role.description ? <span className="text-xs text-muted-foreground">{role.description}</span> : null}
              </span>
            </div>
          ))}
        </fieldset>
        {error ? <p role="alert" className="text-sm text-destructive">{COPY.people.submitError}</p> : null}
      </div>
    </ResponsivePanel>
  )
}
