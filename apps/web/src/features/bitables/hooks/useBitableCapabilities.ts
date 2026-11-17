import { useMemo } from 'react'
import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import type { Capabilities } from '../model/types'

export const noCapabilities: Capabilities = {
  canRead: false, canCreateRecords: false, canUpdateRecords: false, canDeleteRecords: false,
  canManageFields: false, canManageViews: false, canImport: false, canExport: false,
  canUploadAttachments: false, writableFieldIds: [],
}
export function useBitableCapabilities(server: Capabilities | undefined, readOnly: boolean) {
  const access = useOperationCapabilities('bitables')
  return useMemo(() => {
    const read = access.allows(PM_ACTION.BitablesResourcesView)
    const write = read && !readOnly && access.allows(PM_ACTION.BitablesRecordsWrite)
    const b = server ?? noCapabilities
    return { accessStatus: access.status, retry: access.retry, canList: read,
      canCreate: read && !readOnly && access.allows(PM_ACTION.BitablesResourcesCreate),
      capabilities: {
        canRead: read && b.canRead,
        canComment: read && !readOnly && Boolean(b.canComment) && access.allows(PM_ACTION.BitablesCommentsCreate),
        canCreateRecords: write && b.canCreateRecords, canUpdateRecords: write && b.canUpdateRecords,
        canDeleteRecords: write && b.canDeleteRecords,
        canManageFields: read && !readOnly && b.canManageFields && access.allows(PM_ACTION.BitablesFieldsManage),
        canManageViews: read && !readOnly && b.canManageViews && access.allows(PM_ACTION.BitablesViewsManage),
        canImport: write && b.canImport && access.allows(PM_ACTION.BitablesTransferImport),
        canExport: read && b.canExport && access.allows(PM_ACTION.BitablesTransferExport),
        canUploadAttachments: write && b.canUploadAttachments && access.allows(PM_ACTION.BitablesAssetsUpload),
        writableFieldIds: write ? b.writableFieldIds : [],
      },
    }
  }, [access.allows, access.status, access.retry, server, readOnly])
}
