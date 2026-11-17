import { useState } from 'react'

import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  EmptyState,
  Input,
  LoadingSkeleton,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
} from '@/shared/ui'

import type { ResourceAccessLevel, ResourceShareLinksControl, ShareLinkExpiry } from '../model'

function dateLabel(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

export function ResourceShareLinksSection({ control }: { control: ResourceShareLinksControl }) {
  const [revokeId, setRevokeId] = useState<string | null>(null)
  if (control.status === 'loading') {
    return <LoadingSkeleton rows={3} label={copy.links.loading} />
  }
  if (control.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={copy.links.errorTitle}
        action={<Button type="button" variant="outline" onClick={control.onRetry}>{copy.panel.retry}</Button>}
      />
    )
  }
  return (
    <section className="grid gap-4" aria-label={copy.links.title}>
      <p className="m-0 text-xs text-muted-foreground">{control.links.length > 1 ? copy.links.legacyLinksHint : copy.links.singleLinkHint}</p>
      {control.operationError ? (
        <AlertBanner tone="error">{copy.links.operationError}</AlertBanner>
      ) : null}
      {control.secretHref ? (
        <AlertBanner tone="success" onDismiss={control.onDismissSecret}>
          <div className="grid gap-2">
            <p className="m-0 font-medium">{copy.links.secretTitle}</p>
            <p className="m-0">{copy.links.secretDescription}</p>
            <Input readOnly value={control.secretHref} />
            <Button type="button" variant="outline" onClick={control.onCopy}>
              {control.copied ? copy.links.copied : copy.links.copy}
            </Button>
          </div>
        </AlertBanner>
      ) : null}
      {control.canManage ? (
        <div className="grid gap-2 rounded-lg border border-border bg-muted/30 p-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <Select
            value={control.level}
            disabled={control.pending}
            onValueChange={(value) => {
              if (value === 'viewer' || value === 'editor') control.onLevelChange(value as ResourceAccessLevel)
            }}
          >
            <SelectTrigger aria-label={copy.level.label}><SelectValue>{copy.level[control.level]}</SelectValue></SelectTrigger>
            <SelectContent>
              <SelectItem value="viewer">{copy.level.viewer}</SelectItem>
              <SelectItem value="editor">{copy.level.editor}</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={control.expiry}
            disabled={control.pending}
            onValueChange={(value) => {
              if (value === '24h' || value === '7d' || value === 'never') control.onExpiryChange(value)
            }}
          >
            <SelectTrigger aria-label={copy.links.expiryLabel}><SelectValue>{({ '24h': copy.links.expiry24h, '7d': copy.links.expiry7d, never: copy.links.expiryNever } satisfies Record<ShareLinkExpiry, string>)[control.expiry]}</SelectValue></SelectTrigger>
            <SelectContent>
              <SelectItem value="24h">{copy.links.expiry24h}</SelectItem>
              <SelectItem value="7d">{copy.links.expiry7d}</SelectItem>
              <SelectItem value="never">{copy.links.expiryNever}</SelectItem>
            </SelectContent>
          </Select>
          <Button type="button" disabled={control.pending || control.links.length > 1} onClick={control.onCreate}>
            {control.pending ? copy.links.saving : control.links.length > 0 ? copy.links.update : copy.links.create}
          </Button>
        </div>
      ) : null}
      {control.links.length === 0 ? (
        <EmptyState title={copy.links.empty} name="hugeicons:link-01" />
      ) : (
        <div className="grid gap-2">
          {control.links.map((link) => (
            <div key={link.id} className="flex flex-col gap-3 rounded-lg border border-border p-3 sm:flex-row sm:items-center">
              <div className="min-w-0 flex-1">
                <StatusPill tone={link.revokedAt ? 'neutral' : 'info'}>
                  {link.revokedAt ? copy.links.revoked : copy.level[link.level]}
                </StatusPill>
                <p className="m-0 mt-1 text-xs text-muted-foreground">
                  {link.expiresAt ? copy.links.expiresAt(dateLabel(link.expiresAt)) : copy.links.neverExpires}
                </p>
              </div>
              {control.canManage && !link.revokedAt ? (
                <div className="flex gap-2">
                  <Button type="button" variant="destructive" disabled={control.pending} onClick={() => setRevokeId(link.id)}>
                    {copy.links.revoke}
                  </Button>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      )}
      <ConfirmDialog
        open={Boolean(revokeId)}
        title={copy.links.revokeTitle}
        description={copy.links.revokeDescription}
        confirmLabel={copy.links.revokeConfirm}
        cancelLabel={copy.panel.cancel}
        confirmVariant="destructive"
        pending={control.pending}
        onOpenChange={(open) => { if (!open) setRevokeId(null) }}
        onConfirm={() => {
          if (revokeId) control.onRevoke(revokeId)
          setRevokeId(null)
        }}
      />
    </section>
  )
}
