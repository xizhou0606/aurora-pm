export const WORK_ITEM_FORMAT_COPY = Object.freeze({
  separator: ' · ',
  meta: (...parts: string[]) => parts.filter(Boolean).join(' · '),
  dateRange: (start: string, end: string) => `${start} – ${end}`,
  fieldLabel: (label: string, value: string) => `${label}：${value}`,
})
