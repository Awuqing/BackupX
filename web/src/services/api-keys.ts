import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { UserRole } from './users'

export interface ApiKeySummary {
  id: number
  name: string
  role: UserRole
  prefix: string
  createdBy: string
  lastUsedAt?: string
  expiresAt?: string
  disabled: boolean
  createdAt: string
}

export interface ApiKeyCreateInput {
  name: string
  role: UserRole
  ttlHours?: number
}

export interface ApiKeyCreateResult {
  apiKey: ApiKeySummary
  plainKey: string
}

export async function listApiKeys() {
  const response = await http.get<ApiEnvelope<ApiKeySummary[]>>('/api-keys')
  return unwrapApiEnvelope(response.data)
}

export async function createApiKey(payload: ApiKeyCreateInput) {
  const response = await http.post<ApiEnvelope<ApiKeyCreateResult>>('/api-keys', payload)
  return unwrapApiEnvelope(response.data)
}

export async function toggleApiKey(id: number, disabled: boolean) {
  const response = await http.put<ApiEnvelope<{ disabled: boolean }>>(`/api-keys/${id}/toggle`, { disabled })
  return unwrapApiEnvelope(response.data)
}

export async function revokeApiKey(id: number) {
  const response = await http.delete<ApiEnvelope<{ revoked: boolean }>>(`/api-keys/${id}`)
  return unwrapApiEnvelope(response.data)
}
