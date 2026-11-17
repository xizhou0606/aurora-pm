import { describe, expect, it, vi } from 'bun:test'

import { createNativeInternalNavigator } from './internal'

describe('createNativeInternalNavigator', () => {
  it('keeps canonical query values and duplicate entries for route validation', async () => {
    const routerNavigate = vi.fn().mockResolvedValue(undefined)
    const navigate = createNativeInternalNavigator(routerNavigate, {
      deniedMessage: 'denied',
    })

    await navigate(
      '/workspace/work-items?project_set_ids=5&statuses=draft&statuses=done&overdue=true&page=2',
    )

    expect(routerNavigate).toHaveBeenCalledWith({
      to: '/workspace/work-items',
      search: {
        project_set_ids: 5,
        statuses: 'draft,done',
        overdue: true,
        page: 2,
      },
      hash: '',
      replace: false,
    })
  })

  it('preserves safe hashes, strips sensitive values, and forwards replace', async () => {
    const routerNavigate = vi.fn().mockResolvedValue(undefined)
    const navigate = createNativeInternalNavigator(routerNavigate, {
      deniedMessage: 'denied',
    })

    await navigate('/workspace/inbox?project_set_ids=5&token=secret#scope', true)

    expect(routerNavigate).toHaveBeenCalledWith({
      to: '/workspace/inbox',
      search: { project_set_ids: 5 },
      hash: 'scope',
      replace: true,
    })
  })

  it('splits Work-items detail search from the path instead of creating a second question mark', async () => {
    const routerNavigate = vi.fn().mockResolvedValue(undefined)
    const navigate = createNativeInternalNavigator(routerNavigate, {
      deniedMessage: 'denied',
    })

    await navigate('/workspace/work-items/71?view=list&layout=gantt')

    expect(routerNavigate).toHaveBeenCalledWith({
      to: '/workspace/work-items/71',
      search: { view: 'list', layout: 'gantt' },
      hash: '',
      replace: false,
    })
  })

  it('fails closed for unsafe paths and caller-owned route boundaries', async () => {
    const routerNavigate = vi.fn().mockResolvedValue(undefined)
    const navigate = createNativeInternalNavigator(routerNavigate, {
      deniedMessage: 'denied',
      allow: (safePath) => safePath.startsWith('/workspace/work-items'),
    })

    await expect(navigate('https://evil.example/workspace/work-items'))
      .rejects.toThrow('denied')
    await expect(navigate('/workspace/projects/9')).rejects.toThrow('denied')
    expect(routerNavigate).not.toHaveBeenCalled()
  })
})
