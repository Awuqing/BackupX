package service

import (
	"context"
	"sync"
	"time"
)

// EventBroadcaster 企业级事件总线的实时订阅中心。
// 不替代 Notification（持久化订阅、多渠道）；作为"前端实时 UI 推送"的低延迟通道。
//
// 架构：
//   - Notification 总线：持久化/多渠道（邮件/webhook/telegram）/审计
//   - EventBroadcaster：内存 pub-sub，给浏览器 SSE 推送（Dashboard 自刷新、桌面 Toast）
//
// 设计决策：
//   - 非阻塞发布：订阅者 channel 满则丢弃该条，不阻塞生产者
//   - 无持久化：订阅者掉线后重连不回放（业务不需要，事件重要性由 Notification 保证）
//   - 轻量：sync.Map + 缓冲 channel
type EventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[int]chan EventEnvelope
	nextID      int
}

// EventEnvelope 推送给订阅者的事件包。
// 复用 Notification 事件类型常量（model.NotificationEvent*）。
type EventEnvelope struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Fields    map[string]any `json:"fields,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{subscribers: map[int]chan EventEnvelope{}}
}

// Subscribe 订阅事件流。buffer 建议 32，避免慢消费者阻塞。
// 返回 channel 和 cancel 函数，调用方需在退出时 cancel。
func (b *EventBroadcaster) Subscribe(buffer int) (<-chan EventEnvelope, func()) {
	if buffer <= 0 {
		buffer = 32
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	ch := make(chan EventEnvelope, buffer)
	b.subscribers[id] = ch
	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if sub, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(sub)
		}
	}
	return ch, cancel
}

// Publish 非阻塞发布：订阅者 channel 满时丢弃，不影响其他订阅者。
// 实现 EventDispatcher 接口，可直接接入 NotificationService 的分发链。
func (b *EventBroadcaster) Publish(ctx context.Context, eventType, title, body string, fields map[string]any) error {
	envelope := EventEnvelope{
		Type:      eventType,
		Title:     title,
		Body:      body,
		Fields:    fields,
		Timestamp: time.Now().UTC(),
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subscribers {
		select {
		case sub <- envelope:
		default:
			// 订阅者慢消费 → 丢弃本条，不阻塞其他订阅者
		}
	}
	return nil
}

// DispatchEvent 实现 EventDispatcher 接口（与 NotificationService 相同）。
// 让 broadcaster 可以无侵入地接入现有事件派发链。
func (b *EventBroadcaster) DispatchEvent(ctx context.Context, eventType, title, body string, fields map[string]any) error {
	return b.Publish(ctx, eventType, title, body, fields)
}

// SubscriberCount 当前活跃订阅者数，供 metrics / 健康检查使用。
func (b *EventBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
