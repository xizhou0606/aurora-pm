import { PhCalendar, PhCheck, PhClock, PhList, PhListNumbers, PhPaperclip, PhTextBolder, PhUser, type IconComponent } from '@/shared/ui/icons'
import type { FieldType } from './types'
export const fieldIcons: Record<FieldType, IconComponent> = {
  text: PhTextBolder, long_text: PhTextBolder, number: PhListNumbers, checkbox: PhCheck,
  single_select: PhList, multiple_select: PhList, date: PhCalendar, user: PhUser,
  attachment: PhPaperclip, created_by: PhUser, last_modified_by: PhUser,
  created_time: PhClock, last_modified_time: PhClock,
}
