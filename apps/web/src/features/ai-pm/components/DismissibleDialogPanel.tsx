import type { ReactNode } from 'react'
import { copy } from '@/shared/copy'
import { ResponsivePanel } from '@/shared/ui'

export function DismissibleDialogPanel({
  title,
  children,
  onDismiss,
  className,
}: {
  title: string
  children: ReactNode
  onDismiss?: () => void
  className?: string
}) {
  return (
    <ResponsivePanel
      key="workflow-node-action"
      open
      onOpenChange={(next) => { if (!next) onDismiss?.() }}
      title={title}
      closeLabel={copy.aiPM.common.cancel}
      className={className}
    >
      {children}
    </ResponsivePanel>
  )
}