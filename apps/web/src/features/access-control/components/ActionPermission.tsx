import type { ReactNode } from 'react'

import type { PMActionCode } from '@/generated/access-catalog'
import { useCanAction } from '../hooks/hooks'

export type ActionPermissionProps = {
  code: PMActionCode
  children: ReactNode
  fallback?: ReactNode
}

export function ActionPermission({ code, children, fallback = null }: ActionPermissionProps) {
  return useCanAction(code) ? children : fallback
}
