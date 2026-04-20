package http

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// EventsHandler 实时事件推送（SSE）。
// 前端通过 EventSource 订阅 /api/events/stream，实时接收系统事件，
// 用于 Dashboard 免刷新更新 / 桌面 Toast / 实时告警。
type EventsHandler struct {
	broadcaster *service.EventBroadcaster
}

func NewEventsHandler(broadcaster *service.EventBroadcaster) *EventsHandler {
	return &EventsHandler{broadcaster: broadcaster}
}

// Stream SSE 长连接。JWT/API Key 中间件之后。
// 心跳：每 25s 发一条 comment 行（: keepalive）保持连接不被代理断开。
func (h *EventsHandler) Stream(c *gin.Context) {
	if h.broadcaster == nil {
		response.Error(c, apperror.Internal("EVENTS_DISABLED", "事件广播器未启用", nil))
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲
	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		response.Error(c, apperror.Internal("EVENTS_STREAM_UNSUPPORTED", "当前连接不支持 SSE", nil))
		return
	}
	// 首先发送一次 hello 让客户端确认连通
	_, _ = fmt.Fprintf(c.Writer, ": connected %d\n\n", time.Now().Unix())
	flusher.Flush()

	ch, cancel := h.broadcaster.Subscribe(32)
	defer cancel()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(c.Writer, ": heartbeat %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		case envelope, ok := <-ch:
			if !ok {
				return
			}
			if err := writeEventEnvelope(c.Writer, envelope); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEventEnvelope(writer io.Writer, envelope service.EventEnvelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", envelope.Type, data)
	return err
}
