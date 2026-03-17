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
