import { describe, expect, it } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { resolveDocumentCapabilities } from './useDocumentCapabilities'

describe('document object capabilities', () => {
  it('derives leaf booleans only from server allowed_actions', () => {
    expect(resolveDocumentCapabilities([
      PM_ACTION.DocumentsContentView,
      PM_ACTION.DocumentsAssetsView,
      PM_ACTION.DocumentsAssetsDownload,
    ])).toEqual({
      canView: true,
      canExport: false,
      canUpdate: false,
      canViewAssets: true,
      canUploadAssets: false,
      canDownloadAssets: true,
      canDeleteAssets: false,
      canViewComments: false,
      canCreateComments: false,
      canReplyComments: false,
    })
  })
  it('requires both view and the explicit export action', () => {
    expect(resolveDocumentCapabilities([PM_ACTION.DocumentsContentExport]).canExport).toBe(false)
    expect(resolveDocumentCapabilities([PM_ACTION.DocumentsContentView, PM_ACTION.DocumentsContentExport]).canExport).toBe(true)
  })
})
