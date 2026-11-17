import type {
  ResourceAccessDraft,
  ResourceAccessLevel,
  ResourceAccessMode,
  ResourceAccessPolicy,
  ResourceAccessRule,
} from './types'

type EditableState = {
  base: ResourceAccessPolicy
  draft: ResourceAccessDraft
}

export type ResourceAccessEditorState =
  | ({ status: 'pristine' } & EditableState)
  | ({ status: 'dirty' } & EditableState)
  | ({ status: 'submitting' } & EditableState)
  | ({ status: 'conflict'; latest: ResourceAccessPolicy } & EditableState)
  | ({ status: 'unknown_result' } & EditableState)

export type ResourceAccessEditorAction =
  | { type: 'hydrate'; policy: ResourceAccessPolicy; force?: boolean }
  | { type: 'set_mode'; mode: Exclude<ResourceAccessMode, 'legacy'> }
  | { type: 'set_workspace_level'; level: ResourceAccessLevel }
  | { type: 'add_rule'; rule: ResourceAccessRule }
  | { type: 'set_rule_level'; subjectKind: ResourceAccessRule['subjectKind']; subjectId: string; level: ResourceAccessLevel }
  | { type: 'set_rule_descendants'; subjectKind: ResourceAccessRule['subjectKind']; subjectId: string; included: boolean }
  | { type: 'remove_rule'; subjectKind: ResourceAccessRule['subjectKind']; subjectId: string }
  | { type: 'submit' }
  | { type: 'submit_succeeded'; policy: ResourceAccessPolicy }
  | { type: 'submit_conflicted'; latest: ResourceAccessPolicy }
  | { type: 'submit_unknown' }
  | { type: 'submit_failed' }
  | { type: 'rebase' }
  | { type: 'resolve_unknown'; latest: ResourceAccessPolicy }
  | { type: 'discard' }

export type ResourceAccessEditorIntent = Exclude<ResourceAccessEditorAction, {
  type:
    | 'hydrate'
    | 'submit'
    | 'submit_succeeded'
    | 'submit_conflicted'
    | 'submit_unknown'
    | 'submit_failed'
    | 'resolve_unknown'
}>

export type ResourceAccessChangeImpact = 'unchanged' | 'expanded' | 'reduced' | 'mixed'

function draftFromPolicy(policy: ResourceAccessPolicy): ResourceAccessDraft {
  return {
    mode: policy.mode,
    workspaceLevel: policy.workspaceLevel ?? 'viewer',
    rules: policy.rules,
  }
}

export function createResourceAccessEditorState(
  policy: ResourceAccessPolicy,
): ResourceAccessEditorState {
  return { status: 'pristine', base: policy, draft: draftFromPolicy(policy) }
}

function editable(state: ResourceAccessEditorState): boolean {
  return state.status !== 'submitting' && state.base.canManageAccess && state.base.mode !== 'legacy'
}

function changeDraft(
  state: ResourceAccessEditorState,
  update: (draft: ResourceAccessDraft) => ResourceAccessDraft,
): ResourceAccessEditorState {
  if (!editable(state)) return state
  const draft = update(state.draft)
  return sameDraft(draft, draftFromPolicy(state.base))
    ? { status: 'pristine', base: state.base, draft }
    : { status: 'dirty', base: state.base, draft }
}

function matchesRule(left: ResourceAccessRule, right: Pick<ResourceAccessRule, 'subjectKind' | 'subjectId'>) {
  return left.subjectKind === right.subjectKind && left.subjectId === right.subjectId
}

