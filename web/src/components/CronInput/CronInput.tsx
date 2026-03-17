import { Input, Space, Switch, Tabs, Typography, Radio, Checkbox, Select } from '@arco-design/web-react'
import { useEffect, useState } from 'react'

export interface CronInputProps {
  value?: string
  onChange?: (value: string) => void
}

const DEFAULT_CRON = '* * * * *'

type CronPart = 'minute' | 'hour' | 'day' | 'month' | 'week'

interface CronState {
  minute: string
  hour: string
  day: string
  month: string
  week: string
}

function parseCron(expr: string): CronState {
  const parts = (expr || DEFAULT_CRON).trim().split(/\s+/)
  return {
    minute: parts[0] || '*',
    hour: parts[1] || '*',
    day: parts[2] || '*',
    month: parts[3] || '*',
    week: parts[4] || '*',
  }
}

function stringifyCron(state: CronState): string {
  return `${state.minute} ${state.hour} ${state.day} ${state.month} ${state.week}`
}

function generateOptions(min: number, max: number) {
  return Array.from({ length: max - min + 1 }, (_, i) => ({
    label: String(i + min),
    value: String(i + min),
  }))
}

const MINUTES_OPTIONS = generateOptions(0, 59)
const HOURS_OPTIONS = generateOptions(0, 23)
const DAYS_OPTIONS = generateOptions(1, 31)
const MONTHS_OPTIONS = generateOptions(1, 12)
const WEEKS_OPTIONS = [
  { label: '星期日', value: '0' },
  { label: '星期一', value: '1' },
  { label: '星期二', value: '2' },
  { label: '星期三', value: '3' },
  { label: '星期四', value: '4' },
  { label: '星期五', value: '5' },
  { label: '星期六', value: '6' },
]

export function CronInput({ value, onChange }: CronInputProps) {
  const [internalValue, setInternalValue] = useState(value || DEFAULT_CRON)
  const [isAdvanced, setIsAdvanced] = useState(false)
  const [state, setState] = useState<CronState>(parseCron(internalValue))

  // Sync prop to internal state
  useEffect(() => {
    if (value !== undefined && value !== internalValue) {
      setInternalValue(value || DEFAULT_CRON)
      if (!isAdvanced) {
        setState(parseCron(value || DEFAULT_CRON))
      }
    }
  }, [value, isAdvanced, internalValue])

  const notifyChange = (nextValue: string) => {
    setInternalValue(nextValue)
    if (onChange) {
      onChange(nextValue)
    }
  }

  const handleStateChange = (part: CronPart, val: string) => {
    const nextState = { ...state, [part]: val }
    setState(nextState)
    notifyChange(stringifyCron(nextState))
  }

  const renderPartTab = (
    part: CronPart,
    title: string,
    options: { label: string; value: string }[],
    allowAnyVal = '*',
  ) => {
    const currentVal = state[part]
    const isAny = currentVal === allowAnyVal || currentVal === '*' || currentVal === '?'
    const isSpecific = !isAny && !currentVal.includes('/') && !currentVal.includes('-')
    
    // For simplicity in this visual editor, we only support "every" (*) and "specific values" (1,2,3).
    const type = isAny ? 'any' : 'specific'
    const specificValues = isSpecific ? currentVal.split(',') : []

    return (
      <div style={{ padding: '16px 0' }}>
        <Radio.Group
          direction="vertical"
          value={type}
          onChange={(val) => {
            if (val === 'any') {
              handleStateChange(part, allowAnyVal)
            } else {
              handleStateChange(part, options[0].value) // Default to first valid item
            }
          }}
        >
          <Radio value="any">
            <Typography.Text>通配 ({allowAnyVal}) - 任意{title}</Typography.Text>
          </Radio>
          <Radio value="specific">
            <Typography.Text>指定{title}</Typography.Text>
          </Radio>
        </Radio.Group>

        {type === 'specific' && (
          <div style={{ paddingLeft: 24, marginTop: 12 }}>
            <Select
              mode="multiple"
              placeholder={`请选择${title}`}
              value={specificValues}
              options={options}
              onChange={(vals: string[]) => {
                if (vals.length === 0) {
                  handleStateChange(part, allowAnyVal)
                } else {
                  // Sort numerically to keep things neat
                  const sorted = [...vals].sort((a, b) => Number(a) - Number(b))
                  handleStateChange(part, sorted.join(','))
                }
              }}
              style={{ width: '100%', maxWidth: 400 }}
              allowClear
            />
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="cron-input-container">
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Input
          value={internalValue}
          onChange={(val) => {
            setInternalValue(val)
            if (isAdvanced && onChange) {
              onChange(val)
            }
          }}
          readOnly={!isAdvanced}
          style={{ width: 240, fontFamily: 'monospace' }}
          placeholder="* * * * *"
        />
        <Space>
          <Typography.Text type="secondary">高级模式 (手动输入)</Typography.Text>
          <Switch
            checked={isAdvanced}
            onChange={(checked) => {
              setIsAdvanced(checked)
              if (!checked) {
                // When switching back to visual, parse the current raw value
                setState(parseCron(internalValue))
                notifyChange(stringifyCron(parseCron(internalValue)))
              }
            }}
          />
        </Space>
      </div>

      {!isAdvanced && (
        <Tabs type="card-gutter" size="small">
          <Tabs.TabPane key="minute" title="分钟">
            {renderPartTab('minute', '分钟', MINUTES_OPTIONS, '*')}
          </Tabs.TabPane>
          <Tabs.TabPane key="hour" title="小时">
            {renderPartTab('hour', '小时', HOURS_OPTIONS, '*')}
          </Tabs.TabPane>
          <Tabs.TabPane key="day" title="日">
            {renderPartTab('day', '日', DAYS_OPTIONS, '*')}
          </Tabs.TabPane>
          <Tabs.TabPane key="month" title="月">
            {renderPartTab('month', '月', MONTHS_OPTIONS, '*')}
          </Tabs.TabPane>
          <Tabs.TabPane key="week" title="周">
            {renderPartTab('week', '周', WEEKS_OPTIONS, '*')}
          </Tabs.TabPane>
        </Tabs>
      )}
    </div>
  )
}
