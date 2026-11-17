import {
  PMSafeApiError,
  platformApi,
  shouldFallbackModuleApi,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'

export type InboxRequest = {
  path: string
  query?: Readonly<Record<string, unknown>>
  signal?: AbortSignal
}

export type InboxPage<T> = {
  items: T[]
  total: number
}

export type InboxClient = {
  request<T>(request: InboxRequest): Promise<T>
  requestPage<T>(request: InboxRequest): Promise<InboxPage<T>>
  reportRuntimeFailure?(error: unknown): void
}

async function requestLocalInbox<T>(request: InboxRequest): Promise<T> {
  try {
    const response = await platformApi.get(request.path, {
      params: request.query,
      signal: request.signal,
    })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'INBOX_REQUEST_FAILED')
  }
}

async function requestLocalInboxPage<T>(request: InboxRequest): Promise<InboxPage<T>> {
  try {
    const response = await platformApi.get(request.path, {
      params: request.query,
      signal: request.signal,
    })
    return {
      items: unwrapApiRows<T>(response),
      total: unwrapApiTotal(response),
    }
  } catch (error) {
    throw toSafeApiError(error, 'INBOX_REQUEST_FAILED')
  }
}

export const inboxClient: InboxClient = Object.freeze({
  request: requestLocalInbox,
  requestPage: requestLocalInboxPage,
})

export function withInboxRuntimeFailure(
  client: InboxClient,
  onRuntimeFailure?: (error: unknown) => void,
): InboxClient {
  if (!onRuntimeFailure) return client
  const reportRuntimeFailure = (error: unknown) => {
    if (shouldFallbackModuleApi(error)) onRuntimeFailure(error)
  }
  const guard = async <T,>(operation: () => Promise<T>): Promise<T> => {
    try {
      return await operation()
    } catch (error) {
      reportRuntimeFailure(error)
      throw error
    }
  }
  return {
    request: (request) => guard(() => client.request(request)),
    requestPage: (request) => guard(() => client.requestPage(request)),
    reportRuntimeFailure,
  }
}

export function failInboxContract(client: InboxClient): never {
  const error = inboxContractError()
  client.reportRuntimeFailure?.(error)
  throw error
}

/**
 * Scope options are auxiliary while Inbox is unfiltered. Keep their contract
 * failure inside the scope control so independently validated todo feeds stay
 * usable; selected routes still gate every data query on valid scope options.
 */
export function failInboxScopeContract(): never {
  throw inboxContractError()
}

function inboxContractError(): PMSafeApiError {
  return new PMSafeApiError('INBOX_CONTRACT_INVALID', 502)
}
