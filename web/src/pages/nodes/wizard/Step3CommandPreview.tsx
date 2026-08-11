import React, { useEffect, useState } from 'react'
import { Typography, Button, Space, Collapse, Spin, Message, Tag } from '@arco-design/web-react'
import { IconRefresh } from '../../../components/icons'
import { fetchScriptPreview } from '../../../services/nodes'
import type { InstallTokenResult } from '../../../types/nodes'
import { buildAgentDownloadCommand, buildAgentInstallCommand, buildEmbeddedAgentInstallCommand } from '../installCommands'
import { InstallCommandBlock } from './InstallCommandBlock'

const { Text } = Typography

interface Props {
  nodeId: number
  nodeName: string
  token: InstallTokenResult
  previewParams: {
    mode: string
    arch: string
    agentVersion: string
    downloadSrc: string
    agentMasterUrl?: string
    proxyUrl?: string
    caCertFile?: string
  }
  onRegenerate: () => void
}

export function Step3CommandPreview({ nodeId, nodeName, token, previewParams, onRegenerate }: Props) {
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
  const fetchOptions = { proxyUrl: previewParams.proxyUrl, caCertFile: previewParams.caCertFile }
  const command = buildAgentInstallCommand(token.url, token.fallbackUrl, fetchOptions)
  const fallbackCommand = buildAgentDownloadCommand(token.url, token.fallbackUrl, fetchOptions)
  const embeddedCommand = token.scriptBase64 ? buildEmbeddedAgentInstallCommand(token.scriptBase64) : null

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
        <Text>节点：</Text>
        <Tag>{nodeName}</Tag>
        <Tag color={expired ? 'gray' : 'green'}>
          {expired ? '已过期' : `有效期 ${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')}`}
        </Tag>
      </Space>

      <InstallCommandBlock
        command={command}
        disabled={expired}
        onCopy={copy}
        action={expired ? <Button size="small" type="primary" icon={<IconRefresh />} onClick={onRegenerate}>重新生成</Button> : undefined}
      />

      <InstallCommandBlock label="或先下载到 /tmp 后执行：" command={fallbackCommand} disabled={expired} onCopy={copy} />

      {embeddedCommand && (
        <InstallCommandBlock label="安装入口不可达时使用嵌入式备用命令：" command={embeddedCommand} onCopy={copy} />
      )}

      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
        主安装命令包含一次性 install token，会在 TTL 到期或首次消费后作废；嵌入式备用命令包含完整节点 Token，不依赖公开入口，请仅在目标机执行并妥善保存。
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
