import { useEffect, useRef } from 'react'

import { copy } from '@/shared/copy'

export type RouteBoundaryId =
  | 'login'
  | 'workspace-select'
  | 'projects'
  | 'project-detail'
  | 'project-item-detail'
  | 'host-content'
  | 'pending'
  | 'error'
  | 'not-found'

export interface RouteBoundaryProps {
  id: RouteBoundaryId
  embeddedSessionUnavailable?: boolean
}

function boundaryCopy(
  id: RouteBoundaryId,
  embeddedSessionUnavailable: boolean,
): { title: string; description?: string } | null {
  if (embeddedSessionUnavailable) {
    return {
      title: copy.auth.sessionUnavailableTitle,
      description: copy.auth.sessionUnavailableDescription,
    }
  }
  if (id === 'pending') return { title: copy.ui.loading.label }
  if (id === 'error') {
    return {
      title: copy.errors.genericTitle,
      description: copy.errors.genericDescription,
    }
  }
  if (id === 'not-found') {
    return {
      title: copy.errors.notFoundTitle,
      description: copy.errors.notFoundDescription,
    }
  }
  return null
}

export function RouteBoundary({
  id,
  embeddedSessionUnavailable = false,
}: RouteBoundaryProps) {
  const content = boundaryCopy(id, embeddedSessionUnavailable)
  const focusRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (content && id !== 'pending') focusRef.current?.focus()
  }, [embeddedSessionUnavailable, id])

  if (!content) {
    return (
      <div
        className="min-h-full min-w-0 w-full"
        data-route-boundary={id}
        data-testid={`route-${id}`}
      />
    )
  }

  const pending = id === 'pending' && !embeddedSessionUnavailable
  return (
    <div
      ref={focusRef}
      role={pending ? 'status' : 'alert'}
      aria-live={pending ? 'polite' : 'assertive'}
      aria-busy={pending || undefined}
      tabIndex={pending ? undefined : -1}
      data-route-boundary={id}
      data-testid={`route-${id}`}
      className="grid min-h-dvh min-w-0 w-full place-items-center bg-background px-6 text-center outline-none"
    >
      <div className="max-w-md">
        <h1 className="m-0 text-lg font-semibold text-foreground">
          {content.title}
        </h1>
        {content.description ? (
          <p className="mt-1.5 mb-0 text-sm leading-5 text-muted-foreground">
            {content.description}
          </p>
        ) : null}
      </div>
    </div>
  )
}
