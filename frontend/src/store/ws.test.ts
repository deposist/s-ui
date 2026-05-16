import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest'

vi.mock('@/plugins/httputil', () => ({
  default: { get: vi.fn() },
}))

vi.mock('@/store/modules/data', () => ({
  default: () => ({ loadData: vi.fn(), onlines: {} }),
}))

import { reconnectDelayForRetry, WsLike, WsRuntime } from './ws'

class FakeSocket implements WsLike {
  onopen: ((event?: any) => void) | null = null
  onmessage: ((event: any) => void) | null = null
  onclose: ((event?: any) => void) | null = null
  onerror: ((event?: any) => void) | null = null
  close = vi.fn(() => {
    this.onclose?.()
  })
}

describe('WsRuntime fallback', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('enters degraded polling when websocket does not open within 5s', async () => {
    const loadData = vi.fn()
    const states: string[] = []
    const socket = new FakeSocket()
    const runtime = new WsRuntime({
      getToken: async () => 'token',
      createSocket: () => socket,
      loadData,
      onState: (state) => states.push(state),
      location: { protocol: 'http:', host: 'panel.test' },
      baseUrl: '/',
    })

    await runtime.connect()
    expect(runtime.state).toBe('reconnecting')

    vi.advanceTimersByTime(5000)
    expect(runtime.state).toBe('degraded')
    expect(states).toContain('degraded')

    vi.advanceTimersByTime(10000)
    expect(loadData).toHaveBeenCalledTimes(1)
  })

  it('uses capped exponential reconnect backoff with jitter', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0)
    expect([0, 1, 2, 3, 4, 5].map(reconnectDelayForRetry)).toEqual([
      250,
      500,
      1000,
      2000,
      4000,
      5000,
    ])

    vi.mocked(Math.random).mockReturnValue(0.5)
    expect(reconnectDelayForRetry(0)).toBe(375)
    expect(reconnectDelayForRetry(1)).toBe(625)
  })
})
