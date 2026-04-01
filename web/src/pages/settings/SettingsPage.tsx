import { Badge, Button, Card, Descriptions, Grid, Link, PageHeader, Space, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useState } from 'react'
import { fetchSystemInfo, checkUpdate, type SystemInfo, type UpdateCheckResult } from '../../services/system'
import { resolveErrorMessage } from '../../utils/error'
import { formatDuration } from '../../utils/format'

const { Row, Col } = Grid

function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(1)} ${units[i]}`
}

export function SettingsPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [updateResult, setUpdateResult] = useState<UpdateCheckResult | null>(null)
  const [checking, setChecking] = useState(false)

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const result = await fetchSystemInfo()
        if (active) { setInfo(result); setError('') }
      } catch (loadError) {
        if (active) setError(resolveErrorMessage(loadError, '加载系统信息失败'))
      } finally {
        if (active) setLoading(false)
      }
    })()
    return () => { active = false }
  }, [])

  async function handleCheckUpdate() {
    setChecking(true)
    try {
      const result = await checkUpdate()
      setUpdateResult(result)
    } catch (e) {
      setUpdateResult({ currentVersion: info?.version || '-', latestVersion: '-', hasUpdate: false, error: resolveErrorMessage(e, '检查更新失败') })
    } finally {
      setChecking(false)
    }
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader style={{ paddingBottom: 16 }} title="系统设置" subTitle="运行信息、磁盘状态与版本更新">
        {error ? <Typography.Text type="error">{error}</Typography.Text> : null}
      </PageHeader>

      <Row gutter={16}>
        <Col span={12}>
          <Card loading={loading} title="运行信息">
            <Descriptions column={1} border data={[
              { label: '版本', value: <Space>{info?.version ?? '-'}<Button size="mini" type="text" loading={checking} onClick={handleCheckUpdate}>检查更新</Button></Space> },
              { label: '运行模式', value: info?.mode === 'release' ? <Tag color="green">生产</Tag> : <Tag color="orange">{info?.mode ?? '-'}</Tag> },
              { label: '运行时长', value: formatDuration(info?.uptimeSeconds) },
              { label: '启动时间', value: info?.startedAt ?? '-' },
              { label: '数据库路径', value: <Typography.Text copyable>{info?.databasePath ?? '-'}</Typography.Text> },
            ]} />
          </Card>
        </Col>
        <Col span={12}>
          <Card loading={loading} title="磁盘状态">
            <Descriptions column={1} border data={[
              { label: '总空间', value: formatBytes(info?.diskTotal) },
              { label: '已用空间', value: formatBytes(info?.diskUsed) },
              { label: '可用空间', value: formatBytes(info?.diskFree) },
              { label: '使用率', value: info?.diskTotal ? `${((info.diskUsed / info.diskTotal) * 100).toFixed(1)}%` : '-' },
            ]} />
          </Card>
        </Col>
      </Row>

      {/* 更新检查结果 */}
      {updateResult && (
        <Card title="版本更新">
          {updateResult.error ? (
            <Typography.Text type="warning">{updateResult.error}</Typography.Text>
          ) : updateResult.hasUpdate ? (
            <Space direction="vertical" size="medium" style={{ width: '100%' }}>
              <Space>
                <Badge status="processing" />
                <Typography.Text style={{ fontWeight: 600 }}>
                  有新版本可用：{updateResult.latestVersion}
                </Typography.Text>
                <Typography.Text type="secondary">（当前：{updateResult.currentVersion}）</Typography.Text>
              </Space>
              {updateResult.publishedAt && (
                <Typography.Text type="secondary">发布时间：{new Date(updateResult.publishedAt).toLocaleString()}</Typography.Text>
              )}
              {updateResult.releaseNotes && (
                <Card size="small" title="更新说明" style={{ maxHeight: 200, overflow: 'auto' }}>
                  <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>{updateResult.releaseNotes}</Typography.Paragraph>
                </Card>
              )}
              <Space>
                {updateResult.downloadUrl && (
                  <Link href={updateResult.downloadUrl} target="_blank">
                    <Button type="primary">下载二进制包</Button>
                  </Link>
                )}
                {updateResult.releaseUrl && (
                  <Link href={updateResult.releaseUrl} target="_blank">
                    <Button type="outline">查看 Release 详情</Button>
                  </Link>
                )}
              </Space>
              {updateResult.dockerImage && (
                <Card size="small" title="Docker 更新命令">
                  <Typography.Paragraph copyable code style={{ marginBottom: 0 }}>
                    {`docker pull ${updateResult.dockerImage}:${updateResult.latestVersion} && docker compose up -d`}
                  </Typography.Paragraph>
                </Card>
              )}
            </Space>
          ) : (
            <Space>
              <Badge status="success" />
              <Typography.Text>当前已是最新版本 ({updateResult.currentVersion})</Typography.Text>
            </Space>
          )}
        </Card>
      )}
    </Space>
  )
}
