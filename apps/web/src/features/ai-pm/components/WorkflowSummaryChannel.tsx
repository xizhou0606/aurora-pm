import { createContext, useContext, useState, type ReactNode } from 'react'

type WorkflowSummaryChannelValue = {
  host: HTMLDivElement | null
  setHost: (element: HTMLDivElement | null) => void
}

const WorkflowSummaryChannelContext = createContext<WorkflowSummaryChannelValue | null>(null)

/**
 * Wires the compact workflow progress summary (rendered inside the workflow
 * control) to a host node placed elsewhere, e.g. the resource hero banner row,
 * so the summary can share the header row while the node track keeps its own
 * full-width row.
 */
export function WorkflowSummaryChannel({ children }: { children: ReactNode }) {
  const [host, setHost] = useState<HTMLDivElement | null>(null)
  return (
    <WorkflowSummaryChannelContext.Provider value={{ host, setHost }}>
      {children}
    </WorkflowSummaryChannelContext.Provider>
  )
}

/** Slot rendered inside the hero banner row that receives the workflow summary. */
export function WorkflowSummaryHost() {
  const channel = useContext(WorkflowSummaryChannelContext)
  if (!channel) return null
  return (
    <div
      ref={channel.setHost}
      data-testid="workflow-summary-host"
      className="flex min-w-0 items-center"
    />
  )
}

export function useWorkflowSummaryHost(): HTMLDivElement | null {
  return useContext(WorkflowSummaryChannelContext)?.host ?? null
}
