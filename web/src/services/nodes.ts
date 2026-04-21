import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { NodeSummary, DirEntry, BatchCreateResult, InstallTokenInput, InstallTokenResult } from '../types/nodes'

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

export interface NodeUpdateInput {
  name: string
  labels?: string
  maxConcurrent?: number
  bandwidthLimit?: string
}

export async function updateNode(id: number, data: NodeUpdateInput) {
  const response = await http.put<ApiEnvelope<NodeSummary>>(`/nodes/${id}`, data)
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

export async function batchCreateNodes(names: string[]) {
  const response = await http.post<ApiEnvelope<BatchCreateResult[]>>('/nodes/batch', { names })
  return unwrapApiEnvelope(response.data)
}

export async function createInstallToken(nodeId: number, input: InstallTokenInput) {
  const response = await http.post<ApiEnvelope<InstallTokenResult>>(
    `/nodes/${nodeId}/install-tokens`, input,
  )
  return unwrapApiEnvelope(response.data)
}

export async function rotateNodeToken(nodeId: number) {
  const response = await http.post<ApiEnvelope<{ newToken: string }>>(
    `/nodes/${nodeId}/rotate-token`,
  )
  return unwrapApiEnvelope(response.data)
}

export async function fetchScriptPreview(
  nodeId: number,
  params: { mode: string; arch: string; agentVersion: string; downloadSrc: string },
) {
  const response = await http.get<string>(`/nodes/${nodeId}/install-script-preview`, {
    params,
    responseType: 'text',
  })
  return response.data
}
