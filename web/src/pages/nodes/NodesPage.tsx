import React, { useEffect, useState, useCallback } from 'react'
import {
  Table, Button, Space, Tag, Typography, PageHeader, Modal, Input, Message, Badge, Popconfirm, Card,
  Empty, Dropdown, Menu, Tooltip, InputNumber,
} from '@arco-design/web-react'
import {
  IconPlus, IconDelete, IconDesktop, IconCloudDownload, IconEdit, IconMore,
} from '@arco-design/web-react/icon'
import type { NodeSummary } from '../../types/nodes'
import { listNodes, deleteNode, updateNode, rotateNodeToken } from '../../services/nodes'
import { fetchSystemInfo } from '../../services/system'
import { AgentInstallWizard } from './AgentInstallWizard'

const { Text } = Typography

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [loading, setLoading] = useState(false)

  const [wizardVisible, setWizardVisible] = useState(false)
  const [wizardFixedNode, setWizardFixedNode] = useState<{ id: number; name: string } | undefined>()
  // null = 拉取中 / 未知；空字符串 = 拉取失败（UI 将要求用户手动输入版本，避免生成无效 URL）
  const [masterVersion, setMasterVersion] = useState<string | null>(null)

  const [editVisible, setEditVisible] = useState(false)
  const [editNode, setEditNode] = useState<NodeSummary | null>(null)
  const [editName, setEditName] = useState('')
  const [editLabels, setEditLabels] = useState('')
  const [editMaxConcurrent, setEditMaxConcurrent] = useState<number>(0)
  const [editBandwidthLimit, setEditBandwidthLimit] = useState('')

  const fetchNodes = useCallback(async () => {
    setLoading(true)
    try {
      const data = await listNodes()
      setNodes(data)
    } catch {
      Message.error('获取节点列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchNodes()
    // 取 Master 版本号作为 Wizard agentVersion 默认值。
    // 拉取失败或字段缺失时置为空串，Wizard 会提示用户手动输入。
    fetchSystemInfo().then((info) => {
      setMasterVersion(info?.version || '')
    }).catch(() => setMasterVersion(''))
  }, [fetchNodes])

  const handleDelete = async (id: number) => {
    try {
      await deleteNode(id)
      Message.success('节点已删除')
      fetchNodes()
    } catch {
      Message.error('删除节点失败')
    }
  }

  const handleEdit = async () => {
    if (!editNode || !editName.trim()) {
      Message.warning('请输入节点名称')
      return
    }
    try {
      await updateNode(editNode.id, {
        name: editName.trim(),
        labels: editLabels.trim(),
        maxConcurrent: editMaxConcurrent,
        bandwidthLimit: editBandwidthLimit.trim(),
      })
      Message.success('节点更新成功')
      setEditVisible(false)
      fetchNodes()
    } catch {
      Message.error('更新节点失败')
    }
  }

  const handleRotate = async (record: NodeSummary) => {
    try {
      const { newToken } = await rotateNodeToken(record.id)
      Modal.success({
        title: 'Token 已轮换',
        content: (
          <div>
            <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
              新 Token（24 小时内新旧 Token 均可认证，便于滚动替换）：
            </Text>
            <Text copyable style={{ fontFamily: 'monospace', fontSize: 12, wordBreak: 'break-all' }}>
              {newToken}
            </Text>
          </div>
        ),
      })
    } catch {
      Message.error('轮换 Token 失败')
    }
  }

  const columns = [
    {
      title: '节点名称', dataIndex: 'name',
      render: (name: string, record: NodeSummary) => (
        <Space>
          {record.isLocal ? <IconDesktop style={{ color: 'var(--color-primary-6)' }} /> : <IconCloudDownload />}
          <Text bold>{name}</Text>
          {record.isLocal && <Tag color="arcoblue" size="small" bordered>本机</Tag>}
        </Space>
      ),
    },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (status: string) => status === 'online'
        ? <Badge status="success" text="在线" />
        : <Badge status="default" text="离线" />,
    },
    { title: '主机名', dataIndex: 'hostname', render: (v: string) => v || '-' },
    { title: 'IP 地址', dataIndex: 'ipAddress', render: (v: string) => v || '-' },
    {
      title: '系统', dataIndex: 'os', width: 120,
      render: (_: string, record: NodeSummary) => record.os
        ? <Tag bordered>{record.os}/{record.arch}</Tag> : '-',
    },
    {
      title: 'Agent 版本', dataIndex: 'agentVersion', width: 140,
      render: (v: string) => renderAgentVersion(v, masterVersion),
    },
    {
      title: '标签 / 节点池', dataIndex: 'labels', width: 180,
      render: (v: string) => {
        const tags = (v || '').split(',').map(s => s.trim()).filter(Boolean)
        if (tags.length === 0) return <Text type="secondary">-</Text>
        return <Space wrap size={4}>{tags.map(tag => <Tag key={tag} color="arcoblue">{tag}</Tag>)}</Space>
      },
    },
    {
      title: '最后活跃', dataIndex: 'lastSeen', width: 170,
      render: (v: string) => v ? new Date(v).toLocaleString('zh-CN') : '-',
    },
    {
      title: '操作', width: 180,
      render: (_: unknown, record: NodeSummary) => (
        <Space>
          <Button type="text" icon={<IconEdit />} size="small"
            onClick={() => {
              setEditNode(record); setEditName(record.name)
              setEditLabels(record.labels || '')
              setEditMaxConcurrent(record.maxConcurrent || 0)
              setEditBandwidthLimit(record.bandwidthLimit || '')
              setEditVisible(true)
            }} />
          {!record.isLocal && (
            <>
              <Dropdown trigger="click" droplist={(
                <Menu>
                  <Menu.Item key="install"
                    onClick={() => { setWizardFixedNode({ id: record.id, name: record.name }); setWizardVisible(true) }}>
                    生成安装命令
                  </Menu.Item>
                  <Menu.Item key="rotate" onClick={() => handleRotate(record)}>
                    重新生成 Token
                  </Menu.Item>
                </Menu>
              )}>
                <Button type="text" icon={<IconMore />} size="small" />
              </Dropdown>
              <Popconfirm title="确定删除该节点？" onOk={() => handleDelete(record.id)}>
                <Button type="text" status="danger" icon={<IconDelete />} size="small" />
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '0 4px' }}>
      <PageHeader
        title="节点管理"
        subTitle="管理集群中的服务器节点"
        extra={
          <Button type="primary" icon={<IconPlus />}
            onClick={() => { setWizardFixedNode(undefined); setWizardVisible(true) }}>
            添加节点
          </Button>
        }
      />

      <Card style={{ marginTop: 16 }}>
        <Table columns={columns} data={nodes} rowKey="id" loading={loading} pagination={false}
          noDataElement={<Empty description="暂无节点数据，系统将自动创建本机节点" />} />
      </Card>

      <AgentInstallWizard
        visible={wizardVisible}
        onClose={() => setWizardVisible(false)}
        onSuccess={fetchNodes}
        masterVersion={masterVersion}
        fixedNode={wizardFixedNode}
      />

      <Modal title="编辑节点" visible={editVisible}
        onCancel={() => setEditVisible(false)} onOk={handleEdit}
        okText="保存" cancelText="取消" style={{ width: 520 }}>
        <div style={{ marginBottom: 8 }}><Text type="secondary">节点名称</Text></div>
        <Input placeholder="输入节点名称" value={editName} onChange={setEditName} />

        <div style={{ margin: '16px 0 8px 0' }}>
          <Text type="secondary">标签 / 节点池</Text>
          <Tooltip content="以英文逗号分隔，如 prod,db,high-mem。任务配置节点池标签时会从命中的在线节点中按负载最低选一台执行。">
            <Text type="secondary" style={{ marginLeft: 8, cursor: 'help' }}>ⓘ</Text>
          </Tooltip>
        </div>
        <Input placeholder="例如：prod,db,high-mem" value={editLabels} onChange={setEditLabels} />

        <div style={{ margin: '16px 0 8px 0' }}><Text type="secondary">最大并发任务数（0 = 不限）</Text></div>
        <InputNumber min={0} max={64} value={editMaxConcurrent} onChange={v => setEditMaxConcurrent(v ?? 0)} style={{ width: '100%' }} />

        <div style={{ margin: '16px 0 8px 0' }}>
          <Text type="secondary">带宽限速</Text>
          <Tooltip content="rclone 格式，如 10M 表示 10MB/s，留空走全局默认">
            <Text type="secondary" style={{ marginLeft: 8, cursor: 'help' }}>ⓘ</Text>
          </Tooltip>
        </div>
        <Input placeholder="例如：10M 或 1G；留空使用全局默认" value={editBandwidthLimit} onChange={setEditBandwidthLimit} />
      </Modal>
    </div>
  )
}

/**
 * 渲染 Agent 版本 + 与 Master 的漂移状态。
 *   空版本 → "-"（未上报）
 *   与 Master 相同 → 原样显示
 *   不同（且非本机） → 红色 Tag + 提示升级
 */
function renderAgentVersion(agentVer: string, masterVer: string | null): React.ReactNode {
  if (!agentVer) return <Text type="secondary">-</Text>
  if (!masterVer) return agentVer
  if (agentVer === masterVer) return agentVer
  return (
    <Tooltip content={`Master 版本 ${masterVer}，建议重新生成安装命令升级 Agent`}>
      <Tag color="orangered" style={{ cursor: 'help' }}>{agentVer} ≠ {masterVer}</Tag>
    </Tooltip>
  )
}
