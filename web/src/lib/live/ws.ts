// Single WebSocket connection to /ws, multiplexed by channel. Channels are
// subscribed lazily (only while a component wants them) and re-subscribed after
// reconnects, matching the backend's subscribe-gated sampling.

type Handler = (data: unknown) => void

class LiveClient {
  private ws: WebSocket | null = null
  private handlers = new Map<string, Set<Handler>>()
  private subscribed = new Set<string>()
  private reconnectTimer: number | null = null
  private started = false

  connect() {
    if (this.started && this.ws) return
    this.started = true
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/ws`)
    this.ws = ws
    ws.onopen = () => {
      for (const ch of this.subscribed) this.raw('subscribe', ch)
    }
    ws.onmessage = (ev) => {
      try {
        const env = JSON.parse(ev.data)
        const set = this.handlers.get(env.channel ?? env.type)
        if (set) for (const h of set) h(env.data)
      } catch {
        /* ignore malformed frames */
      }
    }
    ws.onclose = () => {
      this.ws = null
      this.scheduleReconnect()
    }
    ws.onerror = () => ws.close()
  }

  private scheduleReconnect() {
    if (this.reconnectTimer != null) return
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, 2000)
  }

  private raw(type: string, channel: string, id?: string) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, channel, id }))
    }
  }

  /** Subscribe to a channel; returns an unsubscribe function. */
  subscribe(channel: string, handler: Handler): () => void {
    let set = this.handlers.get(channel)
    if (!set) {
      set = new Set()
      this.handlers.set(channel, set)
    }
    set.add(handler)
    if (!this.subscribed.has(channel)) {
      this.subscribed.add(channel)
      this.raw('subscribe', channel)
    }
    return () => {
      set!.delete(handler)
      if (set!.size === 0) {
        this.handlers.delete(channel)
        this.subscribed.delete(channel)
        this.raw('unsubscribe', channel)
      }
    }
  }
}

export const live = new LiveClient()
