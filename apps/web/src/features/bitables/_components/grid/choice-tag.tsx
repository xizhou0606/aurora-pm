import { StatusPill } from '@/shared/ui'
import { cn } from '@/shared/lib/utils'
import { choiceTone } from '../../model/choice-tone'
import { displayCellValue } from '../../model/fields'
import type { Field } from '../../model/types'

/** Presentation only: IDs retain their color when labels or option ordering change. */
export function ChoiceTag({ field, id }: { field: Field; id: string }) {
  const choice = field.options?.choices?.find(item => item.id === id) ?? { id, color: undefined }
  const label = displayCellValue(field, [id])
  return <StatusPill tone={choiceTone(choice)} title={label} className={cn('min-w-0 shrink rounded-md border-transparent px-1.5 py-0 font-normal hover:ring-0', choice.color === 'pink' && 'border-pink-200 bg-pink-50 text-pink-700 dark:border-pink-400/35 dark:bg-pink-400/14 dark:text-pink-200')}><span className="truncate">{label}</span></StatusPill>
}
