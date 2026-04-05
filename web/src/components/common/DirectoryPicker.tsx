import { Button, Input, Message, Modal, Space, Spin, Tree, Typography, Empty } from '@arco-design/web-react'
import { IconFolder, IconFile, IconFolderAdd } from '@arco-design/web-react/icon'
import { useCallback, useState } from 'react'
import { listNodeDirectory } from '../../services/nodes'
import type { DirEntry } from '../../types/nodes'

interface DirectoryPickerProps {
  value: string
  onChange: (path: string) => void
  placeholder?: string
  mode?: 'directory' | 'file'
  nodeId?: number
}

interface TreeNodeData {
  key: string
  title: string
  icon?: React.ReactNode
  isLeaf: boolean
  children?: TreeNodeData[]
  loaded?: boolean
}

function entriesToTreeNodes(entries: DirEntry[], mode: 'directory' | 'file'): TreeNodeData[] {
  return entries
    .filter((entry) => mode === 'file' || entry.isDir)
    .map((entry) => ({
      key: entry.path,
      title: entry.name,
      icon: entry.isDir ? <IconFolder style={{ color: 'var(--color-warning-6)' }} /> : <IconFile />,
      isLeaf: !entry.isDir,
    }))
}

export function DirectoryPicker({ value, onChange, placeholder, mode = 'directory', nodeId }: DirectoryPickerProps) {
  const [modalVisible, setModalVisible] = useState(false)
  const [treeData, setTreeData] = useState<TreeNodeData[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedPath, setSelectedPath] = useState('')

  const loadDirectory = useCallback(
    async (path: string): Promise<TreeNodeData[]> => {
      if (nodeId === undefined) return []
      try {
        const entries = await listNodeDirectory(nodeId, path)
        return entriesToTreeNodes(entries, mode)
      } catch {
        Message.error(`加载目录失败: ${path}`)
        return []
      }
    },
    [nodeId, mode],
  )

  async function handleOpen() {
    setModalVisible(true)
    setSelectedPath(value || '')
    setLoading(true)
    try {
      const rootNodes = await loadDirectory('/')
      setTreeData(rootNodes)
    } finally {
      setLoading(false)
    }
  }

  // ArcoDesign Tree loadMore: node.props.dataRef 指向 treeData 中的原始对象
  async function handleLoadMore(treeNode: any): Promise<void> {
    const nodeKey = treeNode.props.dataRef?.key ?? treeNode.props._key
    if (!nodeKey) return

    const children = await loadDirectory(nodeKey)

    setTreeData((prev) => {
      function insertChildren(nodes: TreeNodeData[]): TreeNodeData[] {
        return nodes.map((n) => {
          if (n.key === nodeKey) {
            return { ...n, children, loaded: true }
          }
          if (n.children && n.children.length > 0) {
            return { ...n, children: insertChildren(n.children) }
          }
          return n
        })
      }
      return insertChildren(prev)
    })
  }

  function handleConfirm() {
    if (selectedPath) {
      onChange(selectedPath)
    }
    setModalVisible(false)
  }

  function handleInputKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      const trimmed = value?.trim()
      if (trimmed) {
        onChange(trimmed)
      }
    }
  }

  // 没有 nodeId 时退化为普通输入框
  if (nodeId === undefined) {
    return <Input value={value} placeholder={placeholder} onChange={onChange} onKeyDown={handleInputKeyDown} />
  }

  return (
    <>
      <div style={{ display: 'flex', gap: 8, width: '100%' }}>
        <Input
          style={{ flex: 1 }}
          value={value}
          placeholder={placeholder}
          onChange={onChange}
          onKeyDown={handleInputKeyDown}
          allowClear
        />
        <Button type="outline" size="default" onClick={handleOpen} icon={<IconFolderAdd />}>
          浏览
        </Button>
      </div>

      <Modal
        title={mode === 'directory' ? '选择目录' : '选择文件'}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={handleConfirm}
        okText="确认选择"
        cancelText="取消"
        style={{ width: 640 }}
        okButtonProps={{ disabled: !selectedPath }}
        unmountOnExit
      >
        {/* 当前选中路径 */}
        <div style={{
          padding: '10px 14px',
          marginBottom: 16,
          background: selectedPath ? 'var(--color-primary-light-1)' : 'var(--color-fill-2)',
          borderRadius: 6,
          border: selectedPath ? '1px solid var(--color-primary-light-3)' : '1px solid var(--color-border)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          minHeight: 40,
        }}>
          <IconFolder style={{ color: selectedPath ? 'var(--color-primary-6)' : 'var(--color-text-4)', fontSize: 16, flexShrink: 0 }} />
          {selectedPath ? (
            <Typography.Text copyable style={{ fontSize: 13, fontFamily: 'monospace', wordBreak: 'break-all' }}>
              {selectedPath}
            </Typography.Text>
          ) : (
            <Typography.Text type="secondary" style={{ fontSize: 13 }}>请在下方目录树中选择路径</Typography.Text>
          )}
        </div>

        {/* 目录树 */}
        {loading ? (
          <Spin style={{ display: 'block', textAlign: 'center', padding: 48 }} tip="加载目录中..." />
        ) : treeData.length === 0 ? (
          <Empty style={{ padding: 48 }} description="目录为空" />
        ) : (
          <div style={{
            maxHeight: 400,
            overflow: 'auto',
            border: '1px solid var(--color-border)',
            borderRadius: 6,
            padding: '6px 0',
          }}>
            <Tree
              blockNode
              showLine
              treeData={treeData as any}
              selectedKeys={selectedPath ? [selectedPath] : []}
              onSelect={(keys) => {
                if (keys.length > 0) {
                  setSelectedPath(keys[0] as string)
                }
              }}
              loadMore={handleLoadMore}
              icons={{ switcherIcon: <IconFolder style={{ fontSize: 14 }} /> }}
            />
          </div>
        )}
      </Modal>
    </>
  )
}
