import type { StatusPillTone } from '@/shared/ui'
import type { Choice } from './types'
const palette: readonly StatusPillTone[] = ['info', 'accent', 'warning', 'success', 'attention', 'highlight', 'danger']
const mapped = { neutral: 'neutral', blue: 'info', green: 'success', yellow: 'warning', orange: 'attention', red: 'danger', purple: 'accent', pink: 'accent' } as const
export function choiceTone(choice: Pick<Choice, 'id' | 'color'>): StatusPillTone {
  if (choice.color) return mapped[choice.color]
  let hash = 0
  for (const character of choice.id) hash = (Math.imul(hash, 31) + character.charCodeAt(0)) >>> 0
  return palette[hash % palette.length]
}
export const pinkChoiceClass = 'border-pink-200 bg-pink-50 text-pink-700 dark:border-pink-400/35 dark:bg-pink-400/14 dark:text-pink-200'
