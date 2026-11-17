import { copy } from '@/shared/copy'
import { FormField } from '@/shared/ui'

export function WriteFormField({
  id,
  label,
  required = false,
  error,
  children,
}: {
  id: string
  label: string
  required?: boolean
  error?: string
  children: React.ReactNode
}) {
  return (
    <FormField
      id={id}
      label={<>{label}{required ? <span className="ml-1 text-xs text-destructive">{copy.projectManagement.write.actions.required}</span> : null}</>}
      error={error}
    >
      {children}
    </FormField>
  )
}
