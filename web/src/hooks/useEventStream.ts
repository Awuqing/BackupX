import { useEffect, useRef } from 'react'
import { getAccessToken } from '../services/http'

export interface SystemEvent {
  type: string
  title: string
  body: string
  fields?: Record<string, unknown>
  timestamp: string
}

export type EventHandler = (event: SystemEvent) => void

/**
 * useEventStream 订阅后端 SSE 实时事件流。
 *
 * 因 EventSource 原生不支持自定义 header，这里使用 fetch + ReadableStream 解析 SSE 帧。
 * 优势：带 Authorization Bearer Token，无需把 token 放在 URL 里被日志记录。
 *
 * 连接中断时自动指数退避重连（1s / 2s / 4s / ... / 最大 30s）。
 */
export function useEventStream(handler: EventHandler, types?: string[]) {
  const handlerRef = useRef(handler)
  handlerRef.current = handler

  const typesKey = types ? types.sort().join(',') : ''

  useEffect(() => {
    let active = true
    let controller: AbortController | null = null
    let reconnectTimer: number | null = null
    let backoff = 1000

    async function connect() {
      if (!active) return
      controller = new AbortController()
      const token = getAccessToken()
      try {
        const response = await fetch('/api/events/stream', {
          method: 'GET',
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
          signal: controller.signal,
        })
        if (!response.ok || !response.body) {
          throw new Error(`SSE 连接失败（HTTP ${response.status}）`)
        }
        backoff = 1000 // 连上后重置退避
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        while (active) {
          const { value, done } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          // 按 "\n\n" 分帧
          while (buffer.includes('\n\n')) {
            const boundary = buffer.indexOf('\n\n')
            const frame = buffer.slice(0, boundary)
            buffer = buffer.slice(boundary + 2)
            const event = parseFrame(frame)
            if (!event) continue
            if (types && !types.includes(event.type)) continue
            handlerRef.current(event)
          }
        }
      } catch (e) {
        if (!active) return
        // 连接断开，指数退避后重连
        if (e instanceof DOMException && e.name === 'AbortError') return
        reconnectTimer = window.setTimeout(connect, backoff)
        backoff = Math.min(backoff * 2, 30000)
      }
    }

    void connect()

    return () => {
      active = false
      if (controller) controller.abort()
      if (reconnectTimer) window.clearTimeout(reconnectTimer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [typesKey])
}

// parseFrame 解析 SSE 帧（event: type\ndata: json）。
function parseFrame(frame: string): SystemEvent | null {
  const lines = frame.split('\n')
  let eventType = ''
  let dataLine = ''
  for (const line of lines) {
    if (line.startsWith(':')) continue // comment（心跳）
    if (line.startsWith('event:')) {
      eventType = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      dataLine = line.slice(5).trim()
    }
  }
  if (!dataLine) return null
  try {
    const parsed = JSON.parse(dataLine) as SystemEvent
    if (eventType && !parsed.type) parsed.type = eventType
    return parsed
  } catch {
    return null
  }
}
