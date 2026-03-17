import React, { useEffect, useState, useCallback } from 'react'
import {
  Table, Button, Space, Tag, Typography, PageHeader, Modal, Input, Message, Badge, Popconfirm, Card, Descriptions, Empty
} from '@arco-design/web-react'
import {
  IconPlus, IconDelete, IconDesktop, IconCloudDownload
} from '@arco-design/web-react/icon'
import type { NodeSummary } from '../../types/nodes'
import { listNodes, createNode, deleteNode } from '../../services/nodes'

const { Title, Text } = Typography

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [loading, setLoading] = useState(false)
  const [createVisible, setCreateVisible] = useState(false)
  const [newNodeName, setNewNodeName] = useState('')
  const [newToken, setNewToken] = useState('')

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

  useEffect(() => { fetchNodes() }, [fetchNodes])

  const handleCreate = async () => {
    if (!newNodeName.trim()) {
      Message.warning('请输入节点名称')
      return
    }
    try {
      const result = await createNode(newNodeName.trim())
      setNewToken(result.token)
      Message.success('节点创建成功')
      fetchNodes()
    } catch {
      Message.error('创建节点失败')
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await deleteNode(id)
      Message.success('节点已删除')
      fetchNodes()
    } catch {
      Message.error('删除节点失败')
    }
  }

  const columns = [
    {
      title: '节点名称',
      dataIndex: 'name',
      render: (name: string, record: NodeSummary) => (
        <Space>
          {record.isLocal ? <IconDesktop style={{ color: 'var(--color-primary-6)' }} /> : <IconCloudDownload />}
          <Text bold>{name}</Text>
          {record.isLocal && <Tag color="arcoblue" size="small" bordered>本机</Tag>}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (status: string) => {
        if (status === 'online') return <Badge status="success" text="在线" />
        return <Badge status="default" text="离线" />
      },
    },
    {
      title: '主机名',
      dataIndex: 'hostname',
      render: (v: string) => v || '-',
    },
    {
      title: 'IP 地址',
      dataIndex: 'ipAddress',
      render: (v: string) => v || '-',
    },
    {
      title: '系统',
      dataIndex: 'os',
      width: 120,
      render: (_: string, record: NodeSummary) => {
        if (!record.os) return '-'
        return <Tag bordered>{record.os}/{record.arch}</Tag>
      },
    },
    {
      title: 'Agent 版本',
      dataIndex: 'agentVersion',
      width: 100,
      render: (v: string) => v || '-',
    },
    {
      title: '最后活跃',
      dataIndex: 'lastSeen',
      width: 170,
      render: (v: string) => v ? new Date(v).toLocaleString('zh-CN') : '-',
    },
    {
      title: '操作',
      width: 80,
      render: (_: unknown, record: NodeSummary) => {
        if (record.isLocal) return <Text type="secondary">-</Text>
        return (
          <Popconfirm title="确定删除该节点？" onOk={() => handleDelete(record.id)}>
            <Button type="text" status="danger" icon={<IconDelete />} size="small" />
          </Popconfirm>
        )
      },
    },
  ]

  return (
    <div style={{ padding: '0 4px' }}>
      <PageHeader
        title="节点管理"
        subTitle="管理集群中的服务器节点"
        extra={
          <Button type="primary" icon={<IconPlus />} onClick={() => { setCreateVisible(true); setNewToken(''); setNewNodeName('') }}>
            添加节点
          </Button>
        }
      />

      <Card style={{ marginTop: 16 }}>
        <Table
          columns={columns}
          data={nodes}
          rowKey="id"
          loading={loading}
          pagination={false}
          noDataElement={<Empty description="暂无节点数据，系统将自动创建本机节点" />}
        />
      </Card>

      <Modal
        title="添加远程节点"
        visible={createVisible}
        onCancel={() => setCreateVisible(false)}
        footer={newToken ? (
          <Button type="primary" onClick={() => setCreateVisible(false)}>完成</Button>
        ) : undefined}
        onOk={handleCreate}
        okText="创建"
      >
        {!newToken ? (
          <Input
            placeholder="输入节点名称，如：生产服务器-A"
            value={newNodeName}
            onChange={setNewNodeName}
          />
        ) : (
          <div>
            <Descriptions column={1} border data={[
              { label: '节点名称', value: newNodeName },
              { label: '认证令牌', value: <Text copyable style={{ wordBreak: 'break-all', fontSize: 12, fontFamily: 'monospace' }}>{newToken}</Text> },
            ]} />
            <div style={{ marginTop: 12, padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                请将此令牌配置到远程服务器的 Agent 启动参数中。令牌仅显示一次，请妥善保存。
              </Text>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
