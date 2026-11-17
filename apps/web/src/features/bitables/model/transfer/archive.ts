import { Unzip, UnzipInflate } from 'fflate'
import { BITABLE_LIMITS as limits } from '../types'
import { assertActive, TransferError } from './types'

function pathName(name: string): string {
  if (!name || name.startsWith('/') || /[\\\u0000-\u001f:]/.test(name) || name.split('/').some(part => part === '..' || part === '.')) throw new TransferError('unsafe_archive')
  return name.normalize('NFC')
}

function crc32(bytes: Uint8Array): number {
  let crc = 0xffffffff
  for (const byte of bytes) {
    crc ^= byte
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0)
  }
  return (crc ^ 0xffffffff) >>> 0
}

// Validate the central directory before any inflate; reject ZIP64/encryption and
// ambiguous duplicate names. Stream in small compressed chunks and cap actual output.
export function readArchive(bytes: Uint8Array, signal?: AbortSignal): Map<string, Uint8Array> {
  if (bytes.length > limits.fileBytes) throw new TransferError('limit')
  if (bytes.length < 22) throw new TransferError('invalid_file')
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  let end = bytes.length - 22
  while (end >= Math.max(0, bytes.length - 65557) && view.getUint32(end, true) !== 0x06054b50) end--
  if (end < 0 || end < bytes.length - 65557 || view.getUint16(end + 4, true) !== 0 || view.getUint16(end + 6, true) !== 0) throw new TransferError('invalid_file')
  const count = view.getUint16(end + 10, true), offset = view.getUint32(end + 16, true)
  if (count > limits.zipEntries) throw new TransferError('limit')
  if (count === 0 || offset >= end || offset + view.getUint32(end + 12, true) !== end || end + 22 + view.getUint16(end + 20, true) !== bytes.length) throw new TransferError('invalid_file')
  const names = new Map<string, number>(), checksums = new Map<string, number>(), folded = new Set<string>()
  let cursor = offset, declared = 0
  for (let i = 0; i < count; i++) {
    assertActive(signal)
    if (cursor + 46 > end || view.getUint32(cursor, true) !== 0x02014b50) throw new TransferError('invalid_file')
    const flags = view.getUint16(cursor + 8, true), method = view.getUint16(cursor + 10, true)
    const size = view.getUint32(cursor + 24, true), nameLength = view.getUint16(cursor + 28, true)
    const next = cursor + 46 + nameLength + view.getUint16(cursor + 30, true) + view.getUint16(cursor + 32, true)
    if ((flags & 1) || ![0, 8].includes(method) || next > end) throw new TransferError('invalid_file')
    let name: string
    try { name = pathName(new TextDecoder('utf-8', { fatal: true }).decode(bytes.subarray(cursor + 46, cursor + 46 + nameLength))) }
    catch (error) { if (error instanceof TransferError) throw error; throw new TransferError('invalid_file') }
    const key = name.toLowerCase()
    if (folded.has(key)) throw new TransferError('unsafe_archive')
    folded.add(key); names.set(name, size); checksums.set(name, view.getUint32(cursor + 16, true)); declared += size
    if (declared > limits.expandedBytes) throw new TransferError('limit')
    cursor = next
  }
  if (cursor !== end) throw new TransferError('invalid_file')
  const files = new Map<string, Uint8Array>(), started = new Set<string>()
  let actual = 0, failure: TransferError | undefined
  const unzip = new Unzip(file => {
    const name = pathName(file.name)
    if (!names.has(name) || started.has(name)) throw new TransferError('unsafe_archive')
    started.add(name)
    const parts: Uint8Array[] = []; let length = 0
    file.ondata = (error, data, final) => {
      if (error) { failure = new TransferError('invalid_file'); file.terminate(); return }
      actual += data.length; length += data.length
      if (actual > limits.expandedBytes || length > (names.get(name) ?? 0)) { failure = new TransferError('limit'); file.terminate(); return }
      parts.push(data)
      if (final) {
        if (length !== names.get(name)) { failure = new TransferError('invalid_file'); return }
        const result = new Uint8Array(length); let position = 0
        for (const part of parts) { result.set(part, position); position += part.length }
        if (crc32(result) !== checksums.get(name)) { failure = new TransferError('invalid_file'); return }
        files.set(name, result)
      }
    }
    file.start()
  })
  unzip.register(UnzipInflate)
  try {
    for (let i = 0; i < bytes.length; i += 256) {
      assertActive(signal)
      unzip.push(bytes.subarray(i, i + 256), i + 256 >= bytes.length)
      if (failure) throw failure
    }
  } catch (error) { if (error instanceof TransferError) throw error; throw new TransferError('invalid_file') }
  if (files.size !== names.size) throw new TransferError('invalid_file')
  return files
}
