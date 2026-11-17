import { cn } from '@/shared/lib/utils'

const BOWL_PATH =
  'M11.8 6.1 C18.6 6.1 23.6 10.2 23.6 15.2 C23.6 19.4 20.6 22.8 16.1 23.6'

export function BrandMark({
  className,
  size = 32,
}: {
  className?: string
  size?: number
}) {
  return (
    <span
      data-slot="brand-mark"
      className={cn(
        'relative inline-grid size-(--brand-mark-size) shrink-0 place-items-center',
        className,
      )}
      style={{ '--brand-mark-size': `${size}px` }}
    >
      <svg viewBox="0 0 32 32" aria-hidden="true" className="size-full overflow-visible">
        <rect
          data-lane="stem"
          x="6.2"
          y="4"
          width="5.6"
          height="24"
          rx="1.6"
          fill="currentColor"
        />
        <path
          data-lane="bowl"
          d={BOWL_PATH}
          fill="none"
          stroke="currentColor"
          strokeWidth="5.6"
          strokeLinecap="round"
          pathLength="100"
        />
        <circle data-lane="rest" cx="16.1" cy="23.6" r="1.6" fill="#f4f6ff" />
        <circle data-lane="node" r="1.6" fill="#f4f6ff">
          <animateMotion dur="2.6s" repeatCount="indefinite" path={BOWL_PATH} />
        </circle>
      </svg>
    </span>
  )
}

export function BrandLockup({
  name,
  kicker,
  className,
  kickerClassName,
}: {
  name: string
  kicker?: string
  className?: string
  kickerClassName?: string
}) {
  return (
    <span data-slot="brand-lockup" className={cn('inline-flex items-center gap-2.5', className)}>
      <BrandMark />
      <span className="flex items-baseline gap-1.5 leading-none">
        <strong className="text-[18px] font-semibold tracking-tight">{name}</strong>
        {kicker ? (
          <span
            data-slot="brand-kicker"
            className={cn('text-sm font-medium tracking-[0.04em]', kickerClassName)}
          >
            {kicker}
          </span>
        ) : null}
      </span>
    </span>
  )
}
