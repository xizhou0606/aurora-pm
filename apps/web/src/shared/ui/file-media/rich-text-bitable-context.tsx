import { createContext, useContext, type ReactNode } from 'react'
import type { RichTextBitableReference } from './rich-text-bitable-reference'

export type RichTextBitableIntegration = {
  render: (props: { reference: RichTextBitableReference; readOnly: boolean }) => ReactNode
  pick?: () => Promise<RichTextBitableReference | null>
}

const IntegrationContext = createContext<RichTextBitableIntegration | null>(null)

/** App composition injects the business surface; shared rich text imports no feature. */
export function RichTextBitableProvider({ value, children }: { value: RichTextBitableIntegration; children: ReactNode }) {
  return <IntegrationContext.Provider value={value}>{children}</IntegrationContext.Provider>
}

export function useRichTextBitableIntegration() { return useContext(IntegrationContext) }