export function resourceAccessEditorReducer(
  state: ResourceAccessEditorState,
  action: ResourceAccessEditorAction,
): ResourceAccessEditorState {
  switch (action.type) {
    case 'hydrate':
      if (!action.force && state.status !== 'pristine') return state
      return createResourceAccessEditorState(action.policy)
    case 'set_mode':
      return changeDraft(state, (draft) => ({ ...draft, mode: action.mode }))
    case 'set_workspace_level':
      return changeDraft(state, (draft) => ({ ...draft, workspaceLevel: action.level }))
    case 'add_rule':
      if (!state.base.subjectCapabilities[action.rule.subjectKind].available) return state
      return changeDraft(state, (draft) => ({
        ...draft,
        rules: draft.rules.some((rule) => matchesRule(rule, action.rule))
          ? draft.rules
          : [...draft.rules, action.rule],
      }))
    case 'set_rule_level':
      return changeDraft(state, (draft) => ({
        ...draft,
        rules: draft.rules.map((rule) => matchesRule(rule, action)
          ? { ...rule, level: action.level }
          : rule),
      }))
    case 'set_rule_descendants':
      return changeDraft(state, (draft) => ({
        ...draft,
        rules: draft.rules.map((rule) => matchesRule(rule, action)
          ? { ...rule, includeDescendants: action.included }
          : rule),
      }))
    case 'remove_rule':
      return changeDraft(state, (draft) => ({
        ...draft,
        rules: draft.rules.filter((rule) => !matchesRule(rule, action)),
      }))
    case 'submit':
      return state.status === 'dirty' ? { ...state, status: 'submitting' } : state
    case 'submit_succeeded':
      return state.status === 'submitting'
        ? createResourceAccessEditorState(action.policy)
        : state
    case 'submit_conflicted':
      return state.status === 'submitting'
        ? { ...state, status: 'conflict', latest: action.latest }
        : state
    case 'submit_unknown':
      return state.status === 'submitting' ? { ...state, status: 'unknown_result' } : state
    case 'submit_failed':
      return state.status === 'submitting' ? { ...state, status: 'dirty' } : state
    case 'rebase':
      return state.status === 'conflict'
        ? { status: 'dirty', base: state.latest, draft: state.draft }
        : state
    case 'resolve_unknown':
      if (state.status !== 'unknown_result') return state
      return sameDraft(state.draft, draftFromPolicy(action.latest))
        ? createResourceAccessEditorState(action.latest)
        : { ...state, status: 'conflict', latest: action.latest }
    case 'discard':
      return createResourceAccessEditorState(
        state.status === 'conflict' ? state.latest : state.base,
      )
  }
}

export function sameDraft(left: ResourceAccessDraft, right: ResourceAccessDraft): boolean {
  if (left.mode !== right.mode || left.workspaceLevel !== right.workspaceLevel) return false
  if (left.rules.length !== right.rules.length) return false
  return left.rules.every((rule, index) => {
    const candidate = right.rules[index]
    return Boolean(candidate && matchesRule(rule, candidate)
      && rule.level === candidate.level
      && rule.includeDescendants === candidate.includeDescendants)
  })
}

const EXPOSURE_RANK: Record<ResourceAccessMode, number> = {
  private: 0,
  restricted: 1,
  link: 1,
  workspace: 2,
  legacy: 1,
}

export function resourceAccessChangeImpact(
  base: ResourceAccessPolicy,
  draft: ResourceAccessDraft,
): ResourceAccessChangeImpact {
  if (sameDraft(draft, draftFromPolicy(base))) return 'unchanged'
  const modeDelta = EXPOSURE_RANK[draft.mode] - EXPOSURE_RANK[base.mode]
  if (modeDelta > 0) return 'expanded'
  if (modeDelta < 0) return 'reduced'
  if (draft.mode === 'workspace' && base.workspaceLevel !== draft.workspaceLevel) {
    return draft.workspaceLevel === 'editor' ? 'expanded' : 'reduced'
  }
  if (draft.mode === 'restricted' && base.mode === 'restricted') {
    const additions = draft.rules.filter((rule) => !base.rules.some((item) => matchesRule(item, rule)))
    const removals = base.rules.filter((rule) => !draft.rules.some((item) => matchesRule(item, rule)))
    if (additions.length && !removals.length) return 'expanded'
    if (removals.length && !additions.length) return 'reduced'
  }
  return 'mixed'
}
