import { Component, type ReactNode } from 'react'

export class LandingPreviewBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  render() {
    if (this.state.failed) {
      return <div className="h-full min-h-0 bg-background" />
    }
    return this.props.children
  }
}
