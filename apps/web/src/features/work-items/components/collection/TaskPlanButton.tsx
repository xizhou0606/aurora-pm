import { PhSparkle } from '@/shared/ui/icons'

import { Button } from '@/shared/ui'

export function TaskPlanButton({
  title,
  onClick,
  disabled = false,
  disabledReason,
  withLabel = false,
}: {
  title: string
  onClick: () => void
  disabled?: boolean
  disabledReason?: string
  withLabel?: boolean
}) {
  return (
    <Button
      type="button"
      variant="outline"
      size={withLabel ? 'sm' : 'icon-sm'}
      disabled={disabled}
      title={disabled ? disabledReason : title}
      aria-label={title}
      onClick={onClick}
    >
      <PhSparkle aria-hidden="true" />
      {withLabel ? 'AI 拆分任务' : null}
    </Button>
  )
}
