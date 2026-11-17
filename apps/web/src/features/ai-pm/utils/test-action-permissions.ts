import { seedActionPermissions as seedPermissions } from '@/features/access-control'
import type { PMActionCode } from '@/generated/access-catalog'

export function seedActionPermissions(actionCodes: readonly PMActionCode[]) {
  seedPermissions(actionCodes)
}
