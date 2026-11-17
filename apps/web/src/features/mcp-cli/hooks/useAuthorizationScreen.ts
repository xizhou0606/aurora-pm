import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef } from 'react'

import { pmKeys } from '@/shared/query'
import { useWorkspaceStore } from '@/shared/state'

import { decideAuthorization, fetchAuthorizationRequest, startAuthorization } from '../api/api'
import type { ExternalChannel, OAuthAuthorizeParameters } from '../model/contract'
import type { AuthorizationAccess, AuthorizationScreenModel } from '../model/model'
import { effectiveAccessQueryKey } from './helpers'

export function useAuthorizationScreen(
  input: { requestId?: string; channel?: ExternalChannel; oauth?: OAuthAuthorizeParameters },
  access: AuthorizationAccess,
  onRequestStarted: (requestId: string) => void,
  onOAuthRedirect: (url: string) => void = (url) => { window.location.assign(url) },
): AuthorizationScreenModel {
  const { requestId, channel, oauth } = input
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryWorkspaceId = workspaceId > 0 ? workspaceId : 1
  const queryClient = useQueryClient()
  const startKeyRef = useRef<string | null>(null)
  const start = useMutation({
    mutationFn: startAuthorization,
    onSuccess: (startedRequestId) => onRequestStarted(startedRequestId),
  })
  const startChannel = oauth?.channel ?? channel
  const startInput = useMemo(() => oauth ? {
    channel: oauth.channel,
    oauth: {
      response_type: oauth.responseType,
      client_id: oauth.clientId,
      redirect_uri: oauth.redirectUri,
      code_challenge: oauth.codeChallenge,
      code_challenge_method: oauth.codeChallengeMethod,
      state: oauth.state,
      resource: oauth.resource,
      scope: oauth.scope,
    },
  } : startChannel ? { channel: startChannel } : null, [oauth, startChannel])
  const startKey = requestId || !startInput || workspaceId <= 0
    ? null
    : `${workspaceId}:${oauth ? `oauth:${oauth.clientId}:${oauth.state}` : `manual:${startChannel}`}`
  useEffect(() => {
    if (!startKey || !startInput || !access.createAuthorizationRequest || startKeyRef.current === startKey) return
    startKeyRef.current = startKey
    start.mutate(startInput)
  }, [access.createAuthorizationRequest, start, startInput, startKey])
  const request = useQuery({
    queryKey: pmKeys.resource(
      queryWorkspaceId,
      'workspace',
      'externalTooling',
      'authorizationRequest',
      requestId ?? 'invalid',
    ),
    queryFn: ({ signal }) => fetchAuthorizationRequest(requestId!, signal),
    enabled: workspaceId > 0 && Boolean(requestId) && access.viewAuthorizationRequest,
    retry: false,
  })
  const decision = useMutation({
    mutationFn: (value: 'authorize' | 'deny') => {
      if (!access.decideAuthorizationRequest) {
        return Promise.reject(new Error('authorization decision permission required'))
      }
      return decideAuthorization(requestId!, value, request.data?.oauth === true)
    },
    onSuccess: (redirect, value) => {
      if (value === 'authorize' && workspaceId > 0) {
        void queryClient.invalidateQueries({ queryKey: effectiveAccessQueryKey(workspaceId) })
      }
      if (redirect) onOAuthRedirect(redirect)
    },
  })
  if (!requestId && !startInput) return { status: 'invalid' }
  if (!requestId) {
    if (!access.createAuthorizationRequest) return { status: 'forbidden' }
    if (start.isError) {
      return {
        status: 'error',
        retry: () => {
          if (!startInput) return
          startKeyRef.current = startKey
          start.mutate(startInput)
        },
      }
    }
    return { status: 'loading' }
  }
  if (!access.viewAuthorizationRequest) return { status: 'forbidden' }
  if (request.isPending) return { status: 'loading' }
  if (request.isError || !request.data) {
    return { status: 'error', retry: () => { void request.refetch() } }
  }
  if (decision.isSuccess) {
    if (request.data.oauth) return { status: 'redirecting', request: request.data }
    return decision.variables === 'authorize'
      ? { status: 'authorized', request: request.data }
      : { status: 'denied', request: request.data }
  }
  return {
    status: 'ready',
    request: request.data,
    submit: decision.isPending ? 'pending' : decision.isError ? 'error' : 'idle',
    canDecide: access.decideAuthorizationRequest,
    authorize: () => decision.mutate('authorize'),
    deny: () => decision.mutate('deny'),
  }
}