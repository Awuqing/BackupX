import React, { type ReactNode } from 'react'
import { Button, Space, Typography } from '@arco-design/web-react'
import { IconCopy } from '../../../components/icons'

const { Text } = Typography

interface Props {
  label?: string
  command: string
  disabled?: boolean
  action?: ReactNode
  onCopy: (command: string) => void
}

export function InstallCommandBlock({ label, command, disabled, action, onCopy }: Props) {
  return (
    <div style={{ background: 'var(--color-fill-2)', padding: '12px 14px', borderRadius: 4, marginBottom: 12 }}>
      {label && <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>{label}</Text>}
      <Text style={{ fontSize: 13, wordBreak: 'break-all', opacity: disabled ? 0.4 : 1, userSelect: 'all' }}>
        {command}
      </Text>
      <div style={{ marginTop: 8 }}>
        <Space>
          <Button size="small" icon={<IconCopy />} disabled={disabled} onClick={() => onCopy(command)}>复制</Button>
          {action}
        </Space>
      </div>
    </div>
  )
}
