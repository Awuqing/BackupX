import { Alert, Button, Card, Empty, Form, Input, InputNumber, Message, Modal, Select, Space, Table, Tag, Typography } from '@arco-design/web-react'
import { useCallback, useEffect, useState } from 'react'
import { applyTaskTemplate, deleteTaskTemplate, getTaskTemplate, listTaskTemplates, type TaskTemplateApplyResult, type TaskTemplateSummary, type TaskTemplateVariables } from '../../services/task-templates'
import { useAuthStore } from '../../stores/auth'
import { resolveErrorMessage } from '../../utils/error'
import { canWrite } from '../../utils/permissions'
import { formatDateTime } from '../../utils/format'

interface VariableRow extends TaskTemplateVariables {
  key: string
}

function newRow(defaults?: Partial<TaskTemplateVariables>): VariableRow {
  return {
    key: Math.random().toString(36).slice(2),
    name: '',
    sourcePath: defaults?.sourcePath ?? '',
    dbHost: defaults?.dbHost ?? '',
    dbName: defaults?.dbName ?? '',
    tags: defaults?.tags ?? '',
  }
}

// TaskTemplatesPage 任务模板管理 + 批量创建。
// 仅 operator/admin 角色看到全部操作，viewer 仅查看列表。
export function TaskTemplatesPage() {
  const user = useAuthStore((s) => s.user)
  const writable = canWrite(user)
  const [items, setItems] = useState<TaskTemplateSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [applyVisible, setApplyVisible] = useState(false)
  const [applyTemplateId, setApplyTemplateId] = useState<number | null>(null)
  const [applyTemplateName, setApplyTemplateName] = useState('')
  const [rows, setRows] = useState<VariableRow[]>([newRow()])
  const [applyResult, setApplyResult] = useState<TaskTemplateApplyResult[] | null>(null)
  const [applying, setApplying] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setItems(await listTaskTemplates())
      setError('')
    } catch (e) {
      setError(resolveErrorMessage(e, '加载任务模板失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function openApply(item: TaskTemplateSummary) {
    try {
      const detail = await getTaskTemplate(item.id)
      setApplyTemplateId(item.id)
      setApplyTemplateName(item.name)
      setRows([newRow({ sourcePath: detail.payload.sourcePath, dbHost: detail.payload.dbHost, dbName: detail.payload.dbName, tags: detail.payload.tags })])
      setApplyResult(null)
      setApplyVisible(true)
    } catch (e) {
      Message.error(resolveErrorMessage(e, '加载模板失败'))
    }
  }

  async function handleApply() {
    if (!applyTemplateId) return
    const variables: TaskTemplateVariables[] = rows
      .filter((r) => r.name.trim())
      .map((r) => ({
        name: r.name.trim(),
        sourcePath: r.sourcePath?.trim() || undefined,
        dbHost: r.dbHost?.trim() || undefined,
        dbName: r.dbName?.trim() || undefined,
        tags: r.tags?.trim() || undefined,
        nodeId: r.nodeId,
      }))
    if (variables.length === 0) {
      Message.error('至少填写一条任务名称')
      return
    }
    setApplying(true)
    try {
      const result = await applyTaskTemplate(applyTemplateId, variables)
      setApplyResult(result)
      const succ = result.filter((r) => r.success).length
      Message.success(`已创建 ${succ}/${result.length} 个任务`)
    } catch (e) {
      Message.error(resolveErrorMessage(e, '应用模板失败'))
    } finally {
      setApplying(false)
    }
  }

  async function handleDelete(item: TaskTemplateSummary) {
    if (!window.confirm(`确定删除模板「${item.name}」？`)) return
    try {
      await deleteTaskTemplate(item.id)
      Message.success('已删除')
      await load()
    } catch (e) {
      Message.error(resolveErrorMessage(e, '删除失败'))
    }
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <div>
        <Typography.Title heading={4}>任务模板</Typography.Title>
        <Typography.Paragraph type="secondary">
          保存常用任务参数预设，一次性批量创建任务。适合大规模场景（100+ 主机）。在任务表单点击"保存为模板"可创建模板。
        </Typography.Paragraph>
      </div>

      {error ? <Alert type="error" content={error} /> : null}

      <Card>
        {items.length === 0 && !loading ? (
          <Empty description="暂无模板。在创建任务时勾选'保存为模板'或通过 API 创建。" />
        ) : (
          <Table
            rowKey="id"
            loading={loading}
            data={items}
            stripe
            pagination={false}
            columns={[
              { title: '名称', render: (_: unknown, r: TaskTemplateSummary) => (
                <Space direction="vertical" size={2}>
                  <Typography.Text bold>{r.name}</Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.description || '-'}</Typography.Text>
                </Space>
              )},
              { title: '类型', dataIndex: 'taskType', render: (v: string) => <Tag color="arcoblue" bordered>{v.toUpperCase()}</Tag> },
              { title: '创建者', dataIndex: 'createdBy', render: (v: string) => v || '-' },
              { title: '创建时间', dataIndex: 'createdAt', render: (v: string) => formatDateTime(v) },
              { title: '操作', width: 240, render: (_: unknown, r: TaskTemplateSummary) => (
                <Space>
                  {writable && <Button size="small" type="primary" onClick={() => void openApply(r)}>应用</Button>}
                  {writable && <Button size="small" type="text" status="danger" onClick={() => void handleDelete(r)}>删除</Button>}
                </Space>
              )},
            ]}
          />
        )}
      </Card>

      <Modal
        visible={applyVisible}
        title={`应用模板：${applyTemplateName}`}
        onCancel={() => setApplyVisible(false)}
        onOk={applyResult ? () => setApplyVisible(false) : handleApply}
        okText={applyResult ? '完成' : '批量创建'}
        confirmLoading={applying}
        style={{ width: 780 }}
        unmountOnExit
      >
        {applyResult ? (
          <Table
            rowKey="name"
            pagination={false}
            data={applyResult}
            columns={[
              { title: '任务名', dataIndex: 'name' },
              { title: '结果', dataIndex: 'success', render: (v: boolean) => v ? <Tag color="green" bordered>成功</Tag> : <Tag color="red" bordered>失败</Tag> },
              { title: '任务 ID', dataIndex: 'taskId', render: (v?: number) => v ? `#${v}` : '-' },
              { title: '错误', dataIndex: 'error', render: (v?: string) => v || '-' },
            ]}
          />
        ) : (
          <Space direction="vertical" size="medium" style={{ width: '100%' }}>
            <Alert type="info" content="每行一个任务。仅 name 必填；其他字段非空时覆盖模板。" />
            <Table
              rowKey="key"
              pagination={false}
              data={rows}
              size="small"
              columns={[
                { title: '任务名 *', width: 160, render: (_: unknown, r: VariableRow, idx: number) => (
                  <Input value={r.name} onChange={(v) => setRows((list) => list.map((x, i) => i === idx ? { ...x, name: v } : x))} placeholder="如：prod-web-1" />
                )},
                { title: '源路径', width: 200, render: (_: unknown, r: VariableRow, idx: number) => (
                  <Input value={r.sourcePath} onChange={(v) => setRows((list) => list.map((x, i) => i === idx ? { ...x, sourcePath: v } : x))} placeholder="/var/www" />
                )},
                { title: '数据库主机', width: 140, render: (_: unknown, r: VariableRow, idx: number) => (
                  <Input value={r.dbHost} onChange={(v) => setRows((list) => list.map((x, i) => i === idx ? { ...x, dbHost: v } : x))} placeholder="host-1" />
                )},
                { title: '数据库名', width: 140, render: (_: unknown, r: VariableRow, idx: number) => (
                  <Input value={r.dbName} onChange={(v) => setRows((list) => list.map((x, i) => i === idx ? { ...x, dbName: v } : x))} />
                )},
                { title: '', width: 60, render: (_: unknown, _r: VariableRow, idx: number) => (
                  <Button size="mini" type="text" status="danger" onClick={() => setRows((list) => list.filter((_, i) => i !== idx))}>删除</Button>
                )},
              ]}
            />
            <Button type="outline" long onClick={() => setRows((list) => [...list, newRow()])}>+ 新增一行</Button>
          </Space>
        )}
      </Modal>
    </Space>
  )
}

// 避免未使用变量警告
void Form
void InputNumber
void Select
