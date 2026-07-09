import { writable } from 'svelte/store'
import { live } from '../live/ws'

export interface SystemStats {
  cpu_percent: number
  cpu_temp_c: number
  mem_percent: number
  mem_total: number
  mem_used: number
  disk_percent: number
  disk_total: number
  disk_used: number
}

export const systemStats = writable<SystemStats | null>(null)

/** Begin receiving system stats over the WebSocket. Returns an unsubscribe fn. */
export function subscribeSystem(): () => void {
  return live.subscribe('system', (d) => systemStats.set(d as SystemStats))
}
