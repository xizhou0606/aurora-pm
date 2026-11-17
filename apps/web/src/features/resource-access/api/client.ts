import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import type {
  ResourceAccessGrant,
  ResourceAccessPolicy,
  ResourceAccessPolicyReplacement,
  ResourceAccessSubjectKind,
  ResourceAccessSubjectPage,
  ResourceShareLink,
  ResourceShareLinkCreate,
  ResourceShareLinkSecret,
} from '../model'
import { resourceAccessEndpoints } from './endpoints'
import {
  normalizeAccessGrant,
  normalizePolicy,
  normalizeShareLinkSecret,
  normalizeShareLinks,
  normalizeSubjectPage,
} from './normalizers'

async function request<T>(call: () => Promise<unknown>, normalize: (value: unknown) => T): Promise<T> {
  try {
    return normalize(unwrapApiData(await call()))
  } catch (error) {
    throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED')
  }
}

function ruleBody(rule: ResourceAccessPolicyReplacement['rules'][number]) {
  return {
    subject_kind: rule.subjectKind,
    subject_id: rule.subjectId,
    level: rule.level,
    ...(rule.subjectKind === 'organization_unit'
      ? { include_descendants: rule.includeDescendants }
      : {}),
  }
}

export const resourceAccessClient = {
  policy(resourceId: string, signal?: AbortSignal): Promise<ResourceAccessPolicy> {
    return request(
      () => platformApi.get(resourceAccessEndpoints.policy(resourceId), { signal }),
      (value) => {
        const policy = normalizePolicy(value)
        if (policy.resourceId !== resourceId) throw new Error('Resource access scope mismatch')
        return policy
      },
    )
  },

  replacePolicy(
    resourceId: string,
    input: ResourceAccessPolicyReplacement,
  ): Promise<ResourceAccessPolicy> {
    return request(
      () => platformApi.put(resourceAccessEndpoints.policy(resourceId), {
        mode: input.mode,
        ...(input.mode === 'workspace' ? { workspace_level: input.workspaceLevel } : {}),
        rules: input.mode === 'restricted' ? input.rules.map(ruleBody) : [],
        expected_policy_revision: input.expectedPolicyRevision,
      }, { headers: { 'Idempotency-Key': input.idempotencyKey } }),
      (value) => {
        const policy = normalizePolicy(value)
        if (policy.resourceId !== resourceId) throw new Error('Resource access scope mismatch')
        return policy
      },
    )
  },

  subjects(input: {
    kind: ResourceAccessSubjectKind
    query: string
    limit: number
    offset: number
    signal?: AbortSignal
  }): Promise<ResourceAccessSubjectPage> {
    return request(
      () => platformApi.get(resourceAccessEndpoints.subjects, {
        params: { kind: input.kind, q: input.query, limit: input.limit, offset: input.offset },
        signal: input.signal,
      }),
      (value) => {
        const page = normalizeSubjectPage(value)
        if (page.items.some((subject) => subject.kind !== input.kind)) {
          throw new Error('Resource access subject scope mismatch')
        }
        return page
      },
    )
  },

  links(resourceId: string, signal?: AbortSignal): Promise<readonly ResourceShareLink[]> {
    return request(
      () => platformApi.get(resourceAccessEndpoints.links(resourceId), { signal }),
      normalizeShareLinks,
    )
  },

  createLink(resourceId: string, input: ResourceShareLinkCreate): Promise<ResourceShareLinkSecret> {
    return request(
      () => platformApi.post(resourceAccessEndpoints.links(resourceId), {
        level: input.level,
        expires_in: input.expiresIn,
      }, { headers: { 'Idempotency-Key': input.idempotencyKey } }),
      normalizeShareLinkSecret,
    )
  },

  revokeLink(resourceId: string, linkId: string): Promise<void> {
    return request(
      () => platformApi.delete(resourceAccessEndpoints.link(resourceId, linkId)),
      () => undefined,
    )
  },

  rotateLink(resourceId: string, linkId: string): Promise<ResourceShareLinkSecret> {
    return request(
      () => platformApi.post(resourceAccessEndpoints.rotateLink(resourceId, linkId)),
      normalizeShareLinkSecret,
    )
  },

  exchangeLink(secret: string): Promise<ResourceAccessGrant> {
    return request(
      () => platformApi.post(resourceAccessEndpoints.exchangeLink, { secret }),
      normalizeAccessGrant,
    )
  },
}

export type ResourceAccessClient = typeof resourceAccessClient
