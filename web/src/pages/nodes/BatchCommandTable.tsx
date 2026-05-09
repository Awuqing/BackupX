import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Message, Typography, Tag } from '@arco-design/web-react'
import { IconCopy, IconDownload, IconRefresh } from '@arco-design/web-react/icon'

const { Text } = Typography

export interface BatchCommandRow {
  nodeId: number
  nodeName: string
  status: 'ready' | 'failed'
  command: string
  expiresAt: string
  errorMessage?: string
  embeddedCommand?: string
}

interface Props {
  rows: BatchCommandRow[]
  onRetryNode?: (row: BatchCommandRow) => void
}

export function BatchCommandTable({ rows, onRetryNode }: Props) {
  const [remaining, setRemaining] = useState<Record<number, number>>({})

  useEffect(() => {
    const tick = () => {
      const next: Record<number, number> = {}
      rows.forEach((r) => {
        next[r.nodeId] = secondsLeft(r.expiresAt)
      })
      setRemaining(next)
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [rows])

  const copy = async (s: string) => {
    await navigator.clipboard.writeText(s)
    Message.success('已复制')
  }

  const exportAll = () => {
    const exportRows = getExportableBatchRows(rows)
    const content = [
      '#!/bin/sh',
      '# BackupX Agent 批量部署脚本',
      '# 使用方法：在目标机逐个执行下面对应节点命令',
      '',
      ...exportRows.map((r) => `# --- ${r.nodeName} ---\n${r.command}`),
    ].join('\n\n')
    const blob = new Blob([content], { type: 'text/x-shellscript' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `backupx-batch-install-${new Date().toISOString().slice(0, 10)}.sh`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div>
      <Table
        size="small"
        pagination={false}
        columns={[
          { title: '节点', dataIndex: 'nodeName', width: 140 },
          {
            title: '状态', dataIndex: 'status', width: 90,
            render: (status: BatchCommandRow['status']) => (
              status === 'ready' ? <Tag color="green">可执行</Tag> : <Tag color="red">失败</Tag>
            ),
          },
          {
            title: '安装命令',
            dataIndex: 'command',
            render: (cmd: unknown, row: BatchCommandRow) => {
              const left = remaining[row.nodeId] ?? 0
              if (row.status === 'failed') {
                return <Text type="error" style={{ fontSize: 12 }}>{row.errorMessage || '生成安装命令失败'}</Text>
              }
              return (
                <Text style={{
                  fontFamily: 'monospace', fontSize: 12, wordBreak: 'break-all',
                  opacity: left === 0 ? 0.4 : 1,
                }}>
                  {cmd as string}
                </Text>
              )
            },
          },
          {
            title: '剩余', dataIndex: 'expiresAt', width: 90,
            render: (_v: unknown, row: BatchCommandRow) => {
              const left = remaining[row.nodeId] ?? 0
              if (row.status === 'failed') {
                return <Text type="secondary" style={{ fontSize: 12 }}>-</Text>
              }
              return (
                <Text type={left === 0 ? 'secondary' : 'primary'} style={{ fontSize: 12 }}>
                  {left === 0 ? '已过期' : `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`}
                </Text>
              )
            },
          },
          {
            title: '操作', width: 110,
            render: (_v: unknown, row: BatchCommandRow) => (
              <Space>
                {row.status === 'ready' && (
                  <Button size="small" icon={<IconCopy />} onClick={() => copy(row.command)}
                    disabled={(remaining[row.nodeId] ?? 0) === 0}>复制</Button>
                )}
                {row.status === 'failed' && onRetryNode && (
                  <Button size="small" icon={<IconRefresh />} onClick={() => onRetryNode(row)}>重试</Button>
                )}
              </Space>
            ),
          },
        ]}
        data={rows}
        rowKey="nodeId"
      />
      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Space>
          <Button icon={<IconDownload />} onClick={exportAll}
            disabled={getExportableBatchRows(rows).length === 0}>导出 .sh</Button>
        </Space>
      </div>
    </div>
  )
}

function secondsLeft(expiresAt: string) {
  if (!expiresAt) {
    return 0
  }
  const exp = new Date(expiresAt).getTime()
  return Math.max(0, Math.floor((exp - Date.now()) / 1000))
}

export function getExportableBatchRows(rows: BatchCommandRow[]) {
  return rows.filter((row) => row.status === 'ready' && secondsLeft(row.expiresAt) > 0)
}
