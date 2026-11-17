import { useCallback, useDeferredValue, useState } from 'react'

import {
  resolveProjectListState,
  usePlatformUsersQuery,
  useProjectListQuery,
} from '../model'

const PROJECT_PAGE_SIZE = 20

export function useProjectListModel(selectedProjectId?: number, enabled = true) {
  const [search, setSearch] = useState('')
  const deferredSearch = useDeferredValue(search)
  const [page, setPage] = useState(1)
  const projectsQuery = useProjectListQuery({
    q: deferredSearch,
    limit: PROJECT_PAGE_SIZE,
    offset: (page - 1) * PROJECT_PAGE_SIZE,
  }, enabled)
  const projects = projectsQuery.data?.rows ?? []
  const usersQuery = usePlatformUsersQuery(enabled)
  const retry = useCallback(() => {
    void projectsQuery.refetch()
  }, [projectsQuery.refetch])
  const changeSearch = useCallback((value: string) => {
    setSearch(value)
    setPage(1)
  }, [])
  const changePage = useCallback((value: number) => {
    setPage(value)
  }, [])

  return {
    projects,
    state: resolveProjectListState({
      status: projectsQuery.status,
      projects,
      total: projectsQuery.data?.total ?? 0,
      fetching: projectsQuery.isFetching && !projectsQuery.isPending,
      search: deferredSearch,
      selectedProjectId,
      users: usersQuery.data ?? [],
      retry,
    }),
    search: {
      value: search,
      onChange: changeSearch,
    },
    pagination: {
      page,
      pageSize: PROJECT_PAGE_SIZE,
      onChange: changePage,
    },
  }
}
