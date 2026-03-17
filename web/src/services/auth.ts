import { http } from './http'

export interface SetupPayload {
  username: string
  password: string
  displayName: string
}

export interface LoginPayload {
  username: string
  password: string
}

export interface UserInfo {
  id: number
  username: string
  displayName: string
  role: string
}

export interface AuthResult {
  token: string
  user: UserInfo
}

export async function fetchSetupStatus() {
  const response = await http.get<{ code: string; message: string; data: { initialized: boolean } }>('/auth/setup/status')
  return response.data.data
}

export async function setup(payload: SetupPayload) {
  const response = await http.post<{ code: string; message: string; data: AuthResult }>('/auth/setup', payload)
  return response.data.data
}

export async function login(payload: LoginPayload) {
  const response = await http.post<{ code: string; message: string; data: AuthResult }>('/auth/login', payload)
  return response.data.data
}

export async function fetchProfile() {
  const response = await http.get<{ code: string; message: string; data: UserInfo }>('/auth/profile')
  return response.data.data
}

export interface ChangePasswordPayload {
  oldPassword: string
  newPassword: string
}

export async function changePassword(payload: ChangePasswordPayload) {
  const response = await http.put<{ code: string; message: string; data: { changed: boolean } }>('/auth/password', payload)
  return response.data.data
}

export async function logout() {
  const response = await http.post<{ code: string; message: string; data: { loggedOut: boolean } }>('/auth/logout')
  return response.data.data
}
