import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'

export type UserRole = 'admin' | 'operator' | 'viewer'

export interface UserSummary {
  id: number
  username: string
  displayName: string
  email: string
  role: UserRole
  disabled: boolean
  createdAt: string
}

export interface UserUpsertPayload {
  username: string
  password?: string
  displayName: string
  email?: string
  role: UserRole
  disabled: boolean
}

export async function listUsers() {
  const response = await http.get<ApiEnvelope<UserSummary[]>>('/users')
  return unwrapApiEnvelope(response.data)
}

export async function createUser(payload: UserUpsertPayload) {
  const response = await http.post<ApiEnvelope<UserSummary>>('/users', payload)
  return unwrapApiEnvelope(response.data)
}

export async function updateUser(id: number, payload: UserUpsertPayload) {
  const response = await http.put<ApiEnvelope<UserSummary>>(`/users/${id}`, payload)
  return unwrapApiEnvelope(response.data)
}

export async function deleteUser(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/users/${id}`)
  return unwrapApiEnvelope(response.data)
}
