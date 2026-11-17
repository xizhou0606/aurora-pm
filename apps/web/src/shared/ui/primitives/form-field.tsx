import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'
import { Label } from './label'

export type FormFieldControlProps = {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
}

export type FormFieldProps = {
  id: string
  label: ReactNode
  showLabel?: boolean
  className?: string
  hint?: string
  error?: string
  describedBy?: string
  children: ReactNode | ((control: FormFieldControlProps) => ReactNode)
}

// Presentation reference: https://coss.com/ui/docs/components/field (2026-09-21).
// Explicit control props also support composite controls without implicit registration.
export function FormField({ id, label, showLabel = true, className, hint, error, describedBy, children }: FormFieldProps) {
  const description = [describedBy, hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined
  const control: FormFieldControlProps = {
    id,
    'aria-describedby': description,
    'aria-invalid': error ? true : undefined,
  }
  return (
    <div data-slot="form-field" className={cn('grid min-w-0', (showLabel || hint || error) && 'gap-1.5', className)}>
      <Label htmlFor={id} className={showLabel ? undefined : 'sr-only'}>{label}</Label>
      {typeof children === 'function' ? children(control) : children}
      {hint ? <p id={`${id}-hint`} className="m-0 text-xs text-muted-foreground">{hint}</p> : null}
      {error ? <p id={`${id}-error`} role="alert" className="m-0 text-xs text-destructive">{error}</p> : null}
    </div>
  )
}
