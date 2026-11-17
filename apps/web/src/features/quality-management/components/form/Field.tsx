import { type ReactNode } from 'react'

import { Label } from '@/shared/ui'

export function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="grid min-w-0 gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  )
}