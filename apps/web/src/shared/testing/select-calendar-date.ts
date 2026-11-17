import { fireEvent, screen, within } from '@testing-library/react'
import { copy } from '@/shared/copy'

/** Drive the real calendar, keeping dates deterministic without faking input events. */
export async function selectCalendarDate(trigger: HTMLElement, iso: string, dateTime = false) {
  fireEvent.click(trigger)
  const dialog = await screen.findByRole('dialog', { name: dateTime ? copy.ui.datePicker.dateTimeTitle : copy.ui.datePicker.dateTitle })
  const [year, month, day] = iso.slice(0, 10).split('-').map(Number)
  const target = year * 12 + month
  for (let attempt = 0; attempt < 120; attempt++) {
    const heading = dialog.querySelector('strong')?.textContent?.match(/(\d+)年(\d+)月/)
    if (!heading) throw new Error('Calendar month heading missing')
    const current = Number(heading[1]) * 12 + Number(heading[2])
    if (current === target) break
    fireEvent.click(within(dialog).getByRole('button', { name: current > target ? copy.ui.datePicker.previousMonth : copy.ui.datePicker.nextMonth }))
  }
  fireEvent.click(within(dialog).getByRole('button', { name: copy.ui.datePicker.selectDate(`${year}年${month}月${day}日`) }))
  if (dateTime) fireEvent.click(within(dialog).getByRole('button', { name: copy.ui.datePicker.confirm }))
}
