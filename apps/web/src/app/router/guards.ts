import type { QueryClient } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'

import {
  ensureAuthenticatedIdentity,
} from '@/features/auth/api/runtime'
import {
  resolveWorkspaceForRoute,
  type WorkspaceRouteResolution,
} from '@/features/workspace/api/runtime'
import { optionalPMRedirect, safePMRedirect } from '@/shared/config'
import {
  applyHostData,
  getRuntimeSession,
  isPMMicroApp,
  pmAccountStatus,
  runtimeSessionStore,
  waitForPMHostData,
  type PMAccountStatus,
} from '@/shared/runtime'

export type RouteAccessSnapshot = {
  embedded: boolean
  token: string
  workspaceId: number
  accountStatus?: PMAccountStatus
  passwordResetRequired?: boolean
}

export type RouteAccess = {
  snapshot: () => RouteAccessSnapshot
  waitForHostSession: () => Promise<RouteAccessSnapshot>
  ensureIdentity: (queryClient: QueryClient) => Promise<RouteAccessSnapshot>
  resolveWorkspace: (queryClient: QueryClient) => Promise<WorkspaceRouteResolution>
}

export type AppRouterContext = {
  queryClient: QueryClient
  access: RouteAccess
}

type GuardInput = {
  context: AppRouterContext
  location: { href: string }
}

export class EmbeddedSessionUnavailableError extends Error {
  constructor() {
    super('Embedded PM session is unavailable')
    this.name = 'EmbeddedSessionUnavailableError'
  }
}

function runtimeSnapshot(): RouteAccessSnapshot {
  const session = getRuntimeSession()
  const user = runtimeSessionStore.getState().user
  return {
    embedded: session.embedded || isPMMicroApp(),
    token: session.token,
    workspaceId: session.workspaceId,
    accountStatus: pmAccountStatus(user),
    passwordResetRequired: Boolean(user?.password_reset_required),
  }
}

export const routeAccess: RouteAccess = {
  snapshot: runtimeSnapshot,
  waitForHostSession: async () => {
    const data = await waitForPMHostData()
    if (data) applyHostData(data)
    return runtimeSnapshot()
  },
  ensureIdentity: async (queryClient) => {
    await ensureAuthenticatedIdentity(queryClient)
    return runtimeSnapshot()
  },
  resolveWorkspace: resolveWorkspaceForRoute,
}

function standaloneLoginRedirect(location: { href: string }): never {
  throw redirect({
    to: '/login',
    search: {
      redirect: optionalPMRedirect(location.href),
      login_code: undefined,
      external_registration_code: undefined,
      login_provider: undefined,
    },
    replace: true,
  })
}

function isIdentityStatusLocation(href: string): boolean {
  try {
    return new URL(href, window.location.origin).pathname === '/identity-status'
  } catch {
    return href.split('?')[0] === '/identity-status'
  }
}

function isPasswordResetLocation(href: string): boolean {
  try {
    return new URL(href, window.location.origin).pathname === '/reset-password'
  } catch {
    return href.split('?')[0] === '/reset-password'
  }
}

async function authenticatedSnapshot({
  context,
  location,
}: GuardInput): Promise<RouteAccessSnapshot> {
  let snapshot = context.access.snapshot()
  if (snapshot.embedded && !snapshot.token) {
    snapshot = await context.access.waitForHostSession()
  }
  if (snapshot.token) {
    try {
      snapshot = await context.access.ensureIdentity(context.queryClient)
    } catch {
      if (snapshot.embedded) throw new EmbeddedSessionUnavailableError()
      standaloneLoginRedirect(location)
    }
  }
  if (snapshot.token) {
    if (
      snapshot.accountStatus &&
      snapshot.accountStatus !== 'active' &&
      !isIdentityStatusLocation(location.href)
    ) {
      throw redirect({ to: '/identity-status', replace: true })
    }
    if (
      snapshot.passwordResetRequired &&
      !isPasswordResetLocation(location.href)
    ) {
      throw redirect({ to: '/reset-password', replace: true })
    }
    return snapshot
  }
  if (snapshot.embedded) throw new EmbeddedSessionUnavailableError()
  standaloneLoginRedirect(location)
}

export async function requireAuthentication(input: GuardInput): Promise<void> {
  await authenticatedSnapshot(input)
}

export async function requireRestrictedIdentity(input: GuardInput): Promise<void> {
  const snapshot = await authenticatedSnapshot(input)
  if (!snapshot.accountStatus || snapshot.accountStatus === 'active') {
    throw redirect({ to: '/workspace/projects', replace: true })
  }
}

export async function requireWorkspace(input: GuardInput): Promise<void> {
  const snapshot = await authenticatedSnapshot(input)
  if (snapshot.embedded) {
    if (snapshot.workspaceId > 0) {
      return
    }
    throw new EmbeddedSessionUnavailableError()
  }
  const resolution = await input.context.access.resolveWorkspace(
    input.context.queryClient,
  )
  if (resolution.status === 'ok') {
    return
  }
  throw redirect({
    to: '/select-workspace',
    search: {
      redirect: safePMRedirect(input.location.href),
      empty: resolution.status === 'empty' ? '1' : undefined,
      error: resolution.status === 'error' ? 'load' : undefined,
    },
    replace: true,
  })
}

export async function requireStandalone(input: GuardInput): Promise<void> {
  const snapshot = await authenticatedSnapshot(input)
  if (snapshot.embedded) {
    throw redirect({ to: '/workspace/projects', replace: true })
  }
}

export async function requireStandaloneSelection(input: GuardInput): Promise<void> {
  const snapshot = await authenticatedSnapshot(input)
  if (snapshot.embedded) {
    throw redirect({ to: '/workspace/projects', replace: true })
  }
}

export async function redirectAuthenticatedFromLogin(input: GuardInput): Promise<void> {
  let snapshot = input.context.access.snapshot()
  if (snapshot.embedded && !snapshot.token) {
    snapshot = await input.context.access.waitForHostSession()
  }
  if (!snapshot.token) return
  try {
    snapshot = await input.context.access.ensureIdentity(
      input.context.queryClient,
    )
  } catch {
    return
  }
  if (!snapshot.token) return
  throw redirect({
    to: snapshot.accountStatus && snapshot.accountStatus !== 'active'
      ? '/identity-status'
      : snapshot.passwordResetRequired
        ? '/reset-password'
        : '/workspace/projects',
    replace: true,
  })
}
