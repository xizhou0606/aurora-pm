import { searchFieldsMatch, USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
export { textMatchesQuery } from '@/shared/lib/search'

export type UserSelectOption = {
  id: number
  name: string
  username?: string
  secondary?: string
  avatarUrl?: string
  disabled?: boolean
}

export function userSelectOptionEquals(
  left: UserSelectOption,
  right: UserSelectOption,
): boolean {
  return left.id === right.id
}

export function userSelectOptionMatches(
  option: UserSelectOption,
  query: string,
  itemToString?: (itemValue: UserSelectOption) => string,
  search: SearchOptions = USER_SEARCH,
): boolean {
  return searchFieldsMatch([option?.name, option?.username, option?.secondary, itemToString?.(option)], query, search)
}
