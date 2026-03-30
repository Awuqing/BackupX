import { Button, Input, Message, Modal, Space, Spin, Tree, Typography } from '@arco-design/web-react'
import { IconFolder, IconFile } from '@arco-design/web-react/icon'
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
      icon: entry.isDir ? <IconFolder /> : <IconFile />,
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

  // 没有 nodeId 时退化为普通输入框
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
        style={{ width: 560 }}
        okButtonProps={{ disabled: !selectedPath }}
        unmountOnExit
      >
        {selectedPath && (
          <div style={{ padding: '8px 12px', marginBottom: 12, background: 'var(--color-fill-2)', borderRadius: 4 }}>
            <Typography.Text copyable style={{ fontSize: 13 }}>
              {selectedPath}
            </Typography.Text>
          </div>
        )}
        {loading ? (
          <Spin style={{ display: 'block', textAlign: 'center', padding: 40 }} />
        ) : treeData.length === 0 ? (
          <Typography.Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 40 }}>
            目录为空
          </Typography.Text>
        ) : (
          <div style={{ maxHeight: 420, overflow: 'auto', border: '1px solid var(--color-border)', borderRadius: 4, padding: '4px 0' }}>
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
