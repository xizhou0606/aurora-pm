import type { RequirementDocument } from '../api/requirement-documents'

export type RequirementDocumentChoice =
  | { type: 'feishu'; url: string }
  | { type: 'internal'; document: RequirementDocument | null }
