import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { NodeSummary, DirEntry } from '../types/nodes'

export async function listNodes() {
  const response = await http.get<ApiEnvelope<NodeSummary[]>>('/nodes')
  return unwrapApiEnvelope(response.data)
}

export async function getNode(id: number) {
  const response = await http.get<ApiEnvelope<NodeSummary>>(`/nodes/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function createNode(name: string) {
  const response = await http.post<ApiEnvelope<{ token: string }>>('/nodes', { name })
  return unwrapApiEnvelope(response.data)
}

export async function deleteNode(id: number) {
  const response = await http.delete<ApiEnvelope<null>>(`/nodes/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function listNodeDirectory(nodeId: number, path: string) {
  const response = await http.get<ApiEnvelope<DirEntry[]>>(`/nodes/${nodeId}/fs/list`, { params: { path } })
  return unwrapApiEnvelope(response.data)
}
