import { Radio, Input, Typography } from '@arco-design/web-react'

const { Text } = Typography
const TextArea = Input.TextArea

export type Mode = 'single' | 'batch'

interface Props {
  mode: Mode
  onModeChange: (m: Mode) => void
  singleName: string
  onSingleNameChange: (v: string) => void
  batchText: string
  onBatchTextChange: (v: string) => void
}

export function Step1NodeName({
  mode, onModeChange, singleName, onSingleNameChange, batchText, onBatchTextChange,
}: Props) {
  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Radio.Group
          type="button"
          value={mode}
          onChange={(v) => onModeChange(v as Mode)}
          options={[
            { label: '单节点', value: 'single' },
            { label: '批量创建', value: 'batch' },
          ]}
        />
      </div>
      {mode === 'single' ? (
        <div>
          <Text bold style={{ marginBottom: 6, display: 'block' }}>节点名称</Text>
          <Input
            placeholder="如：prod-db-01"
            value={singleName}
            onChange={onSingleNameChange}
            maxLength={128}
          />
        </div>
      ) : (
        <div>
          <Text bold style={{ marginBottom: 6, display: 'block' }}>节点名称（每行一个，最多 50 个）</Text>
          <TextArea
            rows={8}
            placeholder={'prod-db-01\nprod-db-02\nprod-web-01'}
            value={batchText}
            onChange={onBatchTextChange}
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
          <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
            空行自动忽略；重名会在提交时报错
          </Text>
        </div>
      )}
    </div>
  )
}
