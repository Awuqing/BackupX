import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'

export interface DatabaseDiscoverPayload {
  type: 'mysql' | 'postgresql'
  host: string
  port: number
  user: string
  password: string
  /** 指定执行发现的节点。0 或省略表示 Master 本地执行；远程节点 ID 将通过 Agent 路由。 */
  nodeId?: number
}

interface DatabaseDiscoverResult {
  databases: string[]
}

export async function discoverDatabases(payload: DatabaseDiscoverPayload): Promise<string[]> {
  const response = await http.post<ApiEnvelope<DatabaseDiscoverResult>>('/database/discover', payload, { timeout: 20000 })
  return unwrapApiEnvelope(response.data).databases ?? []
}
