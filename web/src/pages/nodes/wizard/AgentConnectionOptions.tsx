import React from 'react'
import { Form, Input, Radio, Typography } from '@arco-design/web-react'

const { Text } = Typography

export type ConnectionMode = 'direct' | 'restricted'

export interface AgentConnectionValue {
  connectionMode: ConnectionMode
  agentMasterUrl: string
  proxyUrl: string
  caCertFile: string
}

interface Props {
  value: AgentConnectionValue
  onChange: (value: AgentConnectionValue) => void
}

export function AgentConnectionOptions({ value, onChange }: Props) {
  const update = (patch: Partial<AgentConnectionValue>) => onChange({ ...value, ...patch })

  return (
    <>
      <Form.Item
        label="Agent 网络路径"
        extra={<Text type="secondary">Agent 只需主动访问 Master，不需要从 Master 反向开放节点端口。</Text>}
      >
        <Radio.Group
          type="button"
          value={value.connectionMode}
          onChange={(mode) => update({ connectionMode: mode as ConnectionMode })}
          options={[
            { label: '直连', value: 'direct' },
            { label: '代理或堡垒机', value: 'restricted' },
          ]}
        />
      </Form.Item>

      {value.connectionMode === 'restricted' && (
        <>
          <Form.Item
            label="Agent 连接地址"
            extra={<Text type="secondary">可填写经 SSH 本地转发后的地址；留空则继续使用 Master 对外地址。</Text>}
          >
            <Input
              value={value.agentMasterUrl}
              placeholder="例如 http://127.0.0.1:18340"
              onChange={(agentMasterUrl) => update({ agentMasterUrl })}
            />
          </Form.Item>
          <Form.Item
            label="显式代理 URL"
            extra={<Text type="secondary">支持 http、https、socks5、socks5h；SSH 动态转发可使用 socks5h://127.0.0.1:1080。</Text>}
          >
            <Input
              value={value.proxyUrl}
              placeholder="可选，例如 socks5h://127.0.0.1:1080"
              onChange={(proxyUrl) => update({ proxyUrl })}
            />
          </Form.Item>
          <Form.Item
            label="私有 CA 证书路径"
            extra={<Text type="secondary">目标节点上已存在的 PEM 文件绝对路径；安装器会复制到受保护的 Agent 配置目录。</Text>}
          >
            <Input
              value={value.caCertFile}
              placeholder="可选，例如 /etc/pki/ca-trust/source/anchors/internal-ca.pem"
              onChange={(caCertFile) => update({ caCertFile })}
            />
          </Form.Item>
        </>
      )}
    </>
  )
}

export function validateAgentConnection(value: AgentConnectionValue) {
  if (value.connectionMode === 'direct') return ''
  const agentMasterUrl = value.agentMasterUrl.trim()
  const proxyUrl = value.proxyUrl.trim()
  const caCertFile = value.caCertFile.trim()
  if (!agentMasterUrl && !proxyUrl && !caCertFile) {
    return '请至少填写 Agent 连接地址、代理 URL 或私有 CA 路径'
  }
  if (agentMasterUrl) {
    try {
      const parsed = new URL(agentMasterUrl)
      if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.host || parsed.username || parsed.password || parsed.search || parsed.hash || /\s/.test(agentMasterUrl)) {
        return 'Agent 连接地址必须是不含凭据、查询参数和片段的完整 HTTP(S) URL'
      }
    } catch {
      return 'Agent 连接地址必须是完整的 HTTP 或 HTTPS URL'
    }
  }
  if (proxyUrl) {
    try {
      const parsed = new URL(proxyUrl)
      if (!['http:', 'https:', 'socks5:', 'socks5h:'].includes(parsed.protocol) || !parsed.host || parsed.username || parsed.password || (parsed.pathname !== '' && parsed.pathname !== '/') || parsed.search || parsed.hash || /\s/.test(proxyUrl)) {
        return '代理 URL 仅支持无凭据、无路径的 http、https、socks5 或 socks5h 地址'
      }
    } catch {
      return '代理 URL 仅支持 http、https、socks5 或 socks5h'
    }
  }
  if (caCertFile && (!/^\/(?:[A-Za-z0-9._-]+\/)*[A-Za-z0-9._-]+$/.test(caCertFile) || caCertFile.split('/').some((part) => part === '..'))) {
    return '私有 CA 证书必须使用不含空格或特殊字符的绝对路径'
  }
  return ''
}
