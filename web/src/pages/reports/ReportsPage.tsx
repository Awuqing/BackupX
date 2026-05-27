import { Button, Card, Grid, Message, Select, Space, Statistic, Table, Tag, Typography } from '@arco-design/web-react'
import { IconDownload, IconRefresh } from '@arco-design/web-react/icon'
import { useCallback, useEffect, useState } from 'react'
import { downloadComplianceCSV, fetchComplianceReport } from '../../services/reports'
import type { ComplianceReport, ComplianceRisk, ComplianceTaskRow } from '../../types/reports'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatPercent } from '../../utils/format'

const { Row, Col } = Grid
const { Title, Text } = Typography

const rangeOptions = [
  { label: '近 7 天', value: 7 },
  { label: '近 30 天', value: 30 },
  { label: '近 90 天', value: 90 },
  { label: '近 180 天', value: 180 },
  { label: '近 365 天', value: 365 },
]

function riskTag(risk: ComplianceRisk) {
  switch (risk) {
    case 'ok':
      return <Tag color="green">合规</Tag>
    case 'at_risk':
      return <Tag color="red">风险</Tag>
    default:
      return <Tag color="gray">未启用</Tag>
  }
}

export function ReportsPage() {
  const [days, setDays] = useState(30)
  const [report, setReport] = useState<ComplianceReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [exporting, setExporting] = useState(false)

  const loadData = useCallback(async (range: number) => {
    setLoading(true)
    try {
      const data = await fetchComplianceReport(range)
      setReport(data)
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载合规报表失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadData(days)
  }, [days, loadData])

  async function handleExport() {
    setExporting(true)
    try {
      await downloadComplianceCSV(days)
      Message.success('已导出 CSV')
    } catch (e) {
      Message.error(resolveErrorMessage(e, '导出失败'))
    } finally {
      setExporting(false)
    }
  }

  const summary = report?.summary

  const columns = [
    {
      title: '任务',
      dataIndex: 'taskName',
      render: (_: unknown, row: ComplianceTaskRow) => (
        <Space direction="vertical" size={2}>
          <Text style={{ fontWeight: 600 }}>{row.taskName}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {row.type} · {row.nodeName || '本机'}
          </Text>
        </Space>
      ),
    },
    { title: '状态', dataIndex: 'risk', render: (_: unknown, row: ComplianceTaskRow) => riskTag(row.risk) },
    {
      title: '成功率',
      dataIndex: 'successRate',
      render: (value: number, row: ComplianceTaskRow) => (row.totalRuns > 0 ? formatPercent(value) : '—'),
    },
    {
      title: '周期内(成功/失败)',
      dataIndex: 'totalRuns',
      render: (_: unknown, row: ComplianceTaskRow) => `${row.successes} / ${row.failures}`,
    },
    {
      title: '最近成功',
      dataIndex: 'lastSuccessAt',
      render: (value?: string) => (value ? formatDateTime(value) : <Text type="secondary">从未</Text>),
    },
    { title: '保护量', dataIndex: 'protectedBytes', render: (value: number) => formatBytes(value) },
    {
      title: '加密',
      dataIndex: 'encrypted',
      render: (value: boolean) =>
        value ? <Tag color="arcoblue" size="small">已加密</Tag> : <Text type="secondary">否</Text>,
    },
    { title: 'SLA(RPO)', dataIndex: 'slaHoursRpo', render: (value: number) => (value > 0 ? `${value}h` : '—') },
  ]

  const statCards = [
    { title: '受保护任务', value: summary?.enabledTasks ?? 0, suffix: `/ ${summary?.totalTasks ?? 0}` },
    { title: '合规任务', value: summary?.compliantTasks ?? 0, color: 'rgb(var(--green-6))' },
    { title: '风险任务', value: summary?.atRiskTasks ?? 0, color: 'rgb(var(--red-6))' },
    { title: '已加密任务', value: summary?.encryptedTasks ?? 0 },
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
        <div>
          <Title heading={5} style={{ margin: 0 }}>
            合规报表
          </Title>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {report
              ? `生成于 ${formatDateTime(report.generatedAt)} · 统计窗口 ${report.rangeDays} 天`
              : '按任务的备份合规证据，可导出归档以供审计'}
          </Text>
        </div>
        <Space>
          <Select value={days} onChange={(value) => setDays(value as number)} options={rangeOptions} style={{ width: 130 }} />
          <Button icon={<IconRefresh />} onClick={() => void loadData(days)} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<IconDownload />} onClick={() => void handleExport()} loading={exporting} disabled={loading}>
            导出 CSV
          </Button>
        </Space>
      </div>

      <Row gutter={16}>
        {statCards.map((card) => (
          <Col span={6} key={card.title}>
            <Card>
              <Statistic
                title={card.title}
                value={card.value}
                suffix={card.suffix}
                groupSeparator
                styleValue={card.color ? { color: card.color } : undefined}
              />
            </Card>
          </Col>
        ))}
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Card>
            <Statistic title="整体成功率" value={summary ? Number((summary.overallSuccessRate * 100).toFixed(1)) : 0} suffix="%" />
          </Card>
        </Col>
        <Col span={12}>
          <Card>
            <Statistic title="受保护数据总量" value={formatBytes(summary?.totalProtectedBytes)} />
          </Card>
        </Col>
      </Row>

      {error ? (
        <Card>
          <Text type="error">{error}</Text>
        </Card>
      ) : (
        <Card>
          <Table
            rowKey="taskId"
            loading={loading}
            columns={columns}
            data={report?.tasks ?? []}
            pagination={{ pageSize: 20, sizeCanChange: true }}
            border={false}
          />
        </Card>
      )}
    </Space>
  )
}
