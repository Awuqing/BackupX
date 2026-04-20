import { Alert, Card, Empty, Typography } from '@arco-design/web-react'
import ReactEChartsCore from 'echarts-for-react/lib/core'
import { GraphChart } from 'echarts/charts'
import * as echarts from 'echarts/core'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useMemo } from 'react'
import type { BackupTaskSummary } from '../../types/backup-tasks'

echarts.use([GraphChart, TooltipComponent, CanvasRenderer])

interface Props {
  tasks: BackupTaskSummary[]
}

const STATUS_COLORS: Record<string, string> = {
  success: '#00B42A',
  failed: '#F53F3F',
  running: '#165DFF',
  idle: '#86909C',
}

/**
 * TaskDependencyGraph 任务依赖有向图可视化。
 * - 节点 = 任务，按最近状态着色
 * - 边 = 依赖关系（上游 → 下游）
 * - 只显示有依赖关系或被依赖的任务（孤岛任务忽略，减少视觉噪音）
 */
export function TaskDependencyGraph({ tasks }: Props) {
  const { nodes, links, hasAny } = useMemo(() => {
    const nodeIds = new Set<number>()
    const allLinks: { source: string; target: string }[] = []
    for (const task of tasks) {
      const deps = task.dependsOnTaskIds ?? []
      if (deps.length === 0) continue
      nodeIds.add(task.id)
      for (const dep of deps) {
        nodeIds.add(dep)
        allLinks.push({ source: String(dep), target: String(task.id) })
      }
    }
    const taskMap = new Map(tasks.map((t) => [t.id, t]))
    const graphNodes = Array.from(nodeIds).map((id) => {
      const t = taskMap.get(id)
      const status = t?.lastStatus ?? 'idle'
      return {
        id: String(id),
        name: t?.name ?? `#${id}`,
        symbolSize: 40,
        itemStyle: { color: STATUS_COLORS[status] ?? '#86909C' },
        label: { show: true, fontSize: 11, color: 'var(--color-text-1)' },
      }
    })
    return { nodes: graphNodes, links: allLinks, hasAny: allLinks.length > 0 }
  }, [tasks])

  const option = useMemo(
    () => ({
      tooltip: { trigger: 'item' as const, formatter: '{b}' },
      animationDuration: 800,
      series: [
        {
          type: 'graph' as const,
          layout: 'force' as const,
          roam: true,
          draggable: true,
          force: { repulsion: 180, gravity: 0.08, edgeLength: 120 },
          label: { show: true, position: 'bottom' as const },
          edgeSymbol: ['none', 'arrow'] as [string, string],
          edgeSymbolSize: [0, 10] as [number, number],
          lineStyle: { color: 'var(--color-border-3)', curveness: 0.1 },
          data: nodes,
          links,
        },
      ],
    }),
    [nodes, links],
  )

  if (!hasAny) {
    return (
      <Card>
        <Empty description={
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            暂无任务依赖关系。可在任务表单的"任务依赖"中配置上游任务，形成自动化工作流。
          </Typography.Paragraph>
        } />
      </Card>
    )
  }

  return (
    <Card title="任务依赖图">
      <Alert type="info" content="节点颜色按最近执行状态：绿=成功 / 红=失败 / 蓝=执行中 / 灰=未运行。箭头方向 = 上游 → 下游。" style={{ marginBottom: 12 }} />
      <ReactEChartsCore echarts={echarts} option={option} style={{ height: 420 }} />
    </Card>
  )
}
