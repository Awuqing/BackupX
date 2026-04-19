import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Message, Typography } from '@arco-design/web-react'
import { IconCopy, IconDownload } from '@arco-design/web-react/icon'

const { Text } = Typography

export interface BatchCommandRow {
  nodeId: number
  nodeName: string
  command: string
  expiresAt: string
}

interface Props {
  rows: BatchCommandRow[]
}

export function BatchCommandTable({ rows }: Props) {
  const [remaining, setRemaining] = useState<Record<number, number>>({})

  useEffect(() => {
    const tick = () => {
      const next: Record<number, number> = {}
      rows.forEach((r) => {
        const exp = new Date(r.expiresAt).getTime()
        next[r.nodeId] = Math.max(0, Math.floor((exp - Date.now()) / 1000))
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
    const content = [
      '#!/bin/sh',
      '# BackupX Agent 批量部署脚本',
      '# 使用方法：在目标机逐个执行下面对应节点命令',
      '',
      ...rows.map((r) => `# --- ${r.nodeName} ---\n${r.command}`),
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
            title: '安装命令',
            dataIndex: 'command',
            render: (cmd: unknown, row: BatchCommandRow) => {
              const left = remaining[row.nodeId] ?? 0
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
              return (
                <Text type={left === 0 ? 'secondary' : 'primary'} style={{ fontSize: 12 }}>
                  {left === 0 ? '已过期' : `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`}
                </Text>
              )
            },
          },
          {
            title: '操作', width: 80,
            render: (_v: unknown, row: BatchCommandRow) => (
              <Button size="small" icon={<IconCopy />} onClick={() => copy(row.command)}
                disabled={(remaining[row.nodeId] ?? 0) === 0}>复制</Button>
            ),
          },
        ]}
        data={rows}
        rowKey="nodeId"
      />
      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Space>
          <Button icon={<IconDownload />} onClick={exportAll}>导出 .sh</Button>
        </Space>
      </div>
    </div>
  )
}
