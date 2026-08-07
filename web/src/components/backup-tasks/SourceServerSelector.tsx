import { Input, Select, Typography } from '@arco-design/web-react'
import { useMemo } from 'react'
import type { NodeSummary } from '../../types/nodes'

interface SourceServerSelectorProps {
  nodeId: number
  nodePoolTag: string
  localNodeId?: number
  nodes?: NodeSummary[]
  onNodeChange: (nodeId: number) => void
  onNodePoolTagChange: (tag: string) => void
}

export function buildSourceServerOptions(nodes: NodeSummary[] = []) {
  return [
    { label: 'Master 本机', value: 0, disabled: false },
    ...nodes
      .filter((node) => !node.isLocal)
      .map((node) => ({
        label: `${node.name}${node.status === 'online' ? '' : '（离线）'}`,
        value: node.id,
        disabled: node.status !== 'online',
      })),
  ]
}

export function SourceServerSelector({ nodeId, nodePoolTag, localNodeId, nodes, onNodeChange, onNodePoolTagChange }: SourceServerSelectorProps) {
  const options = useMemo(() => buildSourceServerOptions(nodes), [nodes])
  const selectedNode = nodes?.find((node) => node.id === nodeId)
  const isRemote = nodeId > 0 && nodeId !== localNodeId

  return (
    <>
      <div>
        <Typography.Text>源服务器</Typography.Text>
        <Select value={nodeId} options={options} onChange={(value) => onNodeChange(Number(value ?? 0))} />
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
          {isRemote
            ? `源路径与数据库在 ${selectedNode?.name ?? '远程服务器'} 上解析，由 Agent 就地生成备份。网络存储由 Agent 直传；启用 Master 中转的本地磁盘目标会通过认证连接写入中央目录。`
            : '源路径与数据库在 Master 本机解析。要集中备份其他服务器，请先在“节点管理”安装 Agent，再在这里选择对应源服务器。'}
        </Typography.Paragraph>
      </div>
      <div>
        <Typography.Text>源服务器池标签（可选）</Typography.Text>
        <Input
          placeholder="按标签从在线源服务器中动态选择（与固定源服务器互斥）"
          value={nodePoolTag}
          disabled={nodeId > 0}
          onChange={onNodePoolTagChange}
        />
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
          仅在选择 Master 本机时可填写；系统从 Labels 命中该标签的在线 Agent 中选择当前运行任务最少的一台。
        </Typography.Paragraph>
      </div>
    </>
  )
}
