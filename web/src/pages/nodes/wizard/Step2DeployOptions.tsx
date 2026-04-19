import React from 'react'
import { Form, Radio, Select, Typography } from '@arco-design/web-react'
import type { InstallMode, InstallArch, InstallSource } from '../../../types/nodes'

const { Text } = Typography

export interface DeployOptions {
  mode: InstallMode
  arch: InstallArch
  agentVersion: string
  downloadSrc: InstallSource
  ttlSeconds: number
}

interface Props {
  masterVersion: string
  value: DeployOptions
  onChange: (v: DeployOptions) => void
}

export function Step2DeployOptions({ masterVersion, value, onChange }: Props) {
  const update = (patch: Partial<DeployOptions>) => onChange({ ...value, ...patch })

  return (
    <Form layout="vertical" size="default">
      <Form.Item label="安装模式">
        <Radio.Group
          type="button"
          value={value.mode}
          onChange={(v) => update({ mode: v as InstallMode })}
          options={[
            { label: 'systemd（推荐）', value: 'systemd' },
            { label: 'Docker', value: 'docker' },
            { label: '前台运行（调试）', value: 'foreground' },
          ]}
        />
      </Form.Item>

      <Form.Item label="架构">
        <Select
          value={value.arch}
          onChange={(v) => update({ arch: v as InstallArch })}
          options={[
            { label: '自动检测（uname -m）', value: 'auto' },
            { label: 'amd64 (x86_64)', value: 'amd64' },
            { label: 'arm64 (aarch64)', value: 'arm64' },
          ]}
        />
      </Form.Item>

      <Form.Item label="Agent 版本">
        <Select
          value={value.agentVersion}
          onChange={(v) => update({ agentVersion: v })}
          options={[
            { label: `${masterVersion}（跟随 Master，推荐）`, value: masterVersion },
          ]}
        />
      </Form.Item>

      <Form.Item label="安装命令有效期">
        <Select
          value={value.ttlSeconds}
          onChange={(v) => update({ ttlSeconds: v as number })}
          options={[
            { label: '5 分钟', value: 300 },
            { label: '15 分钟（推荐）', value: 900 },
            { label: '1 小时', value: 3600 },
            { label: '24 小时', value: 86400 },
          ]}
        />
      </Form.Item>

      <Form.Item
        label="二进制下载源"
        extra={<Text type="secondary">国内服务器选 ghproxy 镜像加速</Text>}
      >
        <Radio.Group
          type="button"
          value={value.downloadSrc}
          onChange={(v) => update({ downloadSrc: v as InstallSource })}
          options={[
            { label: 'GitHub 直连', value: 'github' },
            { label: 'ghproxy 镜像', value: 'ghproxy' },
          ]}
        />
      </Form.Item>
    </Form>
  )
}
