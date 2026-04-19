import React, { useEffect, useState } from 'react'
import { Typography, Button, Space, Collapse, Spin, Message, Tag } from '@arco-design/web-react'
import { IconCopy, IconRefresh } from '@arco-design/web-react/icon'
import { fetchScriptPreview } from '../../../services/nodes'
import type { InstallTokenResult, InstallMode } from '../../../types/nodes'

const { Text } = Typography

interface Props {
  nodeId: number
  nodeName: string
  token: InstallTokenResult
  mode: InstallMode
  previewParams: { mode: string; arch: string; agentVersion: string; downloadSrc: string }
  onRegenerate: () => void
}

export function Step3CommandPreview({ nodeId, nodeName, token, mode, previewParams, onRegenerate }: Props) {
  const [remaining, setRemaining] = useState(0)
  const [preview, setPreview] = useState<string>('')
  const [loadingPreview, setLoadingPreview] = useState(false)

  useEffect(() => {
    const expires = new Date(token.expiresAt).getTime()
    const tick = () => setRemaining(Math.max(0, Math.floor((expires - Date.now()) / 1000)))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [token.expiresAt])

  const expired = remaining === 0
  const command = `curl -fsSL ${token.url} | sudo sh`
  const dockerComposeCmd = mode === 'docker' && token.composeUrl
    ? `curl -fsSL ${token.composeUrl} -o docker-compose.yml && docker-compose up -d`
    : null

  const copy = async (s: string) => {
    await navigator.clipboard.writeText(s)
    Message.success('已复制')
  }

  const loadPreview = async () => {
    setLoadingPreview(true)
    try {
      const text = await fetchScriptPreview(nodeId, previewParams)
      setPreview(text)
    } catch {
      Message.error('预览加载失败')
    } finally {
      setLoadingPreview(false)
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 12 }}>
        <Text bold>节点：</Text>
        <Tag>{nodeName}</Tag>
        <Tag color={expired ? 'gray' : 'green'}>
          {expired ? '已过期' : `有效期 ${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')}`}
        </Tag>
      </Space>

      <div style={{ background: 'var(--color-fill-2)', padding: '12px 14px', borderRadius: 6, marginBottom: 12 }}>
        <Text style={{
          fontFamily: 'monospace', fontSize: 13, wordBreak: 'break-all',
          opacity: expired ? 0.4 : 1, userSelect: 'all',
        }}>
          {command}
        </Text>
        <div style={{ marginTop: 8 }}>
          <Space>
            <Button size="small" icon={<IconCopy />} disabled={expired} onClick={() => copy(command)}>复制</Button>
            {expired && <Button size="small" type="primary" icon={<IconRefresh />} onClick={onRegenerate}>重新生成</Button>}
          </Space>
        </div>
      </div>

      {dockerComposeCmd && (
        <div style={{ background: 'var(--color-fill-2)', padding: '12px 14px', borderRadius: 6, marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
            或使用 docker-compose：
          </Text>
          <Text style={{ fontFamily: 'monospace', fontSize: 13, wordBreak: 'break-all', opacity: expired ? 0.4 : 1 }}>
            {dockerComposeCmd}
          </Text>
          <div style={{ marginTop: 8 }}>
            <Button size="small" icon={<IconCopy />} disabled={expired} onClick={() => copy(dockerComposeCmd)}>复制</Button>
          </div>
        </div>
      )}

      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
        命令仅显示一次，复制后请尽快在目标机执行。token 一经消费立即作废。
      </Text>

      <Collapse bordered={false} onChange={(_key, keys) => {
        if (keys.includes('preview') && !preview) loadPreview()
      }}>
        <Collapse.Item name="preview" header="展开脚本预览">
          {loadingPreview ? <Spin /> : (
            <pre style={{
              background: 'var(--color-fill-2)', padding: 12, borderRadius: 4,
              fontSize: 12, maxHeight: 400, overflow: 'auto', whiteSpace: 'pre-wrap',
            }}>{preview}</pre>
          )}
        </Collapse.Item>
      </Collapse>
    </div>
  )
}
