import { create } from 'zustand'
import type { SystemEvent } from '../hooks/useEventStream'

// 最多保留最近 50 条事件，防止内存增长
const MAX_EVENTS = 50

interface StoredEvent extends SystemEvent {
  id: string
  read: boolean
}

interface EventState {
  events: StoredEvent[]
  unreadCount: number
  addEvent: (event: SystemEvent) => void
  markAllRead: () => void
  clear: () => void
}

/**
 * useEventStore 全局事件历史存储。
 * 设计为非持久化（session 内存）：
 * - 事件重要性由后端 Notification 持久化保证
 * - 前端 store 只负责当前会话的未读提示与历史查看
 * - 浏览器刷新即清空，避免 localStorage 膨胀
 */
export const useEventStore = create<EventState>((set) => ({
  events: [],
  unreadCount: 0,
  addEvent: (event) =>
    set((state) => {
      const stored: StoredEvent = {
        ...event,
        id: `${event.timestamp}-${event.type}-${Math.random().toString(36).slice(2, 8)}`,
        read: false,
      }
      const events = [stored, ...state.events].slice(0, MAX_EVENTS)
      return { events, unreadCount: state.unreadCount + 1 }
    }),
  markAllRead: () =>
    set((state) => ({
      events: state.events.map((e) => ({ ...e, read: true })),
      unreadCount: 0,
    })),
  clear: () => set({ events: [], unreadCount: 0 }),
}))
