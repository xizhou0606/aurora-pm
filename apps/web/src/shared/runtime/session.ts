import { clearPMHostData, type PMHostData } from './microApp'
import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
} from './state'

export {
  PM_STANDALONE_TOKEN_KEY,
  PM_STANDALONE_WORKSPACE_KEY,
} from './state'

export type RuntimeSession = {
  embedded: boolean
  token: string
  workspaceId: number
  timezone: string
}

export function getRuntimeSession(): Readonly<RuntimeSession> {
  const session = runtimeSessionStore.getState()
  return {
    embedded: session.mode === 'embedded',
    token: session.token,
    workspaceId: runtimeWorkspaceStore.getState().currentWorkspaceId,
    timezone: session.timezone,
  }
}

export function getRuntimeToken(): string {
  return runtimeSessionStore.getState().token
}

export function getRuntimeWorkspaceId(): number {
  return runtimeWorkspaceStore.getState().currentWorkspaceId
}

export function setStandaloneToken(token: string): void {
  runtimeSessionStore.getState().setStandaloneToken(token)
}

export function applyHostData(data: PMHostData): void {
  runtimeSessionStore.getState().applyHostData(data)
}

export function applyEmbeddedSession(input: {
  token: string
  workspaceId: number
  timezone?: string
}): void {
  runtimeSessionStore.getState().applyEmbeddedSession(input)
}

export function setRuntimeWorkspace(workspaceId: number): void {
  const session = runtimeSessionStore.getState()
  if (session.mode === 'embedded') {
    runtimeWorkspaceStore.getState().applyHostWorkspace(workspaceId)
  } else {
    runtimeWorkspaceStore.getState().selectWorkspace(workspaceId)
  }
}

export function clearRuntimeSession(reason = 'runtime_clear'): void {
  const embedded = runtimeSessionStore.getState().mode === 'embedded'
  runtimeSessionStore.getState().clearSession(reason)
  if (embedded) clearPMHostData()
}
