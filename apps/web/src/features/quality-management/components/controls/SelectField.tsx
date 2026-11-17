import { FormField, OptionMultiSelect, OptionSelect, type OptionSelectOption, type OptionSelectProps, type OptionMultiSelectProps } from '@/shared/ui'
import { QUALITY_COPY } from '@/shared/copy'

export type QualitySelectOption = OptionSelectOption

type QualityFieldProps = {
  showLabel?: boolean
  className?: string
  clearable?: boolean
}

type QualitySelectFieldProps = Omit<OptionSelectProps, 'value' | 'onValueChange' | 'clearLabel' | 'emptyOption'> & QualityFieldProps & {
  value: string
  emptyLabel: string
  error?: string
  onValueChange: (value: string) => void
}

// Quality's empty string means either the visible empty option or a placeholder.
// Keep that domain conversion here; no option string is reserved by shared UI.
export function QualitySelectField({ value, emptyLabel, clearable, showLabel, className, error, onValueChange, ...props }: QualitySelectFieldProps) {
  return (
    <FormField id={props.id} label={props.label} showLabel={showLabel} className={className} error={error} describedBy={props.describedBy}>
      {(control) => (
        <OptionSelect {...props} value={value || null} emptyOption={{ label: emptyLabel }} clearLabel={clearable ? emptyLabel : undefined} describedBy={control['aria-describedby']} invalid={control['aria-invalid'] || props.invalid} onValueChange={(next) => onValueChange(next ?? '')} />
      )}
    </FormField>
  )
}

export function QualityMultiSelectField({ clearable, showLabel, className, ...props }: Omit<OptionMultiSelectProps, 'clearLabel'> & QualityFieldProps) {
  return (
    <FormField id={props.id} label={props.label} showLabel={showLabel} className={className}>
      <OptionMultiSelect {...props} clearLabel={clearable ? QUALITY_COPY.filters.clearSelection : undefined} />
    </FormField>
  )
}
