import {
  forwardRef,
  useEffect,
  useState,
  type HTMLAttributes,
} from 'react'

import { cn } from '@/shared/lib/utils'

export type UserPreviewProps = HTMLAttributes<HTMLSpanElement> & {
  name: string
  nameSuffix?: string
  secondary?: string
  avatarUrl?: string
  size?: 'sm' | 'md'
  showName?: boolean
}

function initial(name: string): string {
  return [...name.trim()][0]?.toLocaleUpperCase() || '·'
}

export const UserPreview = forwardRef<HTMLSpanElement, UserPreviewProps>(
  ({ name, nameSuffix, secondary, avatarUrl, size = 'md', showName = true, className, ...props }, ref) => {
    const [avatarFailed, setAvatarFailed] = useState(false)

    useEffect(() => {
      setAvatarFailed(false)
    }, [avatarUrl])

    return (
      <span
        ref={ref}
        data-slot="user-preview"
        data-size={size}
        className={cn(
          'inline-flex min-w-0 max-w-full items-center align-middle',
          size === 'sm' ? 'gap-1.5' : 'gap-2',
          className,
        )}
        {...props}
      >
        <span
          aria-hidden="true"
          data-slot="user-preview-avatar"
          className={cn(
            'grid shrink-0 place-items-center overflow-hidden rounded-full bg-muted font-semibold text-muted-foreground',
            size === 'sm' ? 'size-5 text-xs' : 'size-7 text-xs',
          )}
        >
          {avatarUrl && !avatarFailed ? (
            <img
              src={avatarUrl}
              alt=""
              className="size-full object-cover"
              onError={() => setAvatarFailed(true)}
            />
          ) : (
            <span
              data-slot="user-preview-initial"
              data-initial={initial(name)}
              className="before:content-[attr(data-initial)]"
            />
          )}
        </span>
        {showName ? (
          <span className="min-w-0 flex-1 overflow-hidden">
            <span className={cn(
              'block truncate text-foreground',
              size === 'sm' ? 'text-xs font-normal' : 'text-sm font-medium',
            )} title={nameSuffix ? `${name}${nameSuffix}` : undefined}>
              {name}
              {nameSuffix ? <span className="text-xs font-normal text-muted-foreground">{nameSuffix}</span> : null}
            </span>
            {secondary && size === 'md' ? (
              <span className="block truncate text-xs text-muted-foreground">{secondary}</span>
            ) : null}
          </span>
        ) : null}
      </span>
    )
  },
)
UserPreview.displayName = 'UserPreview'
