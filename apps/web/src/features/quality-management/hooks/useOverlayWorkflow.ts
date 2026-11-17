import { useCallback, useEffect, useState } from 'react'

type Resettable = { reset: () => void }

export function useQualityOverlayWorkflow({
  workspaceId,
  bugId,
  edit,
  assign,
  resolve,
  returnBug,
  transition,
  remove,
  upload,
}: {
  workspaceId: number
  bugId?: number
  edit: Resettable
  assign: Resettable
  resolve: Resettable
  returnBug: Resettable
  transition: Resettable
  remove: Resettable
  upload: Resettable
}) {
  const [assignOpen, setAssignOpen] = useState(false)
  const [resolveOpen, setResolveOpen] = useState(false)
  const [returnOpen, setReturnOpen] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  useEffect(() => {
    setAssignOpen(false)
    setResolveOpen(false)
    setReturnOpen(false)
    setPreviewUrl(null)
    edit.reset()
    assign.reset()
    resolve.reset()
    returnBug.reset()
    transition.reset()
    remove.reset()
    upload.reset()
  }, [
    assign.reset,
    bugId,
    edit.reset,
    remove.reset,
    resolve.reset,
    returnBug.reset,
    transition.reset,
    upload.reset,
    workspaceId,
  ])

  const changeAssignOpen = useCallback((open: boolean) => {
    if (!open) assign.reset()
    setAssignOpen(open)
  }, [assign.reset])
  const changeResolveOpen = useCallback((open: boolean) => {
    if (!open) {
      resolve.reset()
      upload.reset()
    }
    setResolveOpen(open)
  }, [resolve.reset, upload.reset])
  const changeReturnOpen = useCallback((open: boolean) => {
    if (!open) {
      returnBug.reset()
      upload.reset()
    }
    setReturnOpen(open)
  }, [returnBug.reset, upload.reset])

  return {
    assignOpen,
    resolveOpen,
    returnOpen,
    previewUrl,
    setAssignOpen: changeAssignOpen,
    setResolveOpen: changeResolveOpen,
    setReturnOpen: changeReturnOpen,
    setPreviewUrl,
  }
}
