/** Open a dedicated WebSocket stream (logs/stats) and forward messages.
 *  Returns a close function. */
export function openStream(
  path: string,
  onMessage: (data: string) => void,
  onClose?: () => void,
): () => void {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}${path}`)
  ws.onmessage = (e) => onMessage(String(e.data))
  ws.onclose = () => onClose?.()
  return () => {
    try {
      ws.close()
    } catch {
      /* already closed */
    }
  }
}
