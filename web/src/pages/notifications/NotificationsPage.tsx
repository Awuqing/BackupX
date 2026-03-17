import { Button, Card, Empty, Message, PageHeader, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { NotificationFormDrawer } from '../../components/notifications/NotificationFormDrawer'
import { getNotificationTypeLabel } from '../../components/notifications/field-config'
import { createNotification, deleteNotification, getNotification, listNotifications, testNotification, testSavedNotification, updateNotification } from '../../services/notifications'
import type { NotificationDetail, NotificationPayload, NotificationSummary } from '../../types/notifications'
import { resolveErrorMessage } from '../../utils/error'
import { formatDateTime } from '../../utils/format'

export function NotificationsPage() {
  const [items, setItems] = useState<NotificationSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [testing, setTesting] = useState(false)
  const [drawerVisible, setDrawerVisible] = useState(false)
  const [editingItem, setEditingItem] = useState<NotificationDetail | null>(null)
  const [error, setError] = useState('')

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const result = await listNotifications()
      setItems(result)
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载通知配置失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadData()
  }, [loadData])

  async function openEdit(id: number) {
    setSubmitting(true)
    try {
      const detail = await getNotification(id)
      setEditingItem(detail)
      setDrawerVisible(true)
    } catch (loadError) {
      Message.error(resolveErrorMessage(loadError, '加载通知详情失败'))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSubmit(value: NotificationPayload, notificationId?: number) {
    setSubmitting(true)
    try {
      if (notificationId) {
        await updateNotification(notificationId, value)
        Message.success('通知配置已更新')
      } else {
        await createNotification(value)
        Message.success('通知配置已创建')
      }
      setDrawerVisible(false)
      setEditingItem(null)
      await loadData()
    } catch (submitError) {
      Message.error(resolveErrorMessage(submitError, '保存通知配置失败'))
      throw submitError
    } finally {
      setSubmitting(false)
    }
  }

  async function handleTest(value: NotificationPayload, notificationId?: number) {
    setTesting(true)
    try {
      if (notificationId) {
        await testSavedNotification(notificationId)
      } else {
        await testNotification(value)
      }
      Message.success('测试通知已发出，请查收')
    } catch (testError) {
      Message.error(resolveErrorMessage(testError, '发送测试通知失败'))
      throw testError
    } finally {
      setTesting(false)
    }
  }

  async function handleDelete(item: NotificationSummary) {
    if (!window.confirm(`确定删除通知配置“${item.name}”吗？`)) {
      return
    }
    try {
      await deleteNotification(item.id)
      Message.success('通知配置已删除')
      await loadData()
    } catch (deleteError) {
      Message.error(resolveErrorMessage(deleteError, '删除通知配置失败'))
    }
  }

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      render: (_: unknown, record: NotificationSummary) => (
        <Space direction="vertical" size={2}>
          <Typography.Text bold>{record.name}</Typography.Text>
          <Space>
            {getNotificationTypeLabel(record.type) && <Tag color="arcoblue" bordered>{getNotificationTypeLabel(record.type)}</Tag>}
            {record.enabled !== undefined && <Tag color={record.enabled ? 'green' : 'gray'} bordered>{record.enabled ? '已启用' : '已停用'}</Tag>}
          </Space>
        </Space>
      ),
    },
    {
      title: '触发条件',
      dataIndex: 'events',
      render: (_: unknown, record: NotificationSummary) => (
        <Space>
          {record.onSuccess ? <Tag color="green" bordered>成功</Tag> : null}
          {record.onFailure ? <Tag color="red" bordered>失败</Tag> : null}
          {!record.onSuccess && !record.onFailure ? <Tag color="gray" bordered>未配置</Tag> : null}
        </Space>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      render: (value: string) => formatDateTime(value),
    },
    {
      title: '操作',
      dataIndex: 'actions',
      width: 180,
      render: (_: unknown, record: NotificationSummary) => (
        <Space>
          <Button size="small" type="text" onClick={() => void openEdit(record.id)}>
            编辑
          </Button>
          <Button size="small" type="text" status="danger" onClick={() => void handleDelete(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 16 }}
        title="通知配置"
        subTitle="配置 Email、Webhook 与 Telegram 渠道，并控制成功/失败事件的发送策略"
        extra={
          <Button
            type="primary"
            onClick={() => {
              setEditingItem(null)
              setDrawerVisible(true)
            }}
          >
            新建通知
          </Button>
        }
      />

      {error ? <Card><Typography.Text type="error">{error}</Typography.Text></Card> : null}

      <Card>
        <Table rowKey="id" loading={loading} columns={columns} data={items} pagination={{ pageSize: 10 }} stripe noDataElement={<Empty description="暂无通知配置，请先创建" />} />
      </Card>

      <NotificationFormDrawer
        visible={drawerVisible}
        loading={submitting}
        testing={testing}
        initialValue={editingItem}
        onCancel={() => {
          setDrawerVisible(false)
          setEditingItem(null)
        }}
        onSubmit={handleSubmit}
        onTest={handleTest}
      />
    </Space>
  )
}
