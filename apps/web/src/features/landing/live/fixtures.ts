export const LANDING_WORKSPACE_ID = 7
export const LANDING_WORKSPACE_LABEL = 'jiguang'
export const LANDING_USER_LABEL = 'Administrator'

export function landingLocalDate(offsetDays = 0): string {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  date.setDate(date.getDate() + offsetDays)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export const landingUsers = [
  { id: 47, username: 'admin', display_name: 'Administrator' },
  { id: 7, username: 'nangqiao', display_name: '南乔' },
  { id: 8, username: 'mario', display_name: '马里奥' },
  { id: 9, username: 'teo', display_name: '铁奥' },
  { id: 10, username: 'ken', display_name: 'ken' },
  { id: 11, username: 'fuyu', display_name: '符钰' },
  { id: 12, username: 'kilian', display_name: 'Kilian' },
] as const

export const landingScopeRaw = {
  project_sets: [{ id: 8, name: 'S基建' }],
  projects: [{ id: 13, project_new_id: 8, title: 'V1.5' }],
  requirements: [{
    id: 21,
    version_task_new_id: 13,
    title: '测试-test最终',
  }],
}

export const landingScopeOptions = {
  projectSets: [{ id: 8, label: 'S基建' }],
  projects: [{ id: 13, label: 'V1.5', parentId: 8 }],
  requirements: [{ id: 21, label: '测试-test最终', parentId: 13 }],
}
