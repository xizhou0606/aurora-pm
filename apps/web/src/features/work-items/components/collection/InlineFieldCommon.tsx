import type { ComponentProps, ReactNode } from 'react'

import { InlineEditTrigger } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { MutationState } from '../../model/types'

export type CommonProps = {
  id: string
  label: string
  rowTitle: string
  display: ReactNode
  editable: boolean
  mutation: MutationState
}

export function ReadValue({
  label,
  rowTitle,
  display,
  editable,
  triggerRef,
  onOpen,
}: Pick<CommonProps, 'label' | 'rowTitle' | 'display' | 'editable'> &
  Pick<ComponentProps<typeof InlineEditTrigger>, 'triggerRef' | 'onOpen'>) {
  return (
    <InlineEditTrigger
      label={WORK_ITEMS_COPY.format.fieldLabel(label, rowTitle)}
      display={display}
      editable={editable}
      triggerRef={triggerRef}
      onOpen={onOpen}
    />
  )
}