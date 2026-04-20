import { Button, Checkbox, Input, Message, Space, Spin, Typography } from '@arco-design/web-react'
import { useState } from 'react'
import { discoverDatabases } from '../../services/database'

interface DatabasePickerProps {
  dbType: 'mysql' | 'postgresql'
  dbHost: string
  dbPort: number
  dbUser: string
  dbPassword: string
  /** 目标执行节点 ID。0 或 undefined 表示 Master 本地发现；远程节点通过 Agent 发现。 */
  nodeId?: number
  value: string
  onChange: (value: string) => void
}

export function DatabasePicker({ dbType, dbHost, dbPort, dbUser, dbPassword, nodeId, value, onChange }: DatabasePickerProps) {
  const [databases, setDatabases] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [discovered, setDiscovered] = useState(false)
  const [error, setError] = useState('')

  const selectedDbs = value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

  const canDiscover = dbHost.trim() && dbPort > 0 && dbUser.trim() && dbPassword.trim()

  async function handleDiscover() {
    setLoading(true)
    setError('')
    try {
      const result = await discoverDatabases({
        type: dbType,
        host: dbHost.trim(),
        port: dbPort,
        user: dbUser.trim(),
        password: dbPassword.trim(),
        nodeId: nodeId && nodeId > 0 ? nodeId : undefined,
      })
      setDatabases(result)
      setDiscovered(true)
      if (result.length === 0) {
        setError('未发现用户数据库')
      }
    } catch (discoverError: any) {
      const msg = discoverError?.response?.data?.message ?? discoverError?.message ?? '发现数据库失败'
      setError(msg)
      Message.error(msg)
    } finally {
      setLoading(false)
    }
  }

  function handleToggle(db: string, checked: boolean) {
    let next: string[]
    if (checked) {
      next = [...selectedDbs, db]
    } else {
      next = selectedDbs.filter((d) => d !== db)
    }
    onChange(next.join(','))
  }

  function handleSelectAll() {
    onChange(databases.join(','))
  }

  function handleDeselectAll() {
    onChange('')
  }

  return (
    <Space direction="vertical" size="medium" style={{ width: '100%' }}>
      <Space style={{ width: '100%' }}>
        <Input
          style={{ flex: 1 }}
          value={value}
          placeholder="数据库名称（多个以逗号分隔）"
          onChange={onChange}
        />
        <Button
          type="outline"
          size="small"
          loading={loading}
          disabled={!canDiscover}
          onClick={handleDiscover}
        >
          发现数据库
        </Button>
      </Space>

      {error && <Typography.Text type="error">{error}</Typography.Text>}

      {loading && <Spin size={16} />}

      {discovered && databases.length > 0 && (
        <div style={{ border: '1px solid var(--color-border-2)', borderRadius: 4, padding: '8px 12px', maxHeight: 200, overflow: 'auto' }}>
          <Space size="mini" style={{ marginBottom: 8 }}>
            <Button type="text" size="mini" onClick={handleSelectAll}>
              全选
            </Button>
            <Button type="text" size="mini" onClick={handleDeselectAll}>
              清空
            </Button>
          </Space>
          <Space direction="vertical" size={4}>
            {databases.map((db) => (
              <Checkbox
                key={db}
                checked={selectedDbs.includes(db)}
                onChange={(checked) => handleToggle(db, checked)}
              >
                {db}
              </Checkbox>
            ))}
          </Space>
        </div>
      )}
    </Space>
  )
}
