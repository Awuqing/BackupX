import { Empty, Input, Modal, Space, Spin, Tag, Typography } from '@arco-design/web-react'
import { IconSearch } from '@arco-design/web-react/icon'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { globalSearch, type SearchKind, type SearchResult, type SearchResultItem } from '../../services/search'

const KIND_LABELS: Record<SearchKind, string> = {
  task: '任务',
  record: '备份记录',
  storage: '存储目标',
  node: '节点',
}

const KIND_COLORS: Record<SearchKind, string> = {
  task: 'arcoblue',
  record: 'green',
  storage: 'orange',
  node: 'purple',
}

/**
 * GlobalSearch 顶部 Header 的全局搜索入口。
 * Ctrl/Cmd+K 快捷键唤起 Modal，输入 300ms 后触发搜索，避免高频请求。
 */
export function GlobalSearch() {
  const navigate = useNavigate()
  const [visible, setVisible] = useState(false)
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<SearchResult | null>(null)
  const [loading, setLoading] = useState(false)
  const debounceTimer = useRef<number | null>(null)

  // Ctrl/Cmd+K 快捷唤起
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setVisible(true)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  const runSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResult(null)
      return
    }
    setLoading(true)
    try {
      const res = await globalSearch(q)
      setResult(res)
    } catch {
      setResult(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (debounceTimer.current) {
      window.clearTimeout(debounceTimer.current)
    }
    debounceTimer.current = window.setTimeout(() => {
      void runSearch(query)
    }, 300)
    return () => {
      if (debounceTimer.current) {
        window.clearTimeout(debounceTimer.current)
      }
    }
  }, [query, runSearch])

  function handleNavigate(item: SearchResultItem) {
    setVisible(false)
    navigate(item.url)
  }

  function renderSection(kind: SearchKind, items: SearchResultItem[]) {
    if (items.length === 0) return null
    return (
      <div key={kind} style={{ marginBottom: 16 }}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {KIND_LABELS[kind]}（{items.length}）
        </Typography.Text>
        <div style={{ marginTop: 4 }}>
          {items.map((item) => (
            <div
              key={`${kind}-${item.id}`}
              onClick={() => handleNavigate(item)}
              style={{
                padding: '8px 12px',
                cursor: 'pointer',
                borderRadius: 4,
                marginBottom: 4,
                backgroundColor: 'var(--color-fill-1)',
              }}
              onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = 'var(--color-primary-light-1)' }}
              onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'var(--color-fill-1)' }}
            >
              <Space>
                <Tag color={KIND_COLORS[kind]} bordered size="small">{KIND_LABELS[kind]}</Tag>
                <Typography.Text bold>{item.title}</Typography.Text>
                {item.subtitle && <Typography.Text type="secondary" style={{ fontSize: 12 }}>{item.subtitle}</Typography.Text>}
              </Space>
            </div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <>
      <div
        onClick={() => setVisible(true)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '4px 12px',
          borderRadius: 16,
          cursor: 'pointer',
          backgroundColor: 'var(--color-fill-2)',
          color: 'var(--color-text-3)',
          fontSize: 12,
        }}
      >
        <IconSearch />
        <span>搜索任务/记录/存储... (Ctrl+K)</span>
      </div>

      <Modal
        visible={visible}
        title={null}
        footer={null}
        onCancel={() => setVisible(false)}
        style={{ width: 720, top: 80 }}
        unmountOnExit
        maskClosable
      >
        <Input
          autoFocus
          size="large"
          value={query}
          placeholder="输入关键词搜索任务、备份记录、存储目标、节点..."
          prefix={<IconSearch />}
          onChange={setQuery}
          allowClear
        />
        <div style={{ marginTop: 16, maxHeight: 480, overflow: 'auto' }}>
          {loading ? (
            <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
          ) : !result || result.totalCount === 0 ? (
            <Empty description={query ? '未找到匹配项' : '开始输入以搜索'} />
          ) : (
            <>
              {renderSection('task', result.tasks)}
              {renderSection('record', result.records)}
              {renderSection('storage', result.storage)}
              {renderSection('node', result.nodes)}
            </>
          )}
        </div>
      </Modal>
    </>
  )
}
