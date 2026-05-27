import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { ComplianceReport } from '../types/reports'

export async function fetchComplianceReport(days = 30) {
  const response = await http.get<ApiEnvelope<ComplianceReport>>('/reports/compliance', { params: { days } })
  return unwrapApiEnvelope(response.data)
}

// downloadComplianceCSV 通过带认证的 http 客户端拉取 CSV blob 并触发浏览器下载。
export async function downloadComplianceCSV(days = 30) {
  const response = await http.get('/reports/compliance/export', {
    params: { days },
    responseType: 'blob',
  })
  const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `backupx-compliance-${days}d.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
