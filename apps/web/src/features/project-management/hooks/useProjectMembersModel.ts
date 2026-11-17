import { useEffect, useRef, useState } from 'react'
import { isSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import type { PutMembersPayload, RemovalImpact } from '../api'
import { memberSaveError, removalPreview, type MemberRemovalPreview } from '../model/memberRemoval'
import { useMemberRoster, useMemberImpactRequest, useMemberSaveRequest } from './useMemberRequests'

type Phase = 'editing' | 'checking' | 'confirming' | 'saving'

export function useProjectMembersModel(input: { planUnitId: number; currentOwnerId: number | null }) {
  const { planUnitId, currentOwnerId } = input
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const revision = useSessionStore((state) => state.revision)
  const roster = useMemberRoster(workspaceId, revision, planUnitId)
  const inspect = useMemberImpactRequest(workspaceId, revision, planUnitId)
  const mutation = useMemberSaveRequest(workspaceId, planUnitId)
  const members = roster.data ?? []
  const [participants, setParticipants] = useState<number[]>([])
  const [ownerId, setOwnerId] = useState<number | null>(currentOwnerId)
  const [phase, setPhase] = useState<Phase>('editing')
  const [error, setError] = useState<string | null>(null)
  const [preview, setPreview] = useState<MemberRemovalPreview | null>(null)
  const busy = useRef(false)
  const generation = useRef(0)
  useEffect(() => {
    generation.current += 1
    busy.current = false
    setPreview(null)
    setError(null)
    setPhase('editing')
    setParticipants([])
    setOwnerId(currentOwnerId)
    return () => { generation.current += 1 }
  }, [planUnitId, workspaceId, revision, currentOwnerId])
  useEffect(() => {
    if (!roster.data) return
    setParticipants(roster.data.map((member) => member.id).filter((id) => id !== currentOwnerId))
    setOwnerId(currentOwnerId)
  }, [roster.data, currentOwnerId])

  const submit = async (payload: PutMembersPayload, current: number): Promise<boolean> => {
    setPhase('saving')
    try {
      await mutation.mutateAsync(payload)
      if (generation.current !== current) return false
      setPreview(null)
      setError(null)
      setPhase('editing')
      return true
    } catch (err) {
      if (generation.current === current) {
        setError(memberSaveError(err, Boolean(payload.task_resolution)))
        if (payload.task_resolution && isSafeApiError(err) && err.status === 409) {
          setPreview((value) => value ? { ...value, blockedReason: copy.projectManagement.members.tasksChanged } : null)
        }
        setPhase(payload.task_resolution ? 'confirming' : 'editing')
      }
      return false
    }
  }
  const onSave = async (): Promise<boolean> => {
    if (busy.current || roster.isPending || roster.isError || workspaceId <= 0) return false
    if (ownerId == null) { setError(copy.projectManagement.members.ownerRequired); return false }
    busy.current = true
    const current = generation.current
    setError(null)
    setPhase('checking')
    const payload: PutMembersPayload = { owner_user_id: ownerId, user_ids: participants.filter((id) => id !== ownerId), reason: 'via project member editor' }
    try {
      const retained = new Set([payload.owner_user_id, ...payload.user_ids])
      const removed = members.filter((member) => !retained.has(member.id))
      const impacts = new Map<number, RemovalImpact>(await Promise.all(removed.map(async (member) => [member.id, await inspect(member.id)] as const)))
      if (generation.current !== current) return false
      const next = removalPreview(payload, impacts)
      setPreview(next)
      if (next) { setPhase('confirming'); return false }
      return await submit(payload, current)
    } catch {
      if (generation.current === current) { setError(copy.projectManagement.members.impactFailed); setPhase(preview ? 'confirming' : 'editing') }
      return false
    } finally {
      if (generation.current === current) busy.current = false
    }
  }
  const confirmRemoval = async (assigneeId: number | null): Promise<boolean> => {
    if (busy.current || !preview || preview.blockedReason || preview.tasks.length === 0) return false
    const retained = new Set([preview.payload.owner_user_id, ...preview.payload.user_ids])
    if (assigneeId != null && !retained.has(assigneeId)) return false
    busy.current = true
    setError(null)
    const current = generation.current
    try {
      return await submit({ ...preview.payload, task_resolution: { tasks: preview.tasks, assignee_user_id: assigneeId } }, current)
    } finally { if (generation.current === current) busy.current = false }
  }
  return {
    members, participants, ownerId, preview,
    loading: roster.isPending,
    saving: phase === 'saving' || phase === 'checking',
    error: error ?? (roster.isError ? copy.projectManagement.members.loadFailed : null),
    removalImpacts: preview?.impacts ?? new Map<number, RemovalImpact>(),
    onOwnerChange: (id: number) => { if (!busy.current) { setOwnerId(id); setParticipants((ids) => ids.filter((value) => value !== id)); setPreview(null) } },
    onParticipantsChange: (ids: number[]) => { if (!busy.current) { setParticipants(ids); setPreview(null) } },
    onSave, confirmRemoval,
    cancelRemoval: () => { if (!busy.current) { setPreview(null); setError(null); setPhase('editing') } },
    onReset: () => { if (!busy.current) { setPreview(null); setError(null); setPhase('editing'); setOwnerId(currentOwnerId); setParticipants(members.map((member) => member.id).filter((id) => id !== currentOwnerId)); void roster.refetch() } },
    reload: () => { void roster.refetch() },
  }
}
