import { Avatar, Card, Empty, Grid, PageHeader, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { IconCheckCircle, IconHistory, IconSave, IconStorage } from '@arco-design/web-react/icon'
import ReactEChartsCore from 'echarts-for-react/lib/core'
import * as echarts from 'echarts/core'
import { LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useEffect, useMemo, useState } from 'react'
import { fetchDashboardStats, fetchDashboardTimeline } from '../../services/dashboard'
import { useAuthStore } from '../../stores/auth'
import type { BackupTimelinePoint, DashboardStats } from '../../types/dashboard'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatPercent } from '../../utils/format'

echarts.use([LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const { Row, Col } = Grid

export function DashboardPage() {
  const user = useAuthStore((state) => state.user)
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [timeline, setTimeline] = useState<BackupTimelinePoint[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    void (async () => {
      setLoading(true)
      try {
        const [statsResult, timelineResult] = await Promise.all([fetchDashboardStats(), fetchDashboardTimeline(30)])
        if (!active) {
          return
        }
        setStats(statsResult)
        setTimeline(timelineResult || [])
        setError('')
      } catch (loadError) {
        if (active) {
          setError(resolveErrorMessage(loadError, '加载仪表盘失败'))
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

  const cards = useMemo(
    () => [
      { label: '备份任务', value: stats?.totalTasks ?? 0, helper: `${stats?.enabledTasks ?? 0} 个已启用`, icon: <IconStorage />, color: 'var(--color-primary-6)', bg: 'var(--color-primary-1)' },
      { label: '成功率', value: formatPercent(stats?.successRate), helper: '最近 30 天', icon: <IconCheckCircle />, color: 'var(--color-success-6)', bg: 'var(--color-success-1)' },
      { label: '总备份量', value: formatBytes(stats?.totalBackupBytes), helper: '历史累计', icon: <IconSave />, color: 'var(--color-purple-6)', bg: 'var(--color-purple-1)' },
      { label: '最近备份', value: stats?.totalRecords ?? 0, helper: formatDateTime(stats?.lastBackupAt), icon: <IconHistory />, color: 'var(--color-warning-6)', bg: 'var(--color-warning-1)' },
    ],
    [stats],
  )

  const timelineChartOption = useMemo(() => ({
    tooltip: { trigger: 'axis' as const },
    legend: { data: ['成功', '失败'], bottom: 0 },
    grid: { left: 40, right: 20, top: 40, bottom: 40 },
    xAxis: {
      type: 'category' as const,
      data: timeline.map((p) => p.date),
      axisLabel: { rotate: 45, fontSize: 11, color: 'var(--color-text-3)' },
      axisLine: { lineStyle: { color: 'var(--color-border-2)' } },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value' as const,
      minInterval: 1,
      axisLabel: { color: 'var(--color-text-3)' },
      splitLine: { lineStyle: { type: 'dashed', color: 'var(--color-border-2)' } },
    },
    series: [
      {
        name: '成功',
        type: 'line' as const,
        smooth: true,
        data: timeline.map((p) => p.success),
        itemStyle: { color: 'var(--color-primary-6)' },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(52,145,250,0.25)' },
          { offset: 1, color: 'rgba(52,145,250,0.02)' },
        ]) },
        symbolSize: 6,
      },
      {
        name: '失败',
        type: 'line' as const,
        smooth: true,
        data: timeline.map((p) => p.failed),
        itemStyle: { color: 'var(--color-danger-light-4)' },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(245,63,63,0.15)' },
          { offset: 1, color: 'rgba(245,63,63,0.01)' },
        ]) },
        symbolSize: 6,
      },
    ],
  }), [timeline])

  const storageChartOption = useMemo(() => {
    const data = (stats?.storageUsage ?? []).map((s) => ({
      name: s.targetName || '未命名',
      value: s.totalSize,
    }))
    return {
      tooltip: {
        trigger: 'item' as const,
        formatter: (params: { name: string; value: number; percent: number }) =>
          `${params.name}: ${formatBytes(params.value)} (${params.percent}%)`,
      },
      legend: { bottom: 0, type: 'scroll' as const },
      series: [
        {
          type: 'pie' as const,
          radius: ['50%', '70%'],
          avoidLabelOverlap: false,
          itemStyle: { borderRadius: 6, borderColor: 'var(--color-bg-2)', borderWidth: 2 },
          label: { show: false },
          emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold' } },
          data,
          color: ['#165DFF', '#14C9C9', '#FADC19', '#FF7D00', '#F53F3F', '#722ED1'],
        },
      ],
    }
  }, [stats])

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <PageHeader
        style={{ paddingBottom: 16 }}
        title={`欢迎回来，${user?.displayName ?? user?.username ?? '管理员'}`}
        subTitle="快速查看备份执行健康度、最近记录和各存储目标使用量"
      >
        {error ? <Typography.Text type="error">{error}</Typography.Text> : null}
      </PageHeader>

      <Row gutter={16}>
        {cards.map((card) => (
          <Col key={card.label} span={6}>
            <Card loading={loading} hoverable>
              <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
                <Avatar shape="square" size={54} style={{ borderRadius: 12, backgroundColor: card.bg, color: card.color }}>
                  {card.icon}
                </Avatar>
                <div>
                  <Typography.Text type="secondary" style={{ fontSize: 13 }}>{card.label}</Typography.Text>
                  <Typography.Title heading={4} style={{ margin: '4px 0 2px' }}>
                    {card.value}
                  </Typography.Title>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>{card.helper}</Typography.Text>
                </div>
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={16}>
        <Col span={14}>
          <Card loading={loading} title="最近 30 天备份趋势">
            {timeline.length > 0 ? (
              <ReactEChartsCore echarts={echarts} option={timelineChartOption} style={{ height: 300 }} />
            ) : (
              <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography.Text type="secondary">暂无数据</Typography.Text>
              </div>
            )}
          </Card>
        </Col>
        <Col span={10}>
          <Card loading={loading} title="存储使用量分布">
            {(stats?.storageUsage ?? []).length > 0 ? (
              <ReactEChartsCore echarts={echarts} option={storageChartOption} style={{ height: 300 }} />
            ) : (
              <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography.Text type="secondary">暂无存储数据</Typography.Text>
              </div>
            )}
          </Card>
        </Col>
      </Row>

      <Card loading={loading} title="最近备份记录">
        <Table
          noDataElement={<Empty description="暂无近期运行记录" />}
          rowKey="id"
          columns={[
            { title: '任务', dataIndex: 'taskName' },
            {
              title: '状态',
              dataIndex: 'status',
              render: (value: string) => {
                const label = value === 'success' ? '成功' : value === 'failed' ? '失败' : value === 'running' ? '执行中' : value
                return label ? (
                  <Tag color={value === 'success' ? 'green' : value === 'failed' ? 'red' : 'arcoblue'} bordered>
                    {label}
                  </Tag>
                ) : <span style={{ color: 'var(--color-text-3)' }}>-</span>
              },
            },
            { title: '文件大小', dataIndex: 'fileSize', render: (value: number) => formatBytes(value) },
            { title: '开始时间', dataIndex: 'startedAt', render: (value: string) => formatDateTime(value) },
          ]}
          data={stats?.recentRecords ?? []}
          pagination={false}
          stripe
        />
      </Card>
    </Space>
  )
}
