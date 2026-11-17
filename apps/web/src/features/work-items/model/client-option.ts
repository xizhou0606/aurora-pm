export type WorkItemClientOption = {
  id: number
  name: string
  usageCount: number
}

export type WorkItemClientOptionsState =
  | { status: 'loading'; rows: readonly WorkItemClientOption[] }
  | { status: 'error'; rows: readonly WorkItemClientOption[]; retry: () => void }
  | { status: 'ready'; rows: readonly WorkItemClientOption[] }

export type WorkItemClientOptionMutations = {
  canCreate: boolean
  canUpdate: boolean
  canDelete: (option: WorkItemClientOption) => boolean
  pending: boolean
  create(name: string): Promise<WorkItemClientOption>
  update(option: WorkItemClientOption, name: string): Promise<WorkItemClientOption>
  remove(option: WorkItemClientOption): Promise<void>
}
