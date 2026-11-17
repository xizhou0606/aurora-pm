import { resourceAccessClient, type ResourceAccessClient } from '../api'

export type ResourceAccessEnvironment = {
  workspaceId: number
  sessionRevision: number
  resourceId: string
  client: ResourceAccessClient
}

export function createResourceAccessEnvironment(
  input: Omit<ResourceAccessEnvironment, 'client'>,
  client: ResourceAccessClient = resourceAccessClient,
): ResourceAccessEnvironment {
  return { ...input, client }
}

export function isValidResourceAccessEnvironment(environment: ResourceAccessEnvironment): boolean {
  return Number.isInteger(environment.workspaceId) && environment.workspaceId > 0
    && environment.sessionRevision >= 0
    && Boolean(environment.resourceId.trim())
}
