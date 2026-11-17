import { PhDownloadSimple, PhEye } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { resolvePMDocumentHref } from '@/shared/runtime/microApp'
import { useRequirementMaterialMutations } from '../../api/api'
import { useRequirementsOperationAccess } from '../../hooks/useOperationAccess'
import { designMaterialDisplayName, isDesignLinkURL } from '../../utils/workflow-node-materials'
import {
  DeliverableActions,
  DeliverableFilePicker,
  DeliverableLinkField,
  DeliverableMissing,
  DeliverableUploadExtra,
  deliverableActionClass,
  deliverableDisabledActionClass,
  DESIGN_UPLOAD_ACCEPT,
  type DeliverableKind,
  type DeliverablePresentation,
} from './DeliverableComponents'

const poolCopy = copy.aiPM.requirementPool

function deliverableHref(value?: string) {
  const raw = value?.trim() || ''
  return raw ? resolvePMDocumentHref(raw) : ''
}

function deliverableFileName(url: string) {
  try {
    return decodeURIComponent(new URL(url).pathname.split('/').pop() || '') || url
  } catch {
    return url
  }
}

export function poolDesignURLs(requirement: { deliverable_overview?: { design_url?: string; design_urls?: string[]; design_artifacts?: Array<{ id?: number; name: string; content_type?: string; url?: string }> }; design_url?: string }) {
  return poolDesignItems(requirement).map((item) => item.url)
}

export function poolDesignItems(requirement: { deliverable_overview?: { design_url?: string; design_urls?: string[]; design_artifacts?: Array<{ id?: number; name: string; content_type?: string; url?: string }> }; design_url?: string }) {
  const overview = requirement.deliverable_overview
  const fallbackName = poolCopy.deliverableDesign
  if (overview?.design_artifacts?.length) {
    const items = overview.design_artifacts.flatMap((artifact) => {
      const url = (artifact.url || '').trim()
      if (!url) return []
      return [{
        url,
        name: poolDesignItemName(url, artifact.name, artifact.content_type, fallbackName),
      }]
    })
    if (items.length) return items
  }
  const urls = overview?.design_urls?.length
    ? overview.design_urls.map((value) => value.trim()).filter(Boolean)
    : [(overview?.design_url || requirement.design_url || '').trim()].filter(Boolean)
  return urls.map((url) => ({ url, name: poolDesignItemName(url, '', '', fallbackName) }))
}

function poolDesignItemName(url: string, name = '', contentType = '', fallback = poolCopy.deliverableDesign) {
  if (isDesignLinkURL(url) || contentType.toLowerCase() === 'text/uri-list') return url
  return name.trim() || designMaterialDisplayName(url, fallback)
}

export function ExternalDeliverable({
  kind,
  label,
  url,
  urls,
  items,
  presentation = 'chip',
  requirementId,
}: {
  kind: Exclude<DeliverableKind, 'test-cases'>
  label: string
  url?: string
  urls?: string[]
  items?: Array<{ url: string; name: string }>
  presentation?: DeliverablePresentation
  requirementId?: number
}) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const mutations = useRequirementMaterialMutations(workspaceId)
  const access = useRequirementsOperationAccess()
  const resolved = (items?.length
    ? items
    : (urls?.length ? urls : url ? [url] : []).map((value) => ({
      url: value,
      name: designMaterialDisplayName(value, label) || deliverableFileName(value),
    }))
  ).flatMap((item) => {
    const href = deliverableHref(item.url)
    return href ? [{ ...item, href }] : []
  })
  const canUpload = access.canUploadMaterial(kind === 'design' && Boolean(requirementId))
  const upload = (file: File) => {
    if (!requirementId) return
    void mutations.upload.mutateAsync({ requirementId, kind: 'design', file, source: 'requirement' })
  }
  const uploadLink = (link: string) => {
    if (!requirementId) return
    void mutations.upload.mutateAsync({ requirementId, kind: 'design', url: link, source: 'requirement' })
  }
  const uploadExtras = (close: () => void, open: () => void) => canUpload ? (
    <>
      <DeliverableUploadExtra
        label={label}
        pending={mutations.upload.isPending}
        onPick={() => { close(); open() }}
      />
      <DeliverableLinkField pending={mutations.upload.isPending} onSave={(link) => { close(); uploadLink(link) }} />
    </>
  ) : null
  if (!resolved.length) {
    if (canUpload) {
      return (
        <div className="flex w-full justify-center">
          <DeliverableFilePicker accept={DESIGN_UPLOAD_ACCEPT} disabled={mutations.upload.isPending} onPick={upload}>
            {(open) => (
              <DeliverableActions
                kind={kind}
                label={label}
                presentation={presentation}
                triggerLabel={poolCopy.deliverableUpload(label)}
                className="grid min-w-56 gap-1 rounded-md p-1.5"
              >
                {(close) => uploadExtras(close, open)}
              </DeliverableActions>
            )}
          </DeliverableFilePicker>
        </div>
      )
    }
    return <div className="flex w-full justify-center"><DeliverableMissing kind={kind} label={label} presentation={presentation} /></div>
  }
  const many = resolved.length > 1 || kind === 'design'
  return (
    <div className="flex w-full min-w-0 items-center justify-center">
      <DeliverableFilePicker accept={DESIGN_UPLOAD_ACCEPT} disabled={!canUpload || mutations.upload.isPending} onPick={upload}>
        {(open) => (
          <DeliverableActions kind={kind} label={label} presentation={presentation} className={many || canUpload ? 'grid min-w-72 max-w-sm gap-1 rounded-md p-1.5' : undefined}>
            {(close) => (
              <>
                {resolved.map((item) => {
                  const previewLabel = many ? poolCopy.deliverableOpenFile(label, item.name) : poolCopy.deliverablePreview(label)
                  return (
                    <div key={item.href} className={many ? 'flex min-w-0 items-center gap-1' : 'contents'}>
                      {many ? <span className="min-w-0 flex-1 truncate px-1 text-xs text-muted-foreground" title={item.name}>{item.name}</span> : null}
                      {access.canDownloadMaterial() ? <a
                        href={item.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        aria-label={previewLabel}
                        title={previewLabel}
                        className={deliverableActionClass}
                        onClick={close}
                      >
                        <PhEye aria-hidden="true" />
                      </a> : null}
                      <span
                        role="button"
                        aria-disabled="true"
                        aria-label={poolCopy.deliverableDownloadUnavailable(label)}
                        title={poolCopy.deliverableDownloadUnavailable(label)}
                        className={deliverableDisabledActionClass}
                      >
                        <PhDownloadSimple aria-hidden="true" />
                      </span>
                    </div>
                  )
                })}
                {uploadExtras(close, open)}
              </>
            )}
          </DeliverableActions>
        )}
      </DeliverableFilePicker>
    </div>
  )
}