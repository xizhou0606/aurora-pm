import { ResourceToolbar } from '@/features/document-library'
import { ResourceAccessPanel, ResourceAccessRequestsToolbar } from '@/features/resource-access'
import { PhShareNetwork, PhArrowsDownUp } from '@/shared/ui/icons'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_LIBRARY_COPY } from '@/shared/copy/bitables-library'
import { useBitableResourceTools } from '../../hooks/useBitableResourceTools'

export function ResourceTools({ resourceId, bitableId, canEdit, canTransfer, disabled, onTransfer }: {
  resourceId: string
  bitableId: string
  canEdit: boolean
  canTransfer: boolean
  disabled: boolean
  onTransfer: () => void
}) {
  const flow = useBitableResourceTools(resourceId, bitableId)
  return <>
    <ResourceToolbar
      leading={resourceId ? <ResourceAccessRequestsToolbar resourceId={resourceId} resourceType="bitable" canEdit={canEdit} /> : undefined}
      actions={[
        { key: 'share', icon: PhShareNetwork, label: BITABLE_LIBRARY_COPY.share, text: true,
          visible: Boolean(resourceId), onClick: () => flow.setShareOpen(true) },
        { key: 'transfer', icon: PhArrowsDownUp, label: BITABLE_COPY.transfer, text: true,
          visible: canTransfer, disabled, onClick: onTransfer },
      ]} />
    <ResourceAccessPanel purpose="share" open={flow.shareOpen} view={flow.sharing.view}
      onCloseRequest={flow.sharing.onCloseRequest} onOpenChange={flow.setShareOpen} />
  </>
}
