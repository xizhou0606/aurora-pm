import {
  RESOURCE_ACCESS_SUBJECT_KINDS,
  type ResourceAccessSubjectCapabilities,
} from './types'

export function unavailableSubjectCapabilities(): ResourceAccessSubjectCapabilities {
  return Object.fromEntries(RESOURCE_ACCESS_SUBJECT_KINDS.map((kind) => [
    kind,
    { available: false },
  ])) as ResourceAccessSubjectCapabilities
}
