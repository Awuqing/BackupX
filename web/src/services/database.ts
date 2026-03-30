import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'

export interface DatabaseDiscoverPayload {
  type: 'mysql' | 'postgresql'
  host: string
  port: number
  user: string
  password: string
}

interface DatabaseDiscoverResult {
  databases: string[]
}

export async function discoverDatabases(payload: DatabaseDiscoverPayload): Promise<string[]> {
  const response = await http.post<ApiEnvelope<DatabaseDiscoverResult>>('/database/discover', payload, { timeout: 10000 })
  return unwrapApiEnvelope(response.data).databases ?? []
}
