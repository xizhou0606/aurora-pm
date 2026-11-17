import { useMemo } from 'react'

import { PM_ACTION } from '@/generated/access-catalog'
import {
  type DocumentAction,
  type DocumentCapabilities,
} from '../model/types'

export function resolveDocumentCapabilities(
  allowedActions: readonly DocumentAction[] | undefined,
): DocumentCapabilities {
  const allowed = new Set<DocumentAction>(allowedActions ?? [])
  return {
    canView: allowed.has(PM_ACTION.DocumentsContentView),
    canExport: allowed.has(PM_ACTION.DocumentsContentView) && allowed.has(PM_ACTION.DocumentsContentExport),
    canUpdate: allowed.has(PM_ACTION.DocumentsContentUpdate),
    canViewAssets: allowed.has(PM_ACTION.DocumentsAssetsView),
    canUploadAssets: allowed.has(PM_ACTION.DocumentsAssetsUpload),
    canDownloadAssets: allowed.has(PM_ACTION.DocumentsAssetsDownload),
    canDeleteAssets: allowed.has(PM_ACTION.DocumentsAssetsDelete),
    canViewComments: allowed.has(PM_ACTION.DocumentsCommentsView),
    canCreateComments: allowed.has(PM_ACTION.DocumentsCommentsCreate),
    canReplyComments: allowed.has(PM_ACTION.DocumentsCommentsReply),
  }
}

export function useDocumentCapabilities(
  allowedActions: readonly DocumentAction[] | undefined,
): DocumentCapabilities {
  return useMemo(() => resolveDocumentCapabilities(allowedActions), [allowedActions])
}
