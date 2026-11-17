import { useEffect, useRef, useState } from 'react'
import type { GridProps } from '../../model/grid-port'
import type { CellAddress } from '../../model/types'

export function useAttachmentThumbnail(address: CellAddress, assetId: string, image: boolean, load: GridProps['onLoadAttachmentPreview']) {
  const ref = useRef<HTMLSpanElement>(null)
  const key = `${address.recordId}:${address.fieldId}:${assetId}`
  const [preview, setPreview] = useState<{ key: string; load: typeof load; source: string } | null>(null)
  useEffect(() => {
    let active = true, started = false, objectUrl = ''
    setPreview(null)
    if (!image || !load) return
    const start = () => {
      if (started) return
      started = true
      void load({ recordId: address.recordId, fieldId: address.fieldId }, assetId).then(blob => {
        // A scoped loader can be replaced after permission or session changes. Late responses never render.
        if (!active || !blob || !blob.type.startsWith('image/')) return
        objectUrl = URL.createObjectURL(blob); setPreview({ key, load, source: objectUrl })
      }).catch(() => { /* File chip remains the safe fallback. */ })
    }
    const observer = typeof IntersectionObserver === 'undefined' ? null : new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) { observer?.disconnect(); start() }
    })
    if (observer && ref.current) observer.observe(ref.current)
    else start()
    return () => { active = false; observer?.disconnect(); if (objectUrl) URL.revokeObjectURL(objectUrl) }
  }, [address.recordId, address.fieldId, assetId, image, load, key])
  return { ref, source: image && preview?.key === key && preview.load === load ? preview.source : '' }
}
