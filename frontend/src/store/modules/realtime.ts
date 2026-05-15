import { defineStore } from 'pinia'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'

type RealtimeState = 'idle' | 'connected' | 'degraded'

const Realtime = defineStore('Realtime', {
  state: () => ({
    state: <RealtimeState>'idle',
    ws: <WebSocket | null>null,
    fallbackTimer: <ReturnType<typeof setInterval> | null>null,
  }),
  actions: {
    async connect() {
      if (this.ws || this.state === 'connected') return
      const tokenResponse = await HttpUtils.get('api/realtime/ws-token')
      const token = tokenResponse.obj?.token
      if (!tokenResponse.success || typeof token !== 'string') {
        this.startFallback()
        return
      }
      const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
      const base = (window as any).BASE_URL ?? '/'
      const url = `${scheme}://${window.location.host}${base}api/realtime/ws`
      try {
        const ws = new WebSocket(url, ['sui.realtime', token])
        this.ws = ws
        ws.onopen = () => {
          this.state = 'connected'
          this.stopFallback()
        }
        ws.onmessage = (event) => {
          try {
            const payload = JSON.parse(event.data)
            if (payload.type === 'reload') Data().loadData()
          } catch {
            // Ignore malformed realtime messages and keep the connection open.
          }
        }
        ws.onclose = () => {
          this.ws = null
          this.startFallback()
        }
        ws.onerror = () => {
          ws.close()
        }
      } catch {
        this.startFallback()
      }
    },
    disconnect() {
      if (this.ws) {
        this.ws.close()
        this.ws = null
      }
      this.stopFallback()
      this.state = 'idle'
    },
    startFallback() {
      this.state = 'degraded'
      if (this.fallbackTimer) return
      this.fallbackTimer = setInterval(() => {
        Data().loadData()
      }, 10000)
    },
    stopFallback() {
      if (this.fallbackTimer) {
        clearInterval(this.fallbackTimer)
        this.fallbackTimer = null
      }
    },
  },
})

export default Realtime
