import { Button, Divider, Input, Select, Space, Switch, Typography } from '@arco-design/web-react'
import { useEffect, useMemo, useState } from 'react'

export interface CronInputProps {
  value?: string
  onChange?: (value: string) => void
}

const DEFAULT_CRON = '0 2 * * *'

// 常用预设
const PRESETS = [
  { label: '每天 02:00', value: '0 2 * * *' },
  { label: '每天 00:00', value: '0 0 * * *' },
  { label: '每 6 小时', value: '0 */6 * * *' },
  { label: '每 12 小时', value: '0 */12 * * *' },
  { label: '每周日 03:00', value: '0 3 * * 0' },
  { label: '每月 1 日 02:00', value: '0 2 1 * *' },
  { label: '每 30 分钟', value: '*/30 * * * *' },
  { label: '每小时整点', value: '0 * * * *' },
]

const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) => ({
  label: `${String(i).padStart(2, '0')} 时`,
  value: String(i),
}))

const MINUTE_OPTIONS = Array.from({ length: 12 }, (_, i) => ({
  label: `${String(i * 5).padStart(2, '0')} 分`,
  value: String(i * 5),
}))

const WEEKDAY_OPTIONS = [
  { label: '周一', value: '1' },
  { label: '周二', value: '2' },
  { label: '周三', value: '3' },
  { label: '周四', value: '4' },
  { label: '周五', value: '5' },
  { label: '周六', value: '6' },
  { label: '周日', value: '0' },
]

const DAY_OPTIONS = Array.from({ length: 31 }, (_, i) => ({
  label: `${i + 1} 日`,
  value: String(i + 1),
}))

type ScheduleMode = 'daily' | 'weekly' | 'monthly' | 'interval'

// 将 cron 表达式转为自然语言中文描述
function describeCron(expr: string): string {
  const parts = expr.trim().split(/\s+/)
  if (parts.length !== 5) return ''
  const [minute, hour, day, _month, week] = parts

  // 每 N 分钟
  if (minute.includes('/') && hour === '*' && day === '*' && week === '*') {
    return `每 ${minute.split('/')[1]} 分钟执行一次`
  }
  // 每 N 小时
  if (minute !== '*' && hour.includes('/') && day === '*' && week === '*') {
    return `每 ${hour.split('/')[1]} 小时执行一次（在第 ${minute} 分）`
  }
  // 每小时
  if (minute !== '*' && hour === '*' && day === '*' && week === '*') {
    return `每小时的第 ${minute} 分执行`
  }

  const hh = hour.padStart(2, '0')
  const mm = minute.padStart(2, '0')
  const time = `${hh}:${mm}`

  // 每周某天
  if (day === '*' && week !== '*') {
    const weekNames: Record<string, string> = { '0': '日', '1': '一', '2': '二', '3': '三', '4': '四', '5': '五', '6': '六', '7': '日' }
    const days = week.split(',').map((w) => `周${weekNames[w] || w}`).join('、')
    return `每${days} ${time} 执行`
  }
  // 每月某日
  if (day !== '*' && week === '*') {
    return `每月 ${day} 日 ${time} 执行`
  }
  // 每天
  if (day === '*' && week === '*' && hour !== '*' && !hour.includes('/')) {
    return `每天 ${time} 执行`
  }

  return ''
}

