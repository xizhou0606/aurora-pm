import { TransferError } from './types'

export function parseXml(bytes: Uint8Array | undefined): Document {
  if (!bytes) throw new TransferError('invalid_file')
  let source: string
  try { source = new TextDecoder('utf-8', { fatal: true }).decode(bytes) }
  catch { throw new TransferError('invalid_encoding') }
  if (/<!DOCTYPE|<!ENTITY/i.test(source)) throw new TransferError('invalid_xml')
  const doc = new DOMParser().parseFromString(source, 'application/xml')
  if (doc.getElementsByTagName('parsererror').length) throw new TransferError('invalid_xml')
  return doc
}
export function elements(root: Document | Element, name: string): Element[] {
  return Array.from(root.getElementsByTagNameNS('*', name))
}
export function firstText(root: Element, name: string): string | null { return elements(root, name)[0]?.textContent ?? null }
export function escapeXml(value: string): string {
  if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value)) throw new TransferError('invalid_cell')
  return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&apos;')
}
