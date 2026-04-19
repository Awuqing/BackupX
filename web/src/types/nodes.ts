export interface NodeSummary {
  id: number
  name: string
  hostname: string
  ipAddress: string
  status: 'online' | 'offline'
  isLocal: boolean
  os: string
  arch: string
  agentVersion: string
  lastSeen: string
  createdAt: string
}

export interface DirEntry {
  name: string
  path: string
  isDir: boolean
  size: number
}

export type InstallMode = 'systemd' | 'docker' | 'foreground'
export type InstallArch = 'amd64' | 'arm64' | 'auto'
export type InstallSource = 'github' | 'ghproxy'

export interface BatchCreateResult {
  id: number
  name: string
}

export interface InstallTokenInput {
  mode: InstallMode
  arch: InstallArch
  agentVersion: string
  downloadSrc: InstallSource
  ttlSeconds: number
}

export interface InstallTokenResult {
  installToken: string
  expiresAt: string
  url: string
  composeUrl: string
}