export function CronInput({ value, onChange }: CronInputProps) {
  const [cronExpr, setCronExpr] = useState(value || DEFAULT_CRON)
  const [isAdvanced, setIsAdvanced] = useState(false)
  const [showCustom, setShowCustom] = useState(false)

  // 自定义模式的状态
  const [mode, setMode] = useState<ScheduleMode>('daily')
  const [customHour, setCustomHour] = useState('2')
  const [customMinute, setCustomMinute] = useState('0')
  const [customWeekdays, setCustomWeekdays] = useState<string[]>(['0'])
  const [customDay, setCustomDay] = useState('1')
  const [customInterval, setCustomInterval] = useState('6')

  // 从 prop 同步
  useEffect(() => {
    if (value !== undefined && value !== cronExpr) {
      setCronExpr(value || DEFAULT_CRON)
    }
  }, [value])

  const description = useMemo(() => describeCron(cronExpr), [cronExpr])
  const isPreset = PRESETS.some((p) => p.value === cronExpr)

  const emit = (expr: string) => {
    setCronExpr(expr)
    onChange?.(expr)
  }

  // 从自定义选择器构建 cron
  const buildCustomCron = (
    m: ScheduleMode,
    h: string,
    min: string,
    weekdays: string[],
    day: string,
    interval: string,
  ) => {
    switch (m) {
      case 'daily':
        return `${min} ${h} * * *`
      case 'weekly':
        return `${min} ${h} * * ${weekdays.sort().join(',') || '0'}`
      case 'monthly':
        return `${min} ${h} ${day} * *`
      case 'interval':
        return `0 */${interval} * * *`
      default:
        return DEFAULT_CRON
    }
  }

  const handleCustomChange = (updates: {
    mode?: ScheduleMode
    hour?: string
    minute?: string
    weekdays?: string[]
    day?: string
    interval?: string
  }) => {
    const m = updates.mode ?? mode
    const h = updates.hour ?? customHour
    const min = updates.minute ?? customMinute
    const w = updates.weekdays ?? customWeekdays
    const d = updates.day ?? customDay
    const iv = updates.interval ?? customInterval

    if (updates.mode !== undefined) setMode(m)
    if (updates.hour !== undefined) setCustomHour(h)
    if (updates.minute !== undefined) setCustomMinute(min)
    if (updates.weekdays !== undefined) setCustomWeekdays(w)
    if (updates.day !== undefined) setCustomDay(d)
    if (updates.interval !== undefined) setCustomInterval(iv)

    emit(buildCustomCron(m, h, min, w, d, iv))
  }

  return (
    <div>
      {/* 预设按钮 */}
      <Space wrap size="small" style={{ marginBottom: 12 }}>
        {PRESETS.map((preset) => (
          <Button
            key={preset.value}
            size="small"
            type={cronExpr === preset.value ? 'primary' : 'secondary'}
            onClick={() => {
              emit(preset.value)
              setShowCustom(false)
              setIsAdvanced(false)
            }}
          >
            {preset.label}
          </Button>
        ))}
        <Button
          size="small"
          type={!isPreset && !isAdvanced ? 'primary' : 'secondary'}
          onClick={() => {
            setShowCustom(true)
            setIsAdvanced(false)
          }}
        >
          自定义...
        </Button>
      </Space>

      {/* 中文描述 + cron 表达式 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8 }}>
        <Input
          value={cronExpr}
          readOnly={!isAdvanced}
          style={{ width: 180, fontFamily: 'monospace', fontSize: 13 }}
          placeholder="0 2 * * *"
          onChange={(val) => {
            if (isAdvanced) emit(val)
          }}
        />
        {description && (
          <Typography.Text type="secondary">{description}</Typography.Text>
        )}
        <div style={{ marginLeft: 'auto' }}>
          <Space size="mini">
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>手动输入</Typography.Text>
            <Switch
              size="small"
              checked={isAdvanced}
              onChange={(checked) => {
                setIsAdvanced(checked)
                setShowCustom(false)
                if (!checked) {
                  setCronExpr(cronExpr)
                }
              }}
            />
          </Space>
        </div>
      </div>

      {/* 自定义选择器 */}
      {showCustom && !isAdvanced && (
        <div style={{ padding: '12px 16px', background: 'var(--color-fill-1)', borderRadius: 6 }}>
          <Space size="large" style={{ marginBottom: 12 }}>
            <Button size="small" type={mode === 'daily' ? 'primary' : 'text'} onClick={() => handleCustomChange({ mode: 'daily' })}>
              每天
            </Button>
            <Button size="small" type={mode === 'weekly' ? 'primary' : 'text'} onClick={() => handleCustomChange({ mode: 'weekly' })}>
              每周
            </Button>
            <Button size="small" type={mode === 'monthly' ? 'primary' : 'text'} onClick={() => handleCustomChange({ mode: 'monthly' })}>
              每月
            </Button>
            <Button size="small" type={mode === 'interval' ? 'primary' : 'text'} onClick={() => handleCustomChange({ mode: 'interval' })}>
              间隔
            </Button>
          </Space>

          {mode === 'interval' ? (
            <Space align="center">
              <Typography.Text>每</Typography.Text>
              <Select
                size="small"
                value={customInterval}
                style={{ width: 80 }}
                options={[
                  { label: '1', value: '1' },
                  { label: '2', value: '2' },
                  { label: '3', value: '3' },
                  { label: '4', value: '4' },
                  { label: '6', value: '6' },
                  { label: '8', value: '8' },
                  { label: '12', value: '12' },
                ]}
                onChange={(val) => handleCustomChange({ interval: val })}
              />
              <Typography.Text>小时执行一次</Typography.Text>
            </Space>
          ) : (
            <>
              {mode === 'weekly' && (
                <div style={{ marginBottom: 8 }}>
                  <Space wrap size="mini">
                    {WEEKDAY_OPTIONS.map((opt) => (
                      <Button
                        key={opt.value}
                        size="mini"
                        type={customWeekdays.includes(opt.value) ? 'primary' : 'secondary'}
                        onClick={() => {
                          const next = customWeekdays.includes(opt.value)
                            ? customWeekdays.filter((v) => v !== opt.value)
                            : [...customWeekdays, opt.value]
                          handleCustomChange({ weekdays: next.length > 0 ? next : [opt.value] })
                        }}
                      >
                        {opt.label}
                      </Button>
                    ))}
                  </Space>
                </div>
              )}
              {mode === 'monthly' && (
                <div style={{ marginBottom: 8 }}>
                  <Space align="center">
                    <Typography.Text>每月</Typography.Text>
                    <Select
                      size="small"
                      value={customDay}
                      style={{ width: 90 }}
                      options={DAY_OPTIONS}
                      onChange={(val) => handleCustomChange({ day: val })}
                    />
                  </Space>
                </div>
              )}
              <Space align="center">
                <Typography.Text>执行时间</Typography.Text>
                <Select
                  size="small"
                  value={customHour}
                  style={{ width: 90 }}
                  options={HOUR_OPTIONS}
                  onChange={(val) => handleCustomChange({ hour: val })}
                />
                <Typography.Text>:</Typography.Text>
                <Select
                  size="small"
                  value={customMinute}
                  style={{ width: 90 }}
                  options={MINUTE_OPTIONS}
                  onChange={(val) => handleCustomChange({ minute: val })}
                />
              </Space>
            </>
          )}
        </div>
      )}
    </div>
  )
}
