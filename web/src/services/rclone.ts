import { http } from './http'

export interface RcloneBackendOption {
  key: string
  label: string
  required: boolean
  isPassword: boolean
}

export interface RcloneBackendInfo {
  name: string
  description: string
  options: RcloneBackendOption[]
}

export async function listRcloneBackends(): Promise<RcloneBackendInfo[]> {
  const { data } = await http.get<{ data: RcloneBackendInfo[] }>('/api/storage-targets/rclone/backends')
  return data.data
}
