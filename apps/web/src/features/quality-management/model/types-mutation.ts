export type QualityMutationAction =
  | 'create'
  | 'edit'
  | 'assign'
  | 'resolve'
  | 'return'
  | 'transition'
  | 'delete'
  | 'upload'
  | 'create_scene_option'
  | 'delete_scene_option'
  | 'comment'
  | 'delete_comment'

export type QualityMutationState =
  | { status: 'idle'; action: QualityMutationAction }
  | { status: 'pending'; action: QualityMutationAction }
  | { status: 'error'; action: QualityMutationAction; error: unknown; reset: () => void }
  | { status: 'success'; action: QualityMutationAction; reset: () => void }
