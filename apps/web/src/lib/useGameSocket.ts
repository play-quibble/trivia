'use client'

import { useEffect, useRef, useState, useCallback } from 'react'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/**
 * Connection lifecycle states exposed to callers.
 *
 * - connecting   : initial connection attempt in progress
 * - open         : connected and ready
 * - reconnecting : lost connection; retrying with backoff
 * - failed       : exhausted retries or unrecoverable error
 * - closed       : intentionally closed (component unmounted)
 */
export type SocketStatus = 'connecting' | 'open' | 'reconnecting' | 'failed' | 'closed'

export interface UseGameSocketOptions {
  /**
   * Called for every inbound message. `type` is the message's `type` field;
   * `payload` is whatever came in the `payload` field (defaults to `{}`).
   */
  onMessage: (type: string, payload: unknown) => void

  /**
   * Called once the socket successfully opens. Use this to set initial
   * game phase (e.g. move from 'connecting' to 'lobby').
   */
  onOpen?: () => void

  /**
   * Maximum number of reconnection attempts after an unclean drop.
   * Initial connection failures do NOT trigger retries — a bad URL or
   * auth token means there's nothing to reconnect to.
   * Defaults to 5.
   */
  maxRetries?: number
}

export interface UseGameSocketResult {
  /** Current connection state. Drive your loading/error UI from this. */
  status: SocketStatus

  /**
   * Send a message to the server. No-ops silently if the socket is not open,
   * so callers don't need to guard against race conditions.
   */
  send: (type: string, payload?: unknown) => void
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * Core WebSocket hook for Quibble game rooms.
 *
 * Manages connection lifecycle, exponential-backoff reconnection, and message
 * dispatch. Callers (useHostSocket / usePlayerSocket) build typed APIs on top.
 *
 * Reconnection strategy:
 * - Only retries after a *mid-session* drop (socket was previously open).
 * - Does NOT retry if the initial connection fails — that indicates a bad URL
 *   or auth token, not a transient network problem.
 * - Does NOT retry on clean closes (1000 / 1001) or app-level rejections
 *   (close codes 4000+, e.g. auth failure).
 * - Backoff: min(1000 * 2^attempt, 16 000) ms, up to `maxRetries` attempts.
 * - `onerror` is intentionally a no-op; all retry logic lives in `onclose`
 *   because browsers always fire `onerror` immediately before `onclose`.
 */
export function useGameSocket(
  url: string,
  options: UseGameSocketOptions,
): UseGameSocketResult {
  const [status, setStatus] = useState<SocketStatus>('connecting')

  // Keep options in a ref so callbacks inside the effect always see the latest
  // values without the effect needing to re-run (and tear down the socket)
  // whenever an inline function prop is recreated by the parent.
  const optionsRef = useRef(options)
  useEffect(() => { optionsRef.current = options })

  // Stable ref to the live WebSocket so `send` never captures a stale handle.
  const wsRef = useRef<WebSocket | null>(null)

  const send = useCallback((type: string, payload?: unknown) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ type, payload: payload ?? {} }))
  }, [])

  useEffect(() => {
    // Capture maxRetries at mount time — it shouldn't change mid-game and we
    // don't want the effect re-running just because a parent passed a new literal.
    const maxRetries = optionsRef.current.maxRetries ?? 5

    let retries = 0
    let everConnected = false
    let retryTimeout: ReturnType<typeof setTimeout> | null = null
    let unmounted = false

    function backoffMs(attempt: number) {
      return Math.min(1000 * Math.pow(2, attempt), 16_000)
    }

    function connect() {
      if (unmounted) return

      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        if (unmounted) return
        retries = 0
        everConnected = true
        setStatus('open')
        optionsRef.current.onOpen?.()
      }

      ws.onmessage = (e: MessageEvent) => {
        let msg: { type: string; payload?: unknown }
        try {
          msg = JSON.parse(e.data as string)
        } catch {
          return
        }
        optionsRef.current.onMessage(msg.type, msg.payload ?? {})
      }

      // onerror always fires immediately before onclose — handle everything there.
      ws.onerror = () => {}

      ws.onclose = (e: CloseEvent) => {
        if (unmounted) return
        wsRef.current = null

        // Only retry unclean mid-session drops.
        // - Require everConnected: initial failures (wrong code/token) should
        //   surface immediately rather than silently retry.
        // - Skip 1000/1001: server closed the connection intentionally.
        // - Skip 4000+: app-level rejection (auth failure, game not found, etc.)
        const retryable =
          everConnected &&
          e.code !== 1000 &&
          e.code !== 1001 &&
          e.code < 4000

        if (retryable && retries < maxRetries) {
          setStatus('reconnecting')
          retryTimeout = setTimeout(connect, backoffMs(retries))
          retries++
        } else {
          setStatus('failed')
        }
      }
    }

    setStatus('connecting')
    connect()

    return () => {
      unmounted = true
      if (retryTimeout) clearTimeout(retryTimeout)
      wsRef.current?.close()
      setStatus('closed')
    }
  // `url` is the only value that should re-create the socket.
  // `options` is kept current via `optionsRef` without re-running the effect.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url])

  return { status, send }
}
