import { Alert, Button, Card, Empty, Grid, Message, PageHeader, Progress, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import axios from 'axios'
import { useCallback, useEffect, useState } from 'react'
import {
  createStorageTarget,
  deleteStorageTarget,
  getStorageTarget,
  getStorageTargetUsage,
  listStorageTargets,
  startGoogleDriveAuth,
  testSavedStorageTarget,
  testStorageTarget,
  toggleStorageTargetStar,
  type StorageTargetUsage,
  updateStorageTarget,
} from '../../services/storage-targets'
import { formatBytes } from '../../utils/format'
import type { StorageConnectionTestResult, StorageTargetDetail, StorageTargetPayload, StorageTargetSummary } from '../../types/storage-targets'
import { getStorageTargetTypeLabel } from '../../components/storage-targets/field-config'
import { StorageTargetFormDrawer } from '../../components/storage-targets/StorageTargetFormDrawer'

function resolveErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    return error.response?.data?.message ?? '请求失败，请稍后重试'
  }
  return '请求失败，请稍后重试'
}

function renderTestStatus(target: StorageTargetSummary) {
  switch (target.lastTestStatus) {
    case 'success':
      return <Tag color="green" bordered>连接正常</Tag>
    case 'failed':
      return <Tag color="red" bordered>最近测试失败</Tag>
    default:
      return <Tag color="arcoblue" bordered>未测试</Tag>
  }
}

