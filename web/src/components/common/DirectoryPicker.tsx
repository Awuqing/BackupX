import { Button, Input, Modal, Space, Spin, Tree, Typography } from '@arco-design/web-react'
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
  isLeaf: boolean
  children?: TreeNodeData[]
}

function entriesToTreeNodes(entries: DirEntry[], mode: 'directory' | 'file'): TreeNodeData[] {
  return entries
    .filter((entry) => mode === 'file' || entry.isDir)
    .map((entry) => ({
      key: entry.path,
      title: entry.name,
      isLeaf: !entry.isDir,
      children: entry.isDir ? [] : undefined,
    }))
}

export function DirectoryPicker({ value, onChange, placeholder, mode = 'directory', nodeId }: DirectoryPickerProps) {
  const [modalVisible, setModalVisible] = useState(false)
  const [treeData, setTreeData] = useState<TreeNodeData[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedPath, setSelectedPath] = useState('')
  const [error, setError] = useState('')

  const loadDirectory = useCallback(
    async (path: string) => {
      if (nodeId === undefined) return []
      try {
        const entries = await listNodeDirectory(nodeId, path)
        return entriesToTreeNodes(entries, mode)
      } catch {
        setError('加载目录失败')
        return []
      }
    },
    [nodeId, mode],
  )

  async function handleOpen() {
    setModalVisible(true)
    setSelectedPath(value || '')
    setError('')
    setLoading(true)
    try {
      const rootNodes = await loadDirectory('/')
      setTreeData(rootNodes)
    } finally {
      setLoading(false)
    }
  }

  async function handleLoadMore(node: TreeNodeData) {
    const children = await loadDirectory(node.key)
    setTreeData((prev) => {
      function updateChildren(nodes: TreeNodeData[]): TreeNodeData[] {
        return nodes.map((n) => {
          if (n.key === node.key) {
            return { ...n, children }
          }
          if (n.children) {
            return { ...n, children: updateChildren(n.children) }
          }
          return n
        })
      }
      return updateChildren(prev)
    })
  }

  function handleConfirm() {
    if (selectedPath) {
      onChange(selectedPath)
    }
    setModalVisible(false)
  }

  if (nodeId === undefined) {
    return <Input value={value} placeholder={placeholder} onChange={onChange} />
  }

  return (
    <>
      <Space style={{ width: '100%' }}>
        <Input style={{ flex: 1 }} value={value} placeholder={placeholder} onChange={onChange} />
        <Button type="outline" size="small" onClick={handleOpen}>
          浏览
        </Button>
      </Space>

      <Modal
        title={mode === 'directory' ? '选择目录' : '选择文件'}
        visible={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={handleConfirm}
        okText="选择"
        cancelText="取消"
        style={{ width: 520 }}
        okButtonProps={{ disabled: !selectedPath }}
      >
        {error && <Typography.Text type="error">{error}</Typography.Text>}
        {selectedPath && (
          <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
            已选择: {selectedPath}
          </Typography.Text>
        )}
        {loading ? (
          <Spin style={{ display: 'block', textAlign: 'center', padding: 24 }} />
        ) : treeData.length === 0 ? (
          <Typography.Text type="secondary">目录为空</Typography.Text>
        ) : (
          <div style={{ maxHeight: 400, overflow: 'auto' }}>
            <Tree
              treeData={treeData as any}
              onSelect={(keys) => {
                if (keys.length > 0) {
                  setSelectedPath(keys[0] as string)
                }
              }}
              selectedKeys={selectedPath ? [selectedPath] : []}
              loadMore={(node: any) => handleLoadMore(node.props)}
            />
          </div>
        )}
      </Modal>
    </>
  )
}
