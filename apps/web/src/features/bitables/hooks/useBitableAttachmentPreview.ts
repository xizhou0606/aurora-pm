import { useCallback, useEffect, useRef, useState } from 'react'
import { toSafeApiError } from '@/shared/api'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { downloadBlob } from '@/shared/lib/download-blob'
import type { TableScope } from '../api/client'
import type { AttachmentRef, CellAddress, Row } from '../model/types'
import { bitableKey, currentEnvironment, type BitableEnvironment } from './environment'

export function useBitableAttachmentPreview(env: BitableEnvironment, scope: TableScope, rows: readonly Row[], canRead: boolean) {
  const identity = JSON.stringify(bitableKey(env, scope))
  const latest = useRef({ identity, canRead, rows }); latest.current = { identity, canRead, rows }
  const generation = useRef(0), objectUrl = useRef<string | null>(null)
  const [preview, setPreview] = useState<{ url: string; title: string } | null>(null)
  const [pending, setPending] = useState(false), [error, setError] = useState('')
  const release = () => { if (objectUrl.current) URL.revokeObjectURL(objectUrl.current); objectUrl.current = null }
  const close = () => { generation.current += 1; release(); setPreview(null); setPending(false) }
  const valid = () => latest.current.identity === identity && latest.current.canRead && currentEnvironment(env)
  const attachment = (address: CellAddress, id: string) => {
    const values = latest.current.rows.find(row => row.id === address.recordId)?.values[address.fieldId]
    return Array.isArray(values) ? values.find((value): value is AttachmentRef => typeof value === 'object' && value.id === id && 'downloadPath' in value) : undefined
  }
  useEffect(() => { close(); setError(''); return () => { generation.current += 1; release() } }, [identity, canRead])
  const open = async (address: CellAddress, id: string) => {
    const asset = attachment(address, id)
    if (!valid() || !asset || !asset.mimeType.startsWith('image/')) return
    close(); const ticket = generation.current; setPending(true); setError('')
    try {
      const blob = await env.client.asset(scope, address, id, 'preview')
      if (!valid() || generation.current !== ticket) return
      if (!blob.type.startsWith('image/')) throw new Error('BITABLE_ASSET_TYPE_INVALID')
      const url = URL.createObjectURL(blob); objectUrl.current = url
      setPreview({ url, title: asset.name })
    } catch (caught) {
      if (valid() && generation.current === ticket) setError(toSafeApiError(caught).status === 403 ? BITABLE_COPY.forbidden : BITABLE_COPY.error)
    } finally { if (valid() && generation.current === ticket) setPending(false) }
  }
  const download = async (address: CellAddress, id: string) => {
    const asset = attachment(address, id), ticket = generation.current
    if (!valid() || !asset) return
    try {
      const blob = await env.client.asset(scope, address, id, 'download')
      if (valid() && generation.current === ticket) downloadBlob(blob, asset.name)
    } catch (caught) { if (valid() && generation.current === ticket) setError(toSafeApiError(caught).status === 403 ? BITABLE_COPY.forbidden : BITABLE_COPY.error) }
  }
  const loadThumbnail = useCallback(async (address: CellAddress, id: string) => {
    if (!canRead || latest.current.identity !== identity || !latest.current.canRead || !currentEnvironment(env)) return null
    const values = latest.current.rows.find(row => row.id === address.recordId)?.values[address.fieldId]
    const asset = Array.isArray(values) ? values.find((value): value is AttachmentRef => typeof value === 'object' && value.id === id && 'downloadPath' in value) : undefined
    if (!asset?.mimeType.startsWith('image/')) return null
    const blob = await env.client.asset(scope, address, id, 'preview')
    if (latest.current.identity !== identity || !latest.current.canRead || !currentEnvironment(env)) return null
    return blob.type.startsWith('image/') ? blob : null
  }, [env, scope, identity, canRead])
  return { preview: canRead ? preview : null, pending, error, open, close, download, loadThumbnail }
}
