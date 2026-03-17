export function formatDateTime(value?: string | Date | null) {
  if (!value) {
    return '-'
  }
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

export function formatBytes(value?: number | null) {
  if (!value || value <= 0) {
    return '0 B'
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = value
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  const digits = current >= 10 || index === 0 ? 0 : 1
  const formatted = current.toFixed(digits).replace(/\.0$/, '')
  return `${formatted} ${units[index]}`
}

export function formatPercent(value?: number | null) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '0%'
  }
  return `${(value * 100).toFixed(value >= 0.1 ? 0 : 1)}%`
}

export function formatDuration(seconds?: number | null) {
  if (!seconds || seconds <= 0) {
    return '0 秒'
  }
  if (seconds < 60) {
    return `${seconds} 秒`
  }
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainSeconds = seconds % 60
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分 ${remainSeconds} 秒`
  }
  return `${minutes} 分 ${remainSeconds} 秒`
}
