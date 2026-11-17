import { Component, createElement, lazy, Suspense, useMemo, useState, type ComponentType, type ReactNode } from 'react'
import { copy } from '@/shared/copy'
import { AlertBanner } from '../feedback/alert-banner'
import { LoadingSkeleton } from '../feedback/loading-skeleton'
import { Button } from '../primitives/button'

// Two editor layers may load sequentially. Each is bounded to 15 s so the
// document path cannot show an uninterrupted loading skeleton beyond 30 s.
export const RICH_TEXT_RUNTIME_TIMEOUT_MS = 15_000

export function loadRichTextRuntime<T>(load: () => Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('RICH_TEXT_RUNTIME_TIMEOUT')), RICH_TEXT_RUNTIME_TIMEOUT_MS)
    Promise.resolve().then(load).then(resolve, reject).finally(() => clearTimeout(timer))
  })
}

class RuntimeErrorBoundary extends Component<{ children: ReactNode; onRetry: () => void }, { failed: boolean }> {
  state = { failed: false }
  static getDerivedStateFromError() { return { failed: true } }
  render() {
    return this.state.failed ? <AlertBanner tone="error" action={
      <Button type="button" variant="outline" size="sm" onClick={this.props.onRetry}>{copy.ui.richText.retryLoading}</Button>
    }>{copy.ui.richText.loadError}</AlertBanner> : this.props.children
  }
}

/** Retry only the runtime, never the page or the parent-owned editor draft. */
export function RichTextRuntimeLoader<P extends object>({ load, runtimeProps, loadingLabel }: {
  load: () => Promise<{ default: ComponentType<P> }>
  runtimeProps: P
  loadingLabel: string
}) {
  const [attempt, setAttempt] = useState(0)
  const Runtime = useMemo(() => lazy(() => loadRichTextRuntime(load)), [load, attempt])
  return <RuntimeErrorBoundary key={attempt} onRetry={() => setAttempt(value => value + 1)}>
    <Suspense fallback={<LoadingSkeleton rows={1} label={loadingLabel} />}>
      {createElement(Runtime, runtimeProps)}
    </Suspense>
  </RuntimeErrorBoundary>
}
