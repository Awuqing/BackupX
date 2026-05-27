import { Alert, Input, Modal, Spin, Table, Tag, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'
import { getBackupRecordContents } from '../../services/backup-records'
import type { BackupRecordContentEntry, BackupRecordContents } from '../../types/backup-records'
import { resolveErrorMessage } from '../../utils/error'
import { formatBytes } from '../../utils/format'

interface BackupRecordContentsModalProps {
  visible: boolean
  recordId?: number
  onClose: () => void
}

// BackupRecordContentsModal 浏览某次备份捕获的文件清单（只读）。
// 数据来源于全量备份记录的清单，无需下载归档，秒级展示并支持按路径筛选。
export function BackupRecordContentsModal({ visible, recordId, onClose }: BackupRecordContentsModalProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [contents, setContents] = useState<BackupRecordContents | null>(null)
  const [keyword, setKeyword] = useState('')

  useEffect(() => {
    if (!visible || !recordId) {
      return
    }
    let active = true
    setLoading(true)
    setError('')
    setKeyword('')
    setContents(null)
    void (async () => {
      try {
        const data = await getBackupRecordContents(recordId)
        if (active) {
          setContents(data)
        }
      } catch (e) {
        if (active) {
          setError(resolveErrorMessage(e, '加载备份内容失败'))
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    })()
    return () => {
      active = false
    }
  }, [visible, recordId])

  const filtered = useMemo(() => {
    const entries = contents?.entries ?? []
    const kw = keyword.trim().toLowerCase()
    if (!kw) {
      return entries
    }
    return entries.filter((e) => e.path.toLowerCase().includes(kw))
  }, [contents, keyword])

  return (
    <Modal visible={visible} title="备份内容" footer={null} onCancel={onClose} unmountOnExit style={{ width: 760 }}>
      {loading ? (
        <Spin style={{ display: 'block', textAlign: 'center', padding: 40 }} />
      ) : error ? (
        <Alert type="warning" content={error} />
      ) : contents ? (
        <div>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            共 {contents.total} 个条目
            {contents.truncated ? `（清单较大，仅展示前 ${contents.entries.length} 个）` : ''}
            {contents.basedOnFull ? `；差异备份，清单取自基线全量 #${contents.basedOnFull}` : ''}
          </Typography.Text>
          <Input.Search allowClear placeholder="按路径筛选" value={keyword} onChange={setKeyword} style={{ margin: '8px 0' }} />
          <Table
            size="small"
            rowKey="path"
            data={filtered}
            pagination={{ pageSize: 50, sizeCanChange: false }}
            scroll={{ y: 420 }}
            columns={[
              {
                title: '路径',
                dataIndex: 'path',
                render: (_: unknown, row: BackupRecordContentEntry) => (
                  <span>
                    {row.isDir ? (
                      <Tag size="small" color="arcoblue">
                        目录
                      </Tag>
                    ) : null}{' '}
                    {row.path}
                  </span>
                ),
              },
              {
                title: '大小',
                dataIndex: 'size',
                width: 120,
                align: 'right',
                render: (_: unknown, row: BackupRecordContentEntry) => (row.isDir ? '-' : formatBytes(row.size)),
              },
            ]}
          />
        </div>
      ) : null}
    </Modal>
  )
}
