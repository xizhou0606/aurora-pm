import { HttpAuthorityBridge } from './authority-http.js'
import { CollaborationRuntime } from './server.js'

function required(name: string): string {
  const value = process.env[name]
  if (!value) throw new Error(`${name} is required`)
  return value
}

function integer(name: string, fallback: number): number {
  const source = process.env[name]
  if (!source) return fallback
  const value = Number(source)
  if (!Number.isSafeInteger(value) || value <= 0) throw new Error(`${name} must be a positive integer`)
  return value
}

const authority = new HttpAuthorityBridge({
  baseUrl: required('PM_COLLAB_AUTHORITY_URL'),
  serviceToken: required('PM_COLLAB_SERVICE_TOKEN'),
  timeoutMs: integer('PM_COLLAB_AUTHORITY_TIMEOUT_MS', 2_000),
})
const runtime = new CollaborationRuntime({
  authority,
  serviceToken: required('PM_COLLAB_SERVICE_TOKEN'),
  address: process.env.PM_COLLAB_ADDRESS ?? '127.0.0.1',
  port: integer('PM_COLLAB_PORT', 1234),
  maxConnections: integer('PM_COLLAB_MAX_CONNECTIONS', 200),
  maxDocuments: integer('PM_COLLAB_MAX_DOCUMENTS', 100),
  maxPayloadBytes: integer('PM_COLLAB_MAX_PAYLOAD_BYTES', 256 * 1024),
  revalidateCeilingMs: integer('PM_COLLAB_REVALIDATE_CEILING_MS', 5_000),
})

await runtime.listen()
console.log(`PM collaboration spike listening on ${runtime.httpUrl}`)

let stopping = false
const stop = async () => {
  if (stopping) return
  stopping = true
  await runtime.destroy()
}
process.once('SIGINT', stop)
process.once('SIGTERM', stop)
