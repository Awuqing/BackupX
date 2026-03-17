import { Alert, Card, Descriptions, Space, Spin, Typography } from '@arco-design/web-react'
import { useEffect, useState } from 'react'
import axios from 'axios'
import { fetchSystemInfo, type SystemInfo } from '../../services/system'

function resolveErrorMessage(error: unknown) {
  if (axios.isAxiosError(error)) {
    return error.response?.data?.message ?? '加载系统信息失败'
  }
  return '加载系统信息失败'
}

export function SystemInfoPage() {
  const [data, setData] = useState<SystemInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let mounted = true
    void (async () => {
      try {
        const result = await fetchSystemInfo()
        if (mounted) {
          setData(result)
        }
      } catch (err) {
        if (mounted) {
          setError(resolveErrorMessage(err))
        }
      } finally {
        if (mounted) {
          setLoading(false)
        }
      }
    })()
    return () => {
      mounted = false
    }
  }, [])

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>系统信息</Typography.Title>
        <Typography.Paragraph type="secondary">
          用于确认服务版本、运行模式、数据库位置与运行时长。
        </Typography.Paragraph>
      </div>

      <Card>
        {loading ? (
          <Spin />
        ) : error ? (
          <Alert type="error" content={error} />
        ) : (
          <Descriptions column={1} border data={[
            { label: '版本', value: data?.version ?? '-' },
            { label: '运行模式', value: data?.mode ?? '-' },
            { label: '启动时间', value: data?.startedAt ?? '-' },
            { label: '运行秒数', value: data?.uptimeSeconds ?? '-' },
            { label: '数据库路径', value: data?.databasePath ?? '-' },
          ]} />
        )}
      </Card>
    </Space>
  )
}
