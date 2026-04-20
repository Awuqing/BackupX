import { Alert, Avatar, Card, Empty, Grid, PageHeader, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { IconCheckCircle, IconDesktop, IconHistory, IconSafe, IconSave, IconStorage } from '@arco-design/web-react/icon'
import ReactEChartsCore from 'echarts-for-react/lib/core'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { fetchDashboardBreakdown, fetchDashboardCluster, fetchDashboardNodePerformance, fetchDashboardSLA, fetchDashboardStats, fetchDashboardTimeline } from '../../services/dashboard'
import { useEventStream } from '../../hooks/useEventStream'
import { useAuthStore } from '../../stores/auth'
import type { BackupTimelinePoint, BreakdownStats, ClusterOverview, DashboardStats, NodePerformance, SLAComplianceReport } from '../../types/dashboard'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes, formatDateTime, formatPercent } from '../../utils/format'

echarts.use([BarChart, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const { Row, Col } = Grid

export function DashboardPage() {
  const user = useAuthStore((state) => state.user)
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [timeline, setTimeline] = useState<BackupTimelinePoint[]>([])
  const [sla, setSla] = useState<SLAComplianceReport | null>(null)
  const [cluster, setCluster] = useState<ClusterOverview | null>(null)
  const [breakdown, setBreakdown] = useState<BreakdownStats | null>(null)
  const [nodePerf, setNodePerf] = useState<NodePerformance[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // 统一的数据加载入口。SSE 事件到达时复用该方法刷新。
  const reload = useCallback(async (showLoading = true) => {
    if (showLoading) setLoading(true)
    try {
      const [statsResult, timelineResult, slaResult, clusterResult, breakdownResult, nodePerfResult] = await Promise.all([
        fetchDashboardStats(),
        fetchDashboardTimeline(30),
        fetchDashboardSLA(),
        fetchDashboardCluster(),
        fetchDashboardBreakdown(30),
        fetchDashboardNodePerformance(30),
      ])
      setStats(statsResult)
      setTimeline(timelineResult || [])
      setSla(slaResult)
      setCluster(clusterResult)
      setBreakdown(breakdownResult)
      setNodePerf(nodePerfResult || [])
      setError('')
    } catch (loadError) {
      setError(resolveErrorMessage(loadError, '加载仪表盘失败'))
    } finally {
      if (showLoading) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload(true)
  }, [reload])

  // 订阅实时事件：备份完成 / 恢复完成 / SLA 违约 / 存储健康变化时自动刷新 Dashboard。
  // 只关心会影响 Dashboard 显示的事件，避免无关事件造成频繁重渲染。
  useEventStream(
    () => {
      // debounce 500ms：短时间多条事件合并一次刷新
      void reload(false)
    },
    ['backup_success', 'backup_failed', 'restore_success', 'restore_failed', 'verify_failed', 'sla_violation', 'storage_unhealthy', 'storage_capacity_warning'],
  )

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

  // 任务类型分布（饼图）
  const typeChartOption = useMemo(() => {
    const data = (breakdown?.byType ?? []).map((item) => ({ name: item.label, value: item.count ?? 0 }))
    return {
      tooltip: { trigger: 'item' as const },
      legend: { bottom: 0, type: 'scroll' as const },
      series: [{
        type: 'pie' as const,
        radius: ['45%', '68%'],
        avoidLabelOverlap: false,
        itemStyle: { borderRadius: 6, borderColor: 'var(--color-bg-2)', borderWidth: 2 },
        label: { show: false },
        emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold' } },
        data,
        color: ['#165DFF', '#14C9C9', '#FADC19', '#FF7D00', '#722ED1', '#F53F3F'],
      }],
    }
  }, [breakdown])

  // 节点分布（柱状图）
  const nodeChartOption = useMemo(() => {
    const items = breakdown?.byNode ?? []
    return {
      tooltip: { trigger: 'axis' as const },
      grid: { left: 40, right: 20, top: 20, bottom: 40 },
      xAxis: {
        type: 'category' as const,
        data: items.map((i) => i.label),
        axisLabel: { rotate: 30, fontSize: 11, color: 'var(--color-text-3)' },
        axisTick: { show: false },
        axisLine: { lineStyle: { color: 'var(--color-border-2)' } },
      },
      yAxis: {
        type: 'value' as const,
        minInterval: 1,
        axisLabel: { color: 'var(--color-text-3)' },
        splitLine: { lineStyle: { type: 'dashed', color: 'var(--color-border-2)' } },
      },
      series: [{
        type: 'bar' as const,
        data: items.map((i) => i.count ?? 0),
        itemStyle: { color: 'var(--color-primary-6)', borderRadius: [4, 4, 0, 0] },
        barMaxWidth: 40,
      }],
    }
  }, [breakdown])

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

      {breakdown && ((breakdown.byType ?? []).length > 0 || (breakdown.byNode ?? []).length > 0) ? (
        <Row gutter={16}>
          <Col span={12}>
            <Card loading={loading} title="任务类型分布">
              {(breakdown.byType ?? []).length > 0 ? (
                <ReactEChartsCore echarts={echarts} option={typeChartOption} style={{ height: 260 }} />
              ) : (
                <div style={{ height: 260, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Typography.Text type="secondary">暂无任务</Typography.Text>
                </div>
              )}
            </Card>
          </Col>
          <Col span={12}>
            <Card loading={loading} title="任务按节点分布">
              {(breakdown.byNode ?? []).length > 0 ? (
                <ReactEChartsCore echarts={echarts} option={nodeChartOption} style={{ height: 260 }} />
              ) : (
                <div style={{ height: 260, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Typography.Text type="secondary">暂无数据</Typography.Text>
                </div>
              )}
            </Card>
          </Col>
        </Row>
      ) : null}

      {cluster && cluster.totalNodes > 0 ? (
        <Card loading={loading} title={
          <Space>
            <IconDesktop />
            <span>集群概览</span>
            <Tag bordered>Master {cluster.masterVersion || '-'}</Tag>
          </Space>
        }>
          <Row gutter={16}>
            <Col span={6}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>节点总数</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0' }}>{cluster.totalNodes}</Typography.Title>
              </div>
            </Col>
            <Col span={6}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>在线</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0', color: 'var(--color-success-6)' }}>{cluster.onlineNodes}</Typography.Title>
              </div>
            </Col>
            <Col span={6}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>离线</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0', color: cluster.offlineNodes > 0 ? 'var(--color-danger-6)' : undefined }}>{cluster.offlineNodes}</Typography.Title>
              </div>
            </Col>
            <Col span={6}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>Agent 过期</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0', color: cluster.outdatedAgents > 0 ? 'var(--color-warning-6)' : undefined }}>{cluster.outdatedAgents}</Typography.Title>
              </div>
            </Col>
          </Row>
          <Table
            style={{ marginTop: 16 }}
            rowKey="id"
            stripe
            pagination={false}
            data={cluster.nodes}
            columns={[
              { title: '节点', dataIndex: 'name', render: (v: string, row) => (
                <Space direction="vertical" size={2}>
                  <Typography.Text bold>{v}</Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>{row.hostname || '-'}</Typography.Text>
                </Space>
              )},
              { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={s === 'online' ? 'green' : 'red'} bordered>{s === 'online' ? '在线' : '离线'}</Tag> },
              { title: '版本', dataIndex: 'agentVersion', render: (v: string, row) => {
                const color = row.versionStatus === 'outdated' ? 'orange' : row.versionStatus === 'unknown' ? 'gray' : 'arcoblue'
                const label = row.versionStatus === 'outdated' ? '过期' : row.versionStatus === 'unknown' ? '未知' : '当前'
                return <Space><Typography.Text>{v || '-'}</Typography.Text><Tag color={color} bordered size="small">{label}</Tag></Space>
              }},
              { title: '任务', dataIndex: 'taskCount', render: (v: number) => `${v} 个` },
              { title: '最近心跳', dataIndex: 'lastSeen', render: (v: string) => formatDateTime(v) },
            ]}
          />
        </Card>
      ) : null}

      {nodePerf.length > 0 && nodePerf.some((n) => n.totalRuns > 0) ? (
        <Card loading={loading} title="节点执行表现（近 30 天）">
          <Table
            rowKey={(r: NodePerformance) => `${r.nodeId}-${r.nodeName}`}
            stripe
            pagination={false}
            data={nodePerf.filter((n) => n.totalRuns > 0)}
            columns={[
              { title: '节点', render: (_: unknown, r: NodePerformance) => (
                <Space>
                  <Typography.Text bold>{r.nodeName}</Typography.Text>
                  {r.isLocal && <Tag bordered size="small">Master</Tag>}
                </Space>
              )},
              { title: '执行次数', dataIndex: 'totalRuns', render: (v: number) => `${v}` },
              { title: '成功 / 失败', render: (_: unknown, r: NodePerformance) => (
                <Space>
                  <Typography.Text style={{ color: 'var(--color-success-6)' }}>{r.successRuns}</Typography.Text>
                  <Typography.Text type="secondary">/</Typography.Text>
                  <Typography.Text style={{ color: r.failedRuns > 0 ? 'var(--color-danger-6)' : undefined }}>{r.failedRuns}</Typography.Text>
                </Space>
              )},
              { title: '成功率', dataIndex: 'successRate', render: (v: number) => {
                const rate = v * 100
                const color = rate >= 95 ? 'var(--color-success-6)' : rate >= 80 ? 'var(--color-warning-6)' : 'var(--color-danger-6)'
                return <Typography.Text style={{ color }}>{rate.toFixed(1)}%</Typography.Text>
              }},
              { title: '备份总量', dataIndex: 'totalBytes', render: (v: number) => formatBytes(v) },
              { title: '平均耗时', dataIndex: 'avgDurationSecs', render: (v: number) => {
                if (v <= 0) return '-'
                if (v < 60) return `${v.toFixed(0)} 秒`
                return `${(v / 60).toFixed(1)} 分`
              }},
            ]}
          />
        </Card>
      ) : null}

      {sla && sla.totalTasksWithSla > 0 ? (
        <Card loading={loading} title={
          <Space>
            <IconSafe />
            <span>SLA 合规</span>
            <Tag color={sla.violated === 0 ? 'green' : 'red'} bordered>
              {sla.violated === 0 ? '全部达标' : `${sla.violated} 个违约`}
            </Tag>
          </Space>
        }>
          <Row gutter={16}>
            <Col span={8}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>参与 SLA 任务数</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0' }}>{sla.totalTasksWithSla}</Typography.Title>
              </div>
            </Col>
            <Col span={8}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>达标</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0', color: 'var(--color-success-6)' }}>{sla.compliant}</Typography.Title>
              </div>
            </Col>
            <Col span={8}>
              <div>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>合规率</Typography.Text>
                <Typography.Title heading={4} style={{ margin: '4px 0 0' }}>{formatPercent(sla.coverageRate)}</Typography.Title>
              </div>
            </Col>
          </Row>
          {sla.violations.length > 0 && (
            <>
              <Alert type="warning" style={{ marginTop: 16 }} content={`有 ${sla.violations.length} 个任务的 RPO 超标，请尽快排查：`} />
              <Table
                style={{ marginTop: 12 }}
                noDataElement={<Empty description="无违约任务" />}
                rowKey="taskId"
                columns={[
                  { title: '任务', dataIndex: 'taskName', render: (value: string, record: SLAComplianceReport['violations'][number]) => (
                    <Space direction="vertical" size={2}>
                      <Typography.Text bold>{value}</Typography.Text>
                      {record.nodeName ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>节点: {record.nodeName}</Typography.Text> : null}
                    </Space>
                  ) },
                  { title: 'RPO 目标', dataIndex: 'slaHoursRpo', render: (value: number) => `${value} 小时` },
                  { title: '距上次成功', dataIndex: 'hoursSinceLastSuccess', render: (value: number, record: SLAComplianceReport['violations'][number]) =>
                    record.neverSucceeded ? <Tag color="red" bordered>从未成功</Tag> : `${value.toFixed(1)} 小时`,
                  },
                  { title: '最近成功', dataIndex: 'lastSuccessAt', render: (value?: string) => formatDateTime(value) },
                ]}
                data={sla.violations}
                pagination={false}
                stripe
              />
            </>
          )}
        </Card>
      ) : null}

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
