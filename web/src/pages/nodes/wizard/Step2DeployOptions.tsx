import React from 'react'
import { Form, Radio, Select, Input, Typography } from '@arco-design/web-react'
import type { InstallMode, InstallArch, InstallSource } from '../../../types/nodes'
import { AgentConnectionOptions, type AgentConnectionValue } from './AgentConnectionOptions'

const { Text } = Typography

export interface DeployOptions extends AgentConnectionValue {
  mode: InstallMode
  arch: InstallArch
  agentVersion: string
  downloadSrc: InstallSource
  ttlSeconds: number
}

interface Props {
  // null = 拉取中；空串 = 拉取失败（改为手动输入）
  masterVersion: string | null
  value: DeployOptions
  onChange: (v: DeployOptions) => void
}

export function Step2DeployOptions({ masterVersion, value, onChange }: Props) {
  const update = (patch: Partial<DeployOptions>) => onChange({ ...value, ...patch })
  const versionKnown = isReleaseVersion(masterVersion)
  const versionLoading = masterVersion === null

  return (
    <Form layout="vertical" size="default">
      <Form.Item
        label="安装模式"
        extra={value.mode === 'docker'
          ? <Text type="warning">Docker Agent 只能访问显式挂载的目录；备份源使用只读 volume，恢复目录需单独授权写入，或改用 systemd。</Text>
          : undefined}
      >
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

      <Form.Item
        label="Agent 版本"
        extra={
          !versionKnown && !versionLoading ? (
            <Text type="warning" style={{ fontSize: 12 }}>
              {masterVersion
                ? `当前 Master 版本 ${masterVersion} 不是可下载的 Release，请手动输入 Agent Release 标签`
                : '未能自动获取 Master 版本，请手动输入 Agent Release 标签（形如 v1.7.0）'}
            </Text>
          ) : undefined
        }
      >
        {versionKnown ? (
          <Select
            value={value.agentVersion}
            onChange={(v) => update({ agentVersion: v })}
            options={[
              { label: `${masterVersion}（跟随 Master，推荐）`, value: masterVersion as string },
            ]}
          />
        ) : (
          <Input
            placeholder={versionLoading ? '加载中...' : 'v1.7.0'}
            value={value.agentVersion}
            onChange={(v) => update({ agentVersion: v })}
            disabled={versionLoading}
          />
        )}
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

      <AgentConnectionOptions value={value} onChange={(connection) => update(connection)} />
    </Form>
  )
}

export function isReleaseVersion(version: string | null) {
  return !!version && /^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)
}
