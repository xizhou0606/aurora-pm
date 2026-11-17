import type { ReactNode } from 'react'

import { DetailSection } from '@/shared/ui'

export function AccountSection({
  children,
  description,
  title,
}: {
  children: ReactNode
  description: ReactNode
  title: ReactNode
}) {
  return (
    <DetailSection title={title} description={description} className="gap-5 p-5 shadow-xs sm:p-6">
      {children}
    </DetailSection>
  )
}
