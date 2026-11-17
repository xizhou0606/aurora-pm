import crossws from 'crossws/adapters/bun'
import type { Hocuspocus } from '@hocuspocus/server'
import type { ConnectionContext } from './contract.js'

/**
 * Bun transport for the collaboration runtime.
 *
 * `@hocuspocus/server`'s `Server` wrapper hard-imports crossws' Node adapter,
 * which refuses to run under Bun (ueberdosis/hocuspocus#1102). The official
 * Bun integration (hocuspocus playground `backend/src/bun.ts`) is to use the
 * `Hocuspocus` class directly and wire crossws' Bun adapter into `Bun.serve`.
 * This file is that wiring, kept out of the runtime's business logic.
 */
export interface BunTransportOptions {
  hocuspocus: Hocuspocus<ConnectionContext>
  hostname: string
  port: number
  maxPayloadSize: number
  handleHttpRequest: (request: Request) => Promise<Response>
}

interface HocuspocusPeer {
  _hocuspocus?: ReturnType<Hocuspocus<ConnectionContext>['handleConnection']>
}

type CrosswsBunAdapter = ReturnType<typeof crossws>
// The Bun server type crossws' Bun adapter upgrades against (Bun's own
// `Server<ContextData>`; ContextData is crossws-internal, so extract it).
type CrosswsBunServer = Parameters<CrosswsBunAdapter['handleUpgrade']>[1]

export class BunTransport {
  private readonly hocuspocus: Hocuspocus<ConnectionContext>
  private readonly handleHttpRequest: (request: Request) => Promise<Response>
  private readonly hostname: string
  private readonly requestedPort: number
  private readonly maxPayloadSize: number
  private readonly ws: CrosswsBunAdapter
  private server: CrosswsBunServer | undefined

  constructor(options: BunTransportOptions) {
    this.hocuspocus = options.hocuspocus
    this.handleHttpRequest = options.handleHttpRequest
    this.hostname = options.hostname
    this.requestedPort = options.port
    this.maxPayloadSize = options.maxPayloadSize
    this.ws = crossws({
      hooks: {
        open: (peer) => {
          // Use peer methods instead of peer.websocket to avoid Bun's Proxy
          // `this` binding issue with ServerWebSocket (official guidance).
          const wsLike = {
            get readyState(): number {
              return peer.websocket.readyState ?? 3
            },
            send: (data: string | ArrayBufferLike | ArrayBufferView | Blob): void => {
              peer.send(data)
            },
            close: (code?: number, reason?: string): void => {
              peer.close(code, reason)
            },
          }
          ;(peer as HocuspocusPeer)._hocuspocus = this.hocuspocus.handleConnection(wsLike, peer.request)
        },
        message: (peer, message) => {
          ;(peer as HocuspocusPeer)._hocuspocus?.handleMessage(message.uint8Array())
        },
        close: (peer, event) => {
          // CloseEvent requires both fields; Bun always provides them.
          ;(peer as HocuspocusPeer)._hocuspocus?.handleClose({
            code: event.code ?? 1005, // 1005 = No Status Received (RFC 6455)
            reason: event.reason ?? '',
          })
        },
        error: (peer, error) => {
          console.error('WebSocket error for peer:', peer.id)
          console.error(error)
        },
      },
    })
  }

  serve(): void {
    // maxPayloadSize is a real Bun websocket option but is missing from
    // bun-types 1.4.2's WebSocketHandler; widen the crossws handler type with it.
    const websocket: CrosswsBunAdapter['websocket'] & { maxPayloadSize: number } = {
      ...this.ws.websocket,
      maxPayloadSize: this.maxPayloadSize,
    }
    this.server = Bun.serve({
      hostname: this.hostname,
      port: this.requestedPort,
      fetch: async (request, server) => {
        if (request.headers.get('upgrade') === 'websocket') {
          const response = await this.ws.handleUpgrade(request, server)
          return response ?? new Response(null, { status: 101 })
        }
        return this.handleHttpRequest(request)
      },
      websocket,
    })
  }

  stop(): void {
    this.server?.stop(false)
    this.server = undefined
  }

  get port(): number {
    if (!this.server) throw new Error('collaboration transport is not listening')
    // A listening server always reports its bound port.
    return this.server.port ?? 0
  }

  get httpUrl(): string {
    return `http://${this.hostname}:${this.port}`
  }

  get webSocketUrl(): string {
    return `ws://${this.hostname}:${this.port}`
  }
}
