import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/primitives/select'

export function TransferSelect({ label, value, options, disabled, onChange }: {
  label: string; value: string; options: readonly { value: string; label: string }[]; disabled?: boolean; onChange: (value: string) => void
}) {
  return <Select value={value} onValueChange={next => { if (typeof next === 'string') onChange(next) }} disabled={disabled}>
    <SelectTrigger aria-label={label} className="w-full"><SelectValue>{options.find(option => option.value === value)?.label}</SelectValue></SelectTrigger>
    <SelectContent>{options.map(option => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
  </Select>
}
