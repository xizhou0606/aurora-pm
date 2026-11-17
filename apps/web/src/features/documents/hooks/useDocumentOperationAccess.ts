import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import type { DocumentTreeCapabilities } from '../model'

export type DocumentOperationAccess = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
  canViewTree: boolean
  tree: DocumentTreeCapabilities
}

export function useDocumentOperationAccess(): DocumentOperationAccess {
  const operations = useOperationCapabilities('documents')
  return useMemo(() => {
    const scopeAll = operations.allows(PM_ACTION.DocumentsScopeAll)
    return {
      status: operations.status,
      retry: operations.retry,
      canViewTree: operations.allows(PM_ACTION.DocumentsNodesView),
      tree: {
        canCreate: scopeAll && operations.allows(PM_ACTION.DocumentsNodesCreate),
        canImportMarkdown: scopeAll
          && operations.allows(PM_ACTION.DocumentsNodesCreate)
          && operations.allows(PM_ACTION.DocumentsContentUpdate),
        canImportMarkdownAssets: scopeAll
          && operations.allows(PM_ACTION.DocumentsNodesCreate)
          && operations.allows(PM_ACTION.DocumentsContentUpdate)
          && operations.allows(PM_ACTION.DocumentsAssetsUpload),
        canCopy: scopeAll && operations.allows(PM_ACTION.DocumentsNodesCopy),
        canCreateShortcut: scopeAll && operations.allows(PM_ACTION.DocumentsShortcutsCreate),
        canUpdatePreferences: operations.allows(PM_ACTION.DocumentsPreferencesUpdate),
        canTransferOwnership: scopeAll && operations.allows(PM_ACTION.DocumentsOwnershipTransfer),
        canRename: scopeAll && operations.allows(PM_ACTION.DocumentsNodesUpdate),
        canMove: scopeAll && operations.allows(PM_ACTION.DocumentsNodesMove),
        canArchive: scopeAll && operations.allows(PM_ACTION.DocumentsNodesArchive),
      },
    }
  }, [operations.allows, operations.retry, operations.status])
}
