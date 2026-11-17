import { useEffect, useState } from 'react'

export function useObjectURL(blob?: Blob): string | undefined {
  const [url, setURL] = useState<string>()
  useEffect(() => {
    if (!blob || typeof URL.createObjectURL !== 'function') {
      setURL(undefined)
      return
    }
    const next = URL.createObjectURL(blob)
    setURL(next)
    return () => URL.revokeObjectURL(next)
  }, [blob])
  return url
}
