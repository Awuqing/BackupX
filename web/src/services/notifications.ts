import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { NotificationDetail, NotificationPayload, NotificationSummary } from '../types/notifications'

export async function listNotifications() {
  const response = await http.get<ApiEnvelope<NotificationSummary[]>>('/notifications')
  return unwrapApiEnvelope(response.data)
}

export async function getNotification(id: number) {
  const response = await http.get<ApiEnvelope<NotificationDetail>>(`/notifications/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function createNotification(payload: NotificationPayload) {
  const response = await http.post<ApiEnvelope<NotificationDetail>>('/notifications', payload)
  return unwrapApiEnvelope(response.data)
}

export async function updateNotification(id: number, payload: NotificationPayload) {
  const response = await http.put<ApiEnvelope<NotificationDetail>>(`/notifications/${id}`, payload)
  return unwrapApiEnvelope(response.data)
}

export async function deleteNotification(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/notifications/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function testNotification(payload: NotificationPayload) {
  const response = await http.post<ApiEnvelope<{ success: boolean }>>('/notifications/test', payload, { timeout: 30000 })
  return unwrapApiEnvelope(response.data)
}

export async function testSavedNotification(id: number) {
  const response = await http.post<ApiEnvelope<{ success: boolean }>>(`/notifications/${id}/test`, undefined, { timeout: 30000 })
  return unwrapApiEnvelope(response.data)
}
