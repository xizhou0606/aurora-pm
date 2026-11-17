import { LazyRichTextField, isRichTextEmpty } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { useWorkflowMutations } from '../api/api'
import { useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const subjectCopy = copy.aiPM.workflowSubject

export function workflowRichTextEmpty(value: string) {
  return isRichTextEmpty(value, 'html')
}

function sanitizeOpinionHTML(value: string) {
  const document = new DOMParser().parseFromString(value, 'text/html')
  document.querySelectorAll('script,iframe,object,embed,link,meta,style').forEach((node) => node.remove())
  document.querySelectorAll('*').forEach((node) => {
    for (const attribute of Array.from(node.attributes)) {
      const name = attribute.name.toLowerCase()
      const content = attribute.value.trim().toLowerCase()
      if (name.startsWith('on') || content.startsWith('javascript:')) node.removeAttribute(attribute.name)
    }
    if (node.tagName !== 'IMG') return
    const src = node.getAttribute('src') || ''
    if (!/^https?:\/\//i.test(src)) node.remove()
  })
  return document.body.innerHTML
}

export function WorkflowRichTextBody({
  value,
  emptyLabel,
}: {
  value?: string
  emptyLabel?: string
}) {
  const source = value?.trim() || ''
  if (!source || workflowRichTextEmpty(source)) {
    return <p className="m-0 text-sm leading-6 text-muted-foreground">{emptyLabel || ''}</p>
  }
  if (!/<[a-z][\s\S]*>/i.test(source)) {
    return <p className="m-0 whitespace-pre-wrap break-words text-sm leading-6 text-foreground">{source}</p>
  }
  return (
    <div
      className="min-w-0 text-sm leading-5 text-foreground [&_p]:my-0 [&_ul]:my-1 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-1 [&_ol]:list-decimal [&_ol]:pl-5 [&_img]:my-1.5 [&_img]:max-h-72 [&_img]:max-w-full [&_img]:rounded-md [&_img]:object-contain"
      dangerouslySetInnerHTML={{ __html: sanitizeOpinionHTML(source) }}
    />
  )
}

export function WorkflowRichTextField({
  workspaceId,
  instanceId,
  nodeId,
  label,
  value,
  onChange,
  disabled,
  pending,
  placeholder,
  documentId,
  minHeight = 88,
}: {
  workspaceId: number
  instanceId: number
  nodeId: string
  label: string
  value: string
  onChange?: (value: string) => void
  disabled?: boolean
  pending?: boolean
  placeholder?: string
  documentId?: string | number
  minHeight?: number
}) {
  const mutations = useWorkflowMutations(workspaceId)
  const access = useWorkflowOperationAccess()
  const uploadDisabled = disabled || !access.canUploadOpinionImage()
  return (
    <div className="grid gap-1.5">
      <LazyRichTextField
        label={label}
        value={value}
        documentId={documentId}
        scope={`${workspaceId}:${instanceId}:${nodeId}`}
        serialization="html"
        visuallyHiddenLabel
        disabled={disabled}
        pending={pending}
        minHeight={minHeight}
        placeholder={placeholder}
        onChange={onChange}
        onUploadImage={uploadDisabled ? undefined : async (file) => {
          const uploaded = await mutations.uploadOpinionImage.mutateAsync({ instanceId, nodeId, file })
          return uploaded.url
        }}
      />
      {uploadDisabled ? null : <p className="m-0 text-xs leading-4 text-muted-foreground">{subjectCopy.opinionImageHint}</p>}
    </div>
  )
}
