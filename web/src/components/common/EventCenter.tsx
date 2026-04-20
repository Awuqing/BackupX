import { Badge, Button, Drawer, Empty, Notification, Space, Tag, Typography } from '@arco-design/web-react'
import { IconNotification } from '@arco-design/web-react/icon'
import { useEffect, useState } from 'react'
import { useEventStream, type SystemEvent } from '../../hooks/useEventStream'
import { useEventStore } from '../../stores/events'
import { formatDateTime } from '../../utils/format'

// EVENT_CONFIG 把后端事件类型映射到 UI 展示。
// Toast 类型决定颜色；label 中文化；silent=true 则不弹 Toast（仅进历史）。
const EVENT_CONFIG: Record<string, { label: string; toast?: 'success' | 'error' | 'warning' | 'info'; color: string }> = {
  backup_success: { label: '备份成功', toast: 'success', color: 'green' },
  backup_failed: { label: '备份失败', toast: 'error', color: 'red' },
  restore_success: { label: '恢复成功', toast: 'success', color: 'green' },
  restore_failed: { label: '恢复失败', toast: 'error', color: 'red' },
  verify_failed: { label: '验证未通过', toast: 'error', color: 'red' },
  sla_violation: { label: 'SLA 违约', toast: 'warning', color: 'orange' },
  storage_unhealthy: { label: '存储不可用', toast: 'error', color: 'red' },
  storage_capacity_warning: { label: '存储容量预警', toast: 'warning', color: 'orange' },
  replication_failed: { label: '复制失败', toast: 'error', color: 'red' },
  agent_outdated: { label: 'Agent 版本过期', toast: 'warning', color: 'orange' },
}

function labelFor(type: string): string {
  return EVENT_CONFIG[type]?.label ?? type
}

/**
 * EventCenter 头部的事件通知中心。
 * - Bell 图标 + 未读徽章
 * - SSE 事件到达时弹右下角 Toast + 进入历史
 * - 点击 Bell 打开右侧抽屉查看历史
 */
export function EventCenter() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const events = useEventStore((s) => s.events)
  const unreadCount = useEventStore((s) => s.unreadCount)
  const addEvent = useEventStore((s) => s.addEvent)
  const markAllRead = useEventStore((s) => s.markAllRead)
  const clear = useEventStore((s) => s.clear)

  // 订阅全部事件（ Dashboard 的 useEventStream 另一套实例只过滤 Dashboard 关心的事件）
  useEventStream((event: SystemEvent) => {
    addEvent(event)
    const config = EVENT_CONFIG[event.type]
    if (config?.toast) {
      const fn = Notification[config.toast]
      fn({
        title: event.title || config.label,
        content: event.body,
        duration: config.toast === 'error' ? 6000 : 3500,
      })
    }
  })

  // 打开抽屉后自动标记已读
  useEffect(() => {
    if (drawerOpen && unreadCount > 0) {
      markAllRead()
    }
  }, [drawerOpen, unreadCount, markAllRead])

  return (
    <>
      <Badge count={unreadCount} dot={unreadCount > 0 && unreadCount <= 0}>
        <Button
          type="text"
          icon={<IconNotification />}
          onClick={() => setDrawerOpen(true)}
        >
          {unreadCount > 0 ? `${unreadCount}` : ''}
        </Button>
      </Badge>

      <Drawer
        visible={drawerOpen}
        title={
          <Space>
            <span>实时事件</span>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              最近 {events.length} 条（会话内）
            </Typography.Text>
          </Space>
        }
        width={480}
        onCancel={() => setDrawerOpen(false)}
        footer={
          <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
            <Button size="small" onClick={clear} disabled={events.length === 0}>清空</Button>
          </Space>
        }
      >
        {events.length === 0 ? (
          <Empty description="暂无事件。事件仅在当前会话内保留。" />
        ) : (
          <div>
            {events.map((e) => {
              const config = EVENT_CONFIG[e.type]
              return (
                <div
                  key={e.id}
                  style={{
                    padding: '8px 12px',
                    marginBottom: 8,
                    borderRadius: 4,
                    backgroundColor: e.read ? 'var(--color-bg-1)' : 'var(--color-primary-light-1)',
                    border: '1px solid var(--color-border-2)',
                  }}
                >
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Space>
                      <Tag color={config?.color ?? 'gray'} bordered size="small">{labelFor(e.type)}</Tag>
                      <Typography.Text bold style={{ fontSize: 13 }}>{e.title}</Typography.Text>
                    </Space>
                    <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                      {formatDateTime(e.timestamp)}
                    </Typography.Text>
                  </Space>
                  {e.body ? (
                    <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 4, marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                      {e.body}
                    </Typography.Paragraph>
                  ) : null}
                </div>
              )
            })}
          </div>
        )}
      </Drawer>
    </>
  )
}
