import { Card, Descriptions, Grid, PageHeader, Space, Typography } from '@arco-design/web-react'
import { useEffect, useState } from 'react'
import { fetchSystemInfo, type SystemInfo } from '../../services/system'
import { resolveErrorMessage } from '../../utils/error'
import { formatDuration } from '../../utils/format'

const { Row, Col } = Grid

const deploySteps = [
  '1. 构建前端：cd web && npm run build',
  '2. 编译后端：cd server && go build -o backupx ./cmd/backupx',
  '3. 部署静态资源与二进制，并按 deploy/ 目录提供的配置接入 Nginx 与 systemd',
  '4. 首次启动后访问 Web 控制台，完成管理员初始化与存储目标配置',
]

export function SettingsPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const result = await fetchSystemInfo()
        if (active) {
          setInfo(result)
          setError('')
        }
      } catch (loadError) {
        if (active) {
          setError(resolveErrorMessage(loadError, '加载系统设置失败'))
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    })()
    return () => {
      active = false
    }
  }, [])

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 16 }}
        title="系统设置"
        subTitle="展示当前运行信息、部署入口和交付所需的基础操作说明"
      >
        {error ? <Typography.Text type="error">{error}</Typography.Text> : null}
      </PageHeader>

      <Row gutter={16}>
        <Col span={12}>
          <Card loading={loading} title="运行信息">
            <Descriptions
              column={1}
              border
              data={[
                { label: '版本', value: info?.version ?? '-' },
                { label: '运行模式', value: info?.mode ?? '-' },
                { label: '运行时长', value: formatDuration(info?.uptimeSeconds) },
                { label: '启动时间', value: info?.startedAt ?? '-' },
                { label: '数据库路径', value: info?.databasePath ?? '-' },
              ]}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="部署资产">
            <Space direction="vertical" size="medium" style={{ width: '100%' }}>
              <Typography.Text>`deploy/nginx.conf`：静态资源托管与 `/api` 反向代理示例。</Typography.Text>
              <Typography.Text>`deploy/backupx.service`：systemd 服务单元，负责守护 API 进程。</Typography.Text>
              <Typography.Text>`deploy/install.sh`：一键安装示例脚本，用于创建目录、复制文件并启动服务。</Typography.Text>
              <Typography.Text>`README.md`：包含完整部署与使用文档。</Typography.Text>
            </Space>
          </Card>
        </Col>
      </Row>

      <Card title="部署步骤">
        <div className="code-block">{deploySteps.join('\n')}</div>
      </Card>
    </Space>
  )
}