export function StorageTargetsPage() {
  const [targets, setTargets] = useState<StorageTargetSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [testing, setTesting] = useState(false)
  const [drawerVisible, setDrawerVisible] = useState(false)
  const [editingTarget, setEditingTarget] = useState<StorageTargetDetail | null>(null)
  const [error, setError] = useState('')
  const [usageMap, setUsageMap] = useState<Record<number, StorageTargetUsage>>({})

  const loadTargets = useCallback(async () => {
    setLoading(true)
    try {
      const result = await listStorageTargets()
      setTargets(result)
      setError('')
      // 异步加载每个启用目标的使用量（容量 About）。失败不阻塞列表展示。
      const usageEntries = await Promise.all(
        result.filter((t) => t.enabled).map(async (t) => {
          try {
            const u = await getStorageTargetUsage(t.id)
            return [t.id, u] as const
          } catch {
            return null
          }
        }),
      )
      const next: Record<number, StorageTargetUsage> = {}
      for (const entry of usageEntries) {
        if (entry) next[entry[0]] = entry[1]
      }
      setUsageMap(next)
    } catch (loadError) {
      setError(resolveErrorMessage(loadError))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadTargets()
  }, [loadTargets])

  // Auto-refresh when user comes back from Google Drive OAuth tab
  useEffect(() => {
    function handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        void loadTargets()
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [loadTargets])

  async function openEdit(id: number) {
    setSubmitting(true)
    try {
      const detail = await getStorageTarget(id)
      setEditingTarget(detail)
      setDrawerVisible(true)
    } catch (loadError) {
      Message.error(resolveErrorMessage(loadError))
    } finally {
      setSubmitting(false)
    }
  }

  async function handleSubmit(value: StorageTargetPayload, targetId?: number) {
    setSubmitting(true)
    try {
      if (targetId) {
        await updateStorageTarget(targetId, value)
        Message.success('存储目标已更新')
      } else {
        await createStorageTarget(value)
        Message.success('存储目标已创建')
      }
      setDrawerVisible(false)
      setEditingTarget(null)
      await loadTargets()
    } catch (submitError) {
      Message.error(resolveErrorMessage(submitError))
      throw submitError
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm('确定删除该存储目标吗？')) {
      return
    }
    try {
      await deleteStorageTarget(id)
      Message.success('存储目标已删除')
      await loadTargets()
    } catch (deleteError) {
      Message.error(resolveErrorMessage(deleteError))
    }
  }

  async function handleDraftTest(value: StorageTargetPayload, targetId?: number): Promise<StorageConnectionTestResult> {
    setTesting(true)
    try {
      // When editing an existing target, use saved config test to avoid sending masked values
      const result = targetId
        ? await testSavedStorageTarget(targetId)
        : await testStorageTarget(value)
      Message.success(result.message)
      if (targetId) {
        await loadTargets()
      }
      return result
    } catch (testError) {
      const message = resolveErrorMessage(testError)
      Message.error(message)
      return { success: false, message }
    } finally {
      setTesting(false)
    }
  }

  async function handleSavedTest(id: number) {
    try {
      const result = await testSavedStorageTarget(id)
      Message.success(result.message)
      await loadTargets()
    } catch (testError) {
      Message.error(resolveErrorMessage(testError))
    }
  }

  async function handleToggleStar(id: number) {
    try {
      await toggleStorageTargetStar(id)
      await loadTargets()
    } catch (starError) {
      Message.error(resolveErrorMessage(starError))
    }
  }

  async function handleGoogleDriveAuth(value: StorageTargetPayload, targetId?: number) {
    try {
      const result = await startGoogleDriveAuth(value, targetId)
      window.open(result.authUrl, '_blank')
    } catch (authError) {
      Message.error(resolveErrorMessage(authError))
      throw authError
    }
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 16 }}
        title="存储目标"
        subTitle="管理本地磁盘、S3 Compatible、WebDAV 与 Google Drive 等备份目标"
        extra={
          <Button
            type="primary"
            onClick={() => {
              setEditingTarget(null)
              setDrawerVisible(true)
            }}
          >
            新建存储目标
          </Button>
        }
      />

      {error ? <Alert type="error" content={error} /> : null}

      {loading ? (
        <Spin />
      ) : targets.length === 0 ? (
        <Card>
          <Empty description="暂无存储目标，请先创建一个备份落地点。" />
        </Card>
      ) : (
        <Grid.Row gutter={[16, 16]}>
          {targets.map((target) => (
            <Grid.Col span={8} key={target.id}>
              <Card style={{ height: '100%' }}>
                <Space direction="vertical" size="medium" style={{ width: '100%' }}>
                  <Space size="large" align="start" style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
                    <div>
                      <Typography.Title heading={6} style={{ marginBottom: 4 }}>
                        {target.starred ? '★ ' : ''}{target.name}
                      </Typography.Title>
                      <Space>
                        {getStorageTargetTypeLabel(target.type) && <Tag color="arcoblue" bordered>{getStorageTargetTypeLabel(target.type)}</Tag>}
                        {target.enabled ? <Tag color="green" bordered>已启用</Tag> : <Tag color="gray" bordered>已停用</Tag>}
                        {renderTestStatus(target)}
                      </Space>
                    </div>
                  </Space>

                  {target.description ? <Typography.Paragraph>{target.description}</Typography.Paragraph> : null}
                  {target.lastTestMessage ? (
                    <Typography.Paragraph type="secondary">最近测试：{target.lastTestMessage}</Typography.Paragraph>
                  ) : null}
                  {(() => {
                    const usage = usageMap[target.id]
                    if (!usage) return null
                    const disk = usage.diskUsage
                    // 优先后端 About（远端真实容量），否则展示"已用量"（累计备份大小）
                    if (disk && disk.total && disk.used !== undefined) {
                      const rate = disk.total > 0 ? disk.used / disk.total : 0
                      const percent = Math.round(rate * 100)
                      const color = rate >= 0.85 ? '#F53F3F' : rate >= 0.7 ? '#FF7D00' : '#00B42A'
                      return (
                        <div>
                          <Space size="mini" style={{ marginBottom: 4 }}>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>使用率 {percent}%</Typography.Text>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              {formatBytes(disk.used)} / {formatBytes(disk.total)}
                            </Typography.Text>
                            {rate >= 0.85 && <Tag color="red" bordered size="small">容量预警</Tag>}
                          </Space>
                          <Progress percent={percent} color={color} size="small" showText={false} />
                        </div>
                      )
                    }
                    if (usage.totalSize > 0) {
                      return (
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          已用备份：{formatBytes(usage.totalSize)}（{usage.recordCount} 个记录）
                        </Typography.Text>
                      )
                    }
                    return null
                  })()}
                  <Typography.Text type="secondary">更新时间：{target.updatedAt}</Typography.Text>

                  <Space wrap size="mini">
                    <Button size="small" type="text" onClick={() => void handleToggleStar(target.id)}>
                      {target.starred ? '取消收藏' : '收藏'}
                    </Button>
                    <Button size="small" type="text" onClick={() => void openEdit(target.id)} loading={submitting && editingTarget?.id === target.id}>
                      编辑
                    </Button>
                    <Button size="small" type="text" onClick={() => void handleSavedTest(target.id)}>
                      测试连接
                    </Button>
                    <Button size="small" type="text" status="danger" onClick={() => void handleDelete(target.id)}>
                      删除
                    </Button>
                  </Space>
                </Space>
              </Card>
            </Grid.Col>
          ))}
        </Grid.Row>
      )}

      <StorageTargetFormDrawer
        visible={drawerVisible}
        loading={submitting}
        testing={testing}
        initialValue={editingTarget}
        onCancel={() => {
          setDrawerVisible(false)
          setEditingTarget(null)
        }}
        onSubmit={handleSubmit}
        onTest={handleDraftTest}
        onGoogleDriveAuth={handleGoogleDriveAuth}
      />
    </Space>
  )
}
