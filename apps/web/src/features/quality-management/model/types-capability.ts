export type QualityCapabilitySet = {
  canUpdate: boolean
  canDelete: boolean
  canAssign: boolean
  canResolve: boolean
  canReturn: boolean
  canTransition: boolean
  canUpload: boolean
  allowedTransitions: string[]
}

export type QualityLifecycleAction =
  | {
      id: string
      kind: 'transition'
      status: string
      label: string
      variant: 'default' | 'outline'
      disabled: boolean
    }
  | { id: 'assign'; kind: 'assign'; label: string; variant: 'outline'; disabled: false }
  | { id: 'resolve'; kind: 'resolve'; label: string; variant: 'outline'; disabled: false }
  | { id: 'return'; kind: 'return'; label: string; variant: 'outline'; disabled: false }

export type QualityLifecycleModel = {
  currentLabel: string
  actions: readonly QualityLifecycleAction[]
  transitionError: boolean
}
