import { defineStore } from 'pinia'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'

export type WsConnectionState = 'connected' | 'reconnecting' | 'degraded'

export interface WsLike {
  onopen: ((event?: any) => void) | null
  onmessage: ((event: any) => void) | null
  onclose: ((event?: any) => void) | null
  onerror: ((event?: any) => void) | null
  close: () => void
}

export interface WsRuntimeDeps {
  getToken: () => Promise<string | null>
  createSocket: (url: string, token: string) => WsLike
  loadData: () => void | Promise<void>
  onState?: (state: WsConnectionState) => void
  onEvent?: (event: any) => void
  setTimeout?: typeof setTimeout
  clearTimeout?: typeof clearTimeout
  setInterval?: typeof setInterval
  clearInterval?: typeof clearInterval
  location?: Pick<Location, 'protocol' | 'host'>
  baseUrl?: string
}

const noOpenFallbackMs = 5000
const reconnectBaseDelayMs = 250
const reconnectJitterMs = 250
const reconnectMaxDelayMs = 5000
const fallbackPollMs = 10000
const closeFallbackThreshold = 3

export const reconnectDelayForRetry = (retry: number) => {
  const safeRetry = Math.max(0, retry)
  const exponentialDelay = Math.pow(2, safeRetry) * reconnectBaseDelayMs
  const jitter = Math.random() * reconnectJitterMs
  return Math.min(exponentialDelay + jitter, reconnectMaxDelayMs)
}

export class WsRuntime {
  state: WsConnectionState = 'degraded'
  private ws: WsLike | null = null
  private noOpenTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private fallbackTimer: ReturnType<typeof setInterval> | null = null
  private closeCount = 0

  constructor(private deps: WsRuntimeDeps) {}

  async connect() {
    if (this.ws || this.state === 'connected') return
    this.setState('reconnecting')
    this.stopFallback()
    const token = await this.deps.getToken()
    if (!token) {
      this.startFallback()
      return
    }
    try {
      const ws = this.deps.createSocket(this.wsURL(), token)
      this.ws = ws
      this.noOpenTimer = this.timer().setTimeout(() => {
        this.ws = null
        ws.onclose = null
        ws.close()
        this.startFallback()
      }, noOpenFallbackMs)
      ws.onopen = () => {
        this.closeCount = 0
        this.clearNoOpenTimer()
        this.setState('connected')
        this.stopFallback()
      }
      ws.onmessage = (event) => {
        try {
          this.deps.onEvent?.(JSON.parse(event.data))
        } catch {
          // Keep realtime open when a single event is malformed.
        }
      }
      ws.onclose = () => {
        this.clearNoOpenTimer()
        this.ws = null
        this.closeCount++
        if (this.closeCount >= closeFallbackThreshold) {
          this.startFallback()
          return
        }
        this.setState('reconnecting')
        const retry = this.closeCount - 1
        this.reconnectTimer = this.timer().setTimeout(() => {
          void this.connect()
        }, reconnectDelayForRetry(retry))
      }
      ws.onerror = () => {
        ws.close()
      }
    } catch {
      this.startFallback()
    }
  }

  disconnect() {
    this.clearNoOpenTimer()
    if (this.reconnectTimer) {
      this.timer().clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      const ws = this.ws
      this.ws = null
      ws.onclose = null
      ws.close()
    }
    this.stopFallback()
    this.setState('degraded')
  }

  private startFallback() {
    this.clearNoOpenTimer()
    this.setState('degraded')
    if (this.fallbackTimer) return
    this.fallbackTimer = this.timer().setInterval(() => {
      void this.deps.loadData()
    }, fallbackPollMs)
  }

  private stopFallback() {
    if (!this.fallbackTimer) return
    this.timer().clearInterval(this.fallbackTimer)
    this.fallbackTimer = null
  }

  private clearNoOpenTimer() {
    if (!this.noOpenTimer) return
    this.timer().clearTimeout(this.noOpenTimer)
    this.noOpenTimer = null
  }

  private setState(state: WsConnectionState) {
    this.state = state
    this.deps.onState?.(state)
  }

  private wsURL() {
    const loc = this.deps.location ?? window.location
    const scheme = loc.protocol === 'https:' ? 'wss' : 'ws'
    const base = this.deps.baseUrl ?? ((window as any).BASE_URL ?? '/')
    return `${scheme}://${loc.host}${base}api/realtime/ws`
  }

  private timer() {
    return {
      setTimeout: this.deps.setTimeout ?? setTimeout,
      clearTimeout: this.deps.clearTimeout ?? clearTimeout,
      setInterval: this.deps.setInterval ?? setInterval,
      clearInterval: this.deps.clearInterval ?? clearInterval,
    }
  }
}

const applyRealtimeEvent = (event: any) => {
  const data = Data()
  switch (event?.type) {
    case 'onlines':
      if (event.payload) data.onlines = event.payload
      break
    case 'config_invalidated':
    case 'reload':
      void data.loadData()
      break
  }
}

const Ws = defineStore('Ws', {
  state: () => ({
    state: <WsConnectionState>'degraded',
    runtime: <WsRuntime | null>null,
  }),
  actions: {
    ensureRuntime() {
      if (!this.runtime) {
        this.runtime = new WsRuntime({
          getToken: async () => {
            const tokenResponse = await HttpUtils.get('api/realtime/ws-token')
            const token = tokenResponse.obj?.token
            return tokenResponse.success && typeof token === 'string' ? token : null
          },
          createSocket: (url, token) => new WebSocket(url, ['sui.realtime', token]),
          loadData: () => Data().loadData(),
          onState: (state) => {
            this.state = state
          },
          onEvent: applyRealtimeEvent,
        })
      }
      return this.runtime
    },
    connect() {
      return this.ensureRuntime().connect()
    },
    disconnect() {
      this.runtime?.disconnect()
      this.runtime = null
      this.state = 'degraded'
    },
  },
})

export default Ws
