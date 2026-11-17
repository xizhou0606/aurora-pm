import { useEffect, useRef, useState } from 'react'

import { IconBadge } from '@/shared/ui'
import { PhFileText } from '@/shared/ui/icons'

export function DocumentAssetThumbnail({
  assetId,
  fileName,
  load,
}: {
  assetId: number
  fileName: string
  load?: (assetId: number) => Promise<Blob | null>
}) {
  const containerRef = useRef<HTMLSpanElement>(null)
  const [source, setSource] = useState('')
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setSource('')
    setFailed(false)
    if (!load) return
    let disposed = false
    let objectUrl = ''
    const resolve = async () => {
      const blob = await load(assetId)
      if (disposed || !blob) return
      objectUrl = URL.createObjectURL(blob)
      setSource(objectUrl)
    }
    const observer = typeof IntersectionObserver === 'undefined' ? undefined
      : new IntersectionObserver((entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return
        observer?.disconnect()
        void resolve()
      }, { rootMargin: '120px' })
    if (observer && containerRef.current) observer.observe(containerRef.current)
    else void resolve()
    return () => {
      disposed = true
      observer?.disconnect()
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [assetId, load])

  return (
    <span ref={containerRef} className="flex size-full items-center justify-center overflow-hidden bg-muted/40">
      {source && !failed ? (
        <img src={source} alt={fileName} decoding="async" className="size-full object-contain p-1" onError={() => setFailed(true)} />
      ) : (
        <IconBadge icon={PhFileText} tone="neutral" variant="soft" size="lg" />
      )}
    </span>
  )
}
