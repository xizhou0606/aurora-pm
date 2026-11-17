import {
  useMediaQuery as useBaseMediaQuery,
  type UseMediaQueryOptions,
} from '@base-ui/react/unstable-use-media-query'

/**
 * Keeps the project's hydration default stable while Base UI's media-query hook
 * remains published under an unstable package entrypoint.
 */
export function useMediaQuery(
  query: string,
  options: UseMediaQueryOptions = {},
): boolean {
  return useBaseMediaQuery(query, {
    defaultMatches: false,
    ...options,
  })
}
