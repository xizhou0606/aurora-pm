import { PhCheck, PhLink } from '@/shared/ui/icons'
import { useEffect, useState } from 'react'

import { copy } from '@/shared/copy'
import { resolvePMCurrentShareHref } from '@/shared/runtime'
import { Button } from '../primitives/button'

type CopyState = 'idle' | 'copied' | 'error'

export type CopyLinkButtonProps = {
  path: string
  iconOnly?: boolean
}

export function resolveInternalShareHref(path: string): string {
  const currentHref = resolvePMCurrentShareHref()
  if (!currentHref) throw new Error('unsupported_share_link')
  const target = new URL(path, 'https://pm.internal')
  if (target.origin !== 'https://pm.internal' || !target.pathname.startsWith('/workspace/')) {
    throw new Error('unsupported_share_path')
  }
  const current = new URL(currentHref)
  if (current.protocol !== 'http:' && current.protocol !== 'https:') {
    throw new Error('unsupported_share_link')
  }
  const workspaceIndex = current.pathname.indexOf('/workspace/')
  const hostBasePath = workspaceIndex >= 0
    ? current.pathname.slice(0, workspaceIndex).replace(/\/+$/, '')
    : ''
  current.pathname = `${hostBasePath}${target.pathname}`
  current.search = target.search
  current.hash = target.hash
  return current.toString()
}

async function writeClipboard(value: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }
  const field = document.createElement('textarea')
  field.value = value
  field.readOnly = true
  field.setAttribute('aria-hidden', 'true')
  field.style.position = 'fixed'
  field.style.opacity = '0'
  document.body.append(field)
  try {
    field.select()
    if (!document.execCommand('copy')) throw new Error('clipboard_fallback_failed')
  } finally {
    field.remove()
  }
}

export async function copyInternalPath(path: string): Promise<void> {
  await writeClipboard(resolveInternalShareHref(path))
}

export function CopyLinkButton({ path, iconOnly = false }: CopyLinkButtonProps) {
  const [state, setState] = useState<CopyState>('idle')
  useEffect(() => setState('idle'), [path])
  const label = state === 'copied'
    ? copy.ui.linkCopied
    : state === 'error'
      ? copy.ui.copyLinkFailed
      : copy.ui.copyLink

  const copyLink = async () => {
    try {
      await copyInternalPath(path)
      setState('copied')
    } catch {
      setState('error')
    }
  }

  return (
    <Button
      type="button"
      size={iconOnly ? 'icon-xs' : 'sm'}
      variant={iconOnly ? 'ghost' : 'outline'}
      aria-label={iconOnly ? label : undefined}
      title={iconOnly ? label : undefined}
      aria-live="polite"
      onClick={() => void copyLink()}
    >
      {state === 'copied' ? <PhCheck aria-hidden="true" /> : <PhLink aria-hidden="true" />}
      <span className={iconOnly ? 'sr-only' : undefined}>{label}</span>
    </Button>
  )
}
