import { cn } from '@/shared/lib/utils'

type LoadingOverlayProps = {
  label: string
  className?: string
}

function LoadingOverlay({
  label,
  className,
}: LoadingOverlayProps) {
  return (
    <div
      data-slot="loading-overlay"
      role="status"
      aria-busy="true"
      aria-label={label}
      className={cn(
        'pm-loading-overlay-reveal absolute inset-0 z-30 flex items-start justify-center bg-background/75 pt-6',
        className,
      )}
    >
      <span className="flex items-center gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm text-muted-foreground shadow-sm">
        <span
          data-slot="loading-overlay-spinner"
          aria-hidden="true"
          className="size-4 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
        />
        {label}
      </span>
    </div>
  )
}

export { LoadingOverlay }
export type { LoadingOverlayProps }
