import {
  Component,
  type ReactNode,
} from 'react'

import { copy } from '@/shared/copy'
import { Button } from '@/shared/ui'

export class NativeRenderBoundary extends Component<
  {
    fallback: ReactNode
    children: ReactNode
  },
  { failed: boolean }
> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children
  }
}

export function NativeModuleError({
  reason,
  onRetry,
}: {
  reason: 'api' | 'load' | 'render'
  onRetry?: () => void
}) {
  const description = reason === 'api'
    ? copy.app.nativeModules.apiErrorDescription
    : reason === 'render'
      ? copy.app.nativeModules.renderErrorDescription
      : copy.app.nativeModules.loadErrorDescription
  return (
    <section
      role="alert"
      tabIndex={-1}
      className="grid min-h-0 flex-1 place-content-center justify-items-center gap-3 bg-background px-6 text-center outline-none"
    >
      <h2 className="m-0 text-lg font-semibold text-foreground">
        {copy.app.nativeModules.unavailableTitle}
      </h2>
      <p className="m-0 text-sm text-muted-foreground">{description}</p>
      {onRetry ? (
        <Button type="button" variant="outline" onClick={onRetry}>
          {copy.app.nativeModules.retry}
        </Button>
      ) : null}
    </section>
  )
}

export function NativeLoading() {
  return (
    <div
      role="status"
      aria-live="polite"
      className="pm-loading-reveal grid min-h-0 flex-1 place-content-center bg-background text-sm text-muted-foreground"
    >
      {copy.app.nativeModules.loading}
    </div>
  )
}
